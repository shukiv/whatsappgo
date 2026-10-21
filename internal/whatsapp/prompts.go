package whatsapp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// WhatsApp has four shapes for a question with tappable answers, and which one
// arrives depends on what the sender is: a linked device may only send the
// interactive one, but whoever is on the other end of the conversation may
// send any of them, so a reader that handles one handles a fraction of what it
// will be sent.
//
// Until this existed the client mapped the four shapes an *answer* arrives in
// and none of the four a *question* arrives in. A received question therefore
// came out of messageFromEvent with no Kind at all and was dropped before the
// store, which is why a prompt visible on the phone was absent from the
// conversation here rather than merely undrawn.

// promptFromMessage reads whatever question the message carries. The last
// result says whether it carried one, because a question with no choices left
// is not a question and must not be drawn as an empty one.
func promptFromMessage(msg *waE2E.Message) (text string, prompt *model.MessagePrompt, ok bool) {
	switch {
	case msg.GetInteractiveMessage() != nil:
		return interactivePrompt(msg.GetInteractiveMessage())
	case msg.GetButtonsMessage() != nil:
		return buttonsMessagePrompt(msg.GetButtonsMessage())
	case msg.GetListMessage() != nil:
		return listMessagePrompt(msg.GetListMessage())
	case hydratedTemplate(msg.GetTemplateMessage()) != nil:
		return templatePrompt(hydratedTemplate(msg.GetTemplateMessage()))
	}
	return "", nil, false
}

// hydratedTemplate finds the filled-in template wherever this one carries it.
// There are two fields for it - a plain one and a member of the format choice -
// and a sender may use either, so reading only the first leaves the other kind
// of template unread.
func hydratedTemplate(v *waE2E.TemplateMessage) *waE2E.TemplateMessage_HydratedFourRowTemplate {
	if v == nil {
		return nil
	}
	if filled := v.GetHydratedTemplate(); filled != nil {
		return filled
	}
	return v.GetHydratedFourRowTemplate()
}

// promptContext is the quoted-reply information of whichever shape carried the
// question, so a question asked as a reply keeps the message it answers.
func promptContext(msg *waE2E.Message) *waE2E.ContextInfo {
	switch {
	case msg.GetInteractiveMessage() != nil:
		return msg.GetInteractiveMessage().GetContextInfo()
	case msg.GetButtonsMessage() != nil:
		return msg.GetButtonsMessage().GetContextInfo()
	case msg.GetListMessage() != nil:
		return msg.GetListMessage().GetContextInfo()
	case msg.GetTemplateMessage() != nil:
		return msg.GetTemplateMessage().GetContextInfo()
	}
	return nil
}

// interactivePrompt reads the shape this project itself sends, and the one a
// phone is given for a native flow: the choices live in a JSON document inside
// the button rather than in fields of their own.
func interactivePrompt(v *waE2E.InteractiveMessage) (string, *model.MessagePrompt, bool) {
	flow := v.GetNativeFlowMessage()
	if flow == nil {
		return "", nil, false
	}
	built := &model.MessagePrompt{Footer: v.GetFooter().GetText()}
	for _, button := range flow.GetButtons() {
		switch button.GetName() {
		case quickReplyButton:
			var params struct {
				ID          string `json:"id"`
				DisplayText string `json:"display_text"`
			}
			// A button whose parameters will not parse is skipped rather than
			// failing the message: one unreadable choice must not cost the
			// reader the question and the choices beside it.
			if err := json.Unmarshal([]byte(button.GetButtonParamsJSON()), &params); err != nil {
				continue
			}
			if params.ID == "" && params.DisplayText == "" {
				continue
			}
			built.Choices = append(built.Choices, model.MessageChoice{ID: params.ID, Title: params.DisplayText})
		case singleSelectButton:
			var params struct {
				Title    string `json:"title"`
				Sections []struct {
					Rows []struct {
						Title       string `json:"title"`
						Description string `json:"description"`
						ID          string `json:"id"`
					} `json:"rows"`
				} `json:"sections"`
			}
			if err := json.Unmarshal([]byte(button.GetButtonParamsJSON()), &params); err != nil {
				continue
			}
			if params.Title != "" {
				built.Label = params.Title
			}
			for _, section := range params.Sections {
				for _, row := range section.Rows {
					if row.ID == "" && row.Title == "" {
						continue
					}
					built.Choices = append(built.Choices,
						model.MessageChoice{ID: row.ID, Title: row.Title, Description: row.Description})
				}
			}
		}
	}
	if len(built.Choices) == 0 {
		return "", nil, false
	}
	return v.GetBody().GetText(), built, true
}

// buttonsMessagePrompt reads the legacy button shape. This project cannot send
// it - a linked device is refused when it tries - but it can be sent one.
func buttonsMessagePrompt(v *waE2E.ButtonsMessage) (string, *model.MessagePrompt, bool) {
	built := &model.MessagePrompt{Footer: v.GetFooterText()}
	for _, button := range v.GetButtons() {
		title := button.GetButtonText().GetDisplayText()
		if button.GetButtonID() == "" && title == "" {
			continue
		}
		built.Choices = append(built.Choices, model.MessageChoice{ID: button.GetButtonID(), Title: title})
	}
	if len(built.Choices) == 0 {
		return "", nil, false
	}
	// The question sits in a different field depending on whether it was asked
	// with a picture or a document above it.
	return firstNonEmpty(v.GetContentText(), v.GetText()), built, true
}

