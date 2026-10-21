package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// quickReplyButton is the name WhatsApp gives a button that sends a choice back
// rather than opening a link or dialling.
const quickReplyButton = "quick_reply"

// buttonPromptPayload builds the interactive message that carries tappable
// answers. The legacy ButtonsMessage and ListMessage shapes are not used: a
// linked device is refused when it sends those, so the only shape that reaches
// a reader is an interactive message carrying a native flow.
//
// The identifiers are put into the parameter JSON with a marshaller rather than
// by joining strings, because they come from whatever client called the daemon
// and a quotation mark in one of them would otherwise rewrite the document.
func buttonPromptPayload(prompt model.ButtonPrompt, reply *waE2E.ContextInfo, now time.Time) (*waE2E.Message, []waBinary.Node, error) {
	buttons := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, 0, len(prompt.Buttons))
	for _, button := range prompt.Buttons {
		params, err := json.Marshal(struct {
			DisplayText string `json:"display_text"`
			ID          string `json:"id"`
		}{DisplayText: button.Title, ID: button.ID})
		if err != nil {
			return nil, nil, err
		}
		buttons = append(buttons, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
			Name: proto.String(quickReplyButton), ButtonParamsJSON: proto.String(string(params)),
		})
	}
	interactive := &waE2E.InteractiveMessage{
		Body:        &waE2E.InteractiveMessage_Body{Text: proto.String(prompt.Text)},
		ContextInfo: reply,
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: buttons, MessageVersion: proto.Int32(1),
			},
		},
	}
	if prompt.Footer != "" {
		interactive.Footer = &waE2E.InteractiveMessage_Footer{Text: proto.String(prompt.Footer)}
	}
	// The node travels with the payload rather than being added at the send,
	// because the two are only correct together: this message without the node
	// is the shape WhatsApp refuses, and nothing in the payload itself shows
	// that the node is missing.
	return &waE2E.Message{InteractiveMessage: interactive},
		[]waBinary.Node{nativeFlowBizNode(quickReplyButton, now)}, nil
}

// nativeFlowBizNode is the stanza node WhatsApp's servers require beside an
// interactive message. The library this project builds on does not write it:
// its own node is only attached for the legacy button and list shapes, and an
// interactive message sent without it is refused. The attributes mirror the
// ones a working client sends, including the one that declares the sender as a
// third party, so nothing here pretends to be WhatsApp's own software.
func nativeFlowBizNode(name string, now time.Time) waBinary.Node {
	return waBinary.Node{
		Tag: "biz",
		Attrs: waBinary.Attrs{
			"actual_actors":   "2",
			"host_storage":    "2",
			"privacy_mode_ts": strconv.FormatInt(now.Unix(), 10),
		},
		Content: []waBinary.Node{
			{
				Tag:   "interactive",
				Attrs: waBinary.Attrs{"type": "native_flow", "v": "1"},
				Content: []waBinary.Node{
					{Tag: "native_flow", Attrs: waBinary.Attrs{"name": name, "v": "9"}},
				},
			},
			{Tag: "quality_control", Attrs: waBinary.Attrs{"source_type": "third_party"}},
		},
	}
}

func (c *Client) SendButtonPrompt(ctx context.Context, req gateway.ButtonPromptRequest) (model.Message, error) {
	prompt, err := model.NewButtonPrompt(req.Prompt.Text, req.Prompt.Footer, req.Prompt.Buttons)
	if err != nil {
		return model.Message{}, err
	}
	chat, err := types.ParseJID(req.ChatJID)
	if err != nil || chat.User == "" || (chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.GroupServer) {
		return model.Message{}, errors.New("a direct or group conversation is required")
	}
	if c.wa == nil || !c.wa.IsConnected() {
		return model.Message{}, errors.New("WhatsApp is disconnected")
	}
	payload, nodes, err := buttonPromptPayload(prompt, c.replyContext(ctx, req.ChatJID, req.ReplyTo), time.Now())
	if err != nil {
		return model.Message{}, err
	}
	resp, err := c.wa.SendMessage(ctx, chat, payload, whatsmeow.SendRequestExtra{AdditionalNodes: &nodes})
	if err != nil {
		return model.Message{}, err
	}
	// The stored message carries its choices, so a question this account asked
	// shows the same answers here that the reader is looking at. They are not
	// pressable on this side: the asker is not the one being asked.
	offered := make([]model.MessageChoice, 0, len(prompt.Buttons))
	for _, button := range prompt.Buttons {
		offered = append(offered, model.MessageChoice{ID: button.ID, Title: button.Title})
	}
	msg := model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(), Kind: "text", Body: prompt.Text, FromMe: true,
		Status: "sent", ReplyTo: req.ReplyTo,
		Prompt: &model.MessagePrompt{Footer: prompt.Footer, Choices: offered}}
	// Remembering what was offered is what lets an answer be checked against
	// the question rather than merely against this account having sent it.
	_ = c.store.RecordPromptChoices(ctx, msg.ChatJID, msg.ID, buttonChoiceIDs(prompt))
	return c.withReplyPreview(ctx, msg), nil
}

// nativeFlowChoice reads which button a reader tapped. The answer arrives as a
// JSON document rather than as typed fields, so an unparsable one is reported
// as no choice rather than guessed at. The identifier key is the one working
// clients send for a quick-reply tap; it is not verified against WhatsApp here,
// and if it is ever renamed the event carries an empty choice while the message
// still shows the title that was tapped, so the gap is visible and not silent.
func nativeFlowChoice(resp *waE2E.InteractiveResponseMessage_NativeFlowResponseMessage) (id, title string) {
	if resp == nil {
		return "", ""
	}
	var params struct {
		ID          string `json:"id"`
		DisplayText string `json:"display_text"`
	}
	if err := json.Unmarshal([]byte(resp.GetParamsJSON()), &params); err != nil {
		return "", ""
	}
	return params.ID, params.DisplayText
}

// buttonAnswer is which button a reader pressed. The title is kept beside the
// identifier because the identifier is not always readable: it arrives inside a
// JSON document whose key WhatsApp documents nowhere, while the title is a
// plain field in every shape. A caller that cannot match the identifier can
// still see which answer was given.
type buttonAnswer struct {
	ID    string
	Title string
}

// buttonTap reports whether a message is an answer to a prompt, and which
// choice it carries. Every known response shape counts. WhatsApp has not
// settled on one: the reader's client picks the shape, a prompt sent by someone
// else such as a business account is answered in the legacy shapes, and the
// same tap can arrive as a template reply on one version and an interactive
// response on another. A tap whose identifier cannot be read is still reported
// as a tap, so a caller waiting on an answer learns that one arrived instead of
// waiting on in silence.
func buttonTap(msg *waE2E.Message) (buttonAnswer, bool) {
	switch {
	case msg.GetButtonsResponseMessage() != nil:
		v := msg.GetButtonsResponseMessage()
		return buttonAnswer{ID: v.GetSelectedButtonID(), Title: v.GetSelectedDisplayText()}, true
	case msg.GetListResponseMessage() != nil:
		v := msg.GetListResponseMessage()
		return buttonAnswer{ID: v.GetSingleSelectReply().GetSelectedRowID(), Title: v.GetTitle()}, true
	case msg.GetInteractiveResponseMessage() != nil:
		v := msg.GetInteractiveResponseMessage()
		id, title := nativeFlowChoice(v.GetNativeFlowResponseMessage())
		return buttonAnswer{ID: id, Title: firstNonEmpty(title, v.GetBody().GetText())}, true
	case msg.GetTemplateButtonReplyMessage() != nil:
		v := msg.GetTemplateButtonReplyMessage()
		return buttonAnswer{ID: v.GetSelectedID(), Title: v.GetSelectedDisplayText()}, true
	}
	return buttonAnswer{}, false
}

// singleSelectButton is the name WhatsApp gives the button that opens a list.
const singleSelectButton = "single_select"