// listMessagePrompt reads the legacy list shape, whose choices are already
// fields rather than a JSON document.
func listMessagePrompt(v *waE2E.ListMessage) (string, *model.MessagePrompt, bool) {
	built := &model.MessagePrompt{Footer: v.GetFooterText(), Label: v.GetButtonText()}
	for _, section := range v.GetSections() {
		for _, row := range section.GetRows() {
			if row.GetRowID() == "" && row.GetTitle() == "" {
				continue
			}
			built.Choices = append(built.Choices, model.MessageChoice{
				ID: row.GetRowID(), Title: row.GetTitle(), Description: row.GetDescription(),
			})
		}
	}
	if len(built.Choices) == 0 {
		return "", nil, false
	}
	return v.GetDescription(), built, true
}

// templatePrompt reads a hydrated template's quick replies. Its other buttons
// open a link or dial a number, which the reader's own system handles and which
// are not answers to anything, so they are left out rather than drawn as
// choices that would send something.
func templatePrompt(v *waE2E.TemplateMessage_HydratedFourRowTemplate) (string, *model.MessagePrompt, bool) {
	built := &model.MessagePrompt{Footer: v.GetHydratedFooterText()}
	for _, button := range v.GetHydratedButtons() {
		reply := button.GetQuickReplyButton()
		if reply == nil {
			continue
		}
		if reply.GetID() == "" && reply.GetDisplayText() == "" {
			continue
		}
		built.Choices = append(built.Choices,
			model.MessageChoice{ID: reply.GetID(), Title: reply.GetDisplayText()})
	}
	if len(built.Choices) == 0 {
		return "", nil, false
	}
	return v.GetHydratedContentText(), built, true
}

// SendPromptAnswer presses one of the answers a received question offered.
//
// The choice is checked against the question as it was stored, not taken from
// the caller: the identifier goes out over the wire to whoever asked, and a
// caller that could put an arbitrary one there could answer a question with
// something it never offered. That is the same reasoning as the `verified`
// marker on an incoming answer, applied in the other direction.
func (c *Client) SendPromptAnswer(ctx context.Context, req gateway.PromptAnswerRequest) (model.Message, error) {
	chat, err := types.ParseJID(req.ChatJID)
	if err != nil || chat.User == "" || (chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.GroupServer) {
		return model.Message{}, errors.New("a direct or group conversation is required")
	}
	if req.MessageID == "" || req.ChoiceID == "" {
		return model.Message{}, errors.New("a message and a choice are required")
	}
	question, err := c.store.GetMessage(ctx, req.ChatJID, req.MessageID)
	if err != nil {
		return model.Message{}, errors.New("that question is not in this conversation")
	}
	if question.Prompt == nil {
		return model.Message{}, errors.New("that message asked nothing that can be answered")
	}
	// Answering your own question sends nobody anything: the asker is the one
	// who is waiting, and that is the other end of the conversation.
	if question.FromMe {
		return model.Message{}, errors.New("this account asked that question")
	}
	var chosen model.MessageChoice
	for _, choice := range question.Prompt.Choices {
		if choice.ID == req.ChoiceID {
			chosen = choice
			break
		}
	}
	if chosen.ID == "" {
		return model.Message{}, errors.New("that question did not offer that choice")
	}
	// Checked after the choice rather than before it, so a caller is told what
	// is wrong with the request itself whether or not the socket happens to be
	// up, and so the check above can be tested without one.
	if c.wa == nil || !c.wa.IsConnected() {
		return model.Message{}, errors.New("WhatsApp is disconnected")
	}
	params, err := json.Marshal(struct {
		ID          string `json:"id"`
		DisplayText string `json:"display_text"`
	}{ID: chosen.ID, DisplayText: chosen.Title})
	if err != nil {
		return model.Message{}, err
	}
	// The name says which kind of control was pressed, and a list's row is
	// answered as a list rather than as a button.
	name := quickReplyButton
	if question.Prompt.Label != "" {
		name = singleSelectButton
	}
	payload := &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
		Body: &waE2E.InteractiveResponseMessage_Body{
			Text:   proto.String(chosen.Title),
			Format: waE2E.InteractiveResponseMessage_Body_DEFAULT.Enum(),
		},
		ContextInfo: c.replyContext(ctx, req.ChatJID, req.MessageID),
		InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
			NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
				Name: proto.String(name), ParamsJSON: proto.String(string(params)), Version: proto.Int32(1),
			},
		},
	}}
	resp, err := c.wa.SendMessage(ctx, chat, payload)
	if err != nil {
		return model.Message{}, err
	}
	// Stored as the answer the other end will see: WhatsApp shows a pressed
	// choice as a reply carrying its title, which is what the conversation
	// reads like on a phone.
	msg := model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(), Kind: "text", Body: chosen.Title, FromMe: true,
		Status: "sent", ReplyTo: req.MessageID}
	return c.withReplyPreview(ctx, msg), nil
}