// listPromptPayload builds a question whose answers are too many to be
// buttons. It travels the same way a button prompt does, as an interactive
// message carrying a native flow, with the same stanza node beside it; only
// the one button differs, and it opens a list instead of answering.
//
// The legacy ListMessage is not used. A linked device is refused when it sends
// one, and the library this project builds on tags it as a list of its own
// accord, which is the tagging those refusals are about.
func listPromptPayload(prompt model.ListPrompt, reply *waE2E.ContextInfo, now time.Time) (*waE2E.Message, []waBinary.Node, error) {
	type row struct {
		Header      string `json:"header"`
		Title       string `json:"title"`
		Description string `json:"description"`
		ID          string `json:"id"`
	}
	type section struct {
		Title string `json:"title"`
		Rows  []row  `json:"rows"`
	}
	params := struct {
		Title    string    `json:"title"`
		Sections []section `json:"sections"`
	}{Title: prompt.ButtonText}
	for _, from := range prompt.Sections {
		built := section{Title: from.Title}
		for _, choice := range from.Rows {
			built.Rows = append(built.Rows, row{Title: choice.Title, Description: choice.Description, ID: choice.ID})
		}
		params.Sections = append(params.Sections, built)
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, nil, err
	}
	interactive := &waE2E.InteractiveMessage{
		Body:        &waE2E.InteractiveMessage_Body{Text: proto.String(prompt.Text)},
		ContextInfo: reply,
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{
				Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{{
					Name: proto.String(singleSelectButton), ButtonParamsJSON: proto.String(string(encoded)),
				}},
				MessageVersion: proto.Int32(1),
			},
		},
	}
	if prompt.Footer != "" {
		interactive.Footer = &waE2E.InteractiveMessage_Footer{Text: proto.String(prompt.Footer)}
	}
	return &waE2E.Message{InteractiveMessage: interactive},
		[]waBinary.Node{nativeFlowBizNode(singleSelectButton, now)}, nil
}

func (c *Client) SendListPrompt(ctx context.Context, req gateway.ListPromptRequest) (model.Message, error) {
	prompt, err := model.NewListPrompt(req.Prompt.Text, req.Prompt.Footer, req.Prompt.ButtonText, req.Prompt.Sections)
	if err != nil {
		return model.Message{}, err
	}
	chat, err := types.ParseJID(req.ChatJID)
	if err != nil || chat.User == "" || (chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.GroupServer) {
		return model.Message{}, errors.New("a direct or group conversation is required")
	}
	if c.wa == nil || !c.wa.IsConnected() {
		return model.Message{}, errors.New("WhatsApp is disconnected")
	}
	payload, nodes, err := listPromptPayload(prompt, c.replyContext(ctx, req.ChatJID, req.ReplyTo), time.Now())
	if err != nil {
		return model.Message{}, err
	}
	resp, err := c.wa.SendMessage(ctx, chat, payload, whatsmeow.SendRequestExtra{AdditionalNodes: &nodes})
	if err != nil {
		return model.Message{}, err
	}
	offered := make([]model.MessageChoice, 0, model.MaxPromptRows)
	for _, section := range prompt.Sections {
		for _, row := range section.Rows {
			offered = append(offered, model.MessageChoice{ID: row.ID, Title: row.Title, Description: row.Description})
		}
	}
	msg := model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(),
		Timestamp: resp.Timestamp.UnixMilli(), Kind: "text", Body: prompt.Text, FromMe: true,
		Status: "sent", ReplyTo: req.ReplyTo,
		Prompt: &model.MessagePrompt{Footer: prompt.Footer, Label: prompt.ButtonText, Choices: offered}}
	_ = c.store.RecordPromptChoices(ctx, msg.ChatJID, msg.ID, listChoiceIDs(prompt))
	return c.withReplyPreview(ctx, msg), nil
}

// buttonChoiceIDs and listChoiceIDs name the answers a question offered. They
// collect identifiers rather than titles: the identifier is what an answer
// comes back with, and a question checked against its own titles would accept
// an answer naming a choice it never put on the screen.
func buttonChoiceIDs(prompt model.ButtonPrompt) []string {
	choices := make([]string, 0, len(prompt.Buttons))
	for _, button := range prompt.Buttons {
		choices = append(choices, button.ID)
	}
	return choices
}

func listChoiceIDs(prompt model.ListPrompt) []string {
	var choices []string
	for _, section := range prompt.Sections {
		for _, choice := range section.Rows {
			choices = append(choices, choice.ID)
		}
	}
	return choices
}
