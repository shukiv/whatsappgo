package whatsapp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shukiv/whatsappgo/internal/model"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func tapEvent(t *testing.T, msg *waE2E.Message) *waEvents.Message {
	t.Helper()
	return &waEvents.Message{
		Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: types.NewJID("1", types.DefaultUserServer)},
			ID: "tap1", Timestamp: time.Unix(1, 0)},
		Message: msg,
	}
}

func replyTo(id string) *waE2E.ContextInfo {
	return &waE2E.ContextInfo{StanzaID: proto.String(id)}
}

// The prompt has to go out as an interactive message carrying a native flow:
// a linked device is refused when it sends the legacy buttons or list shapes.
func TestAPromptIsSentAsANativeFlow(t *testing.T) {
	prompt, err := model.NewButtonPrompt("Approve the transfer?", "ref 4821", []model.PromptButton{
		{ID: "yes", Title: `He said "go"`}, {ID: "no", Title: "Reject"},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, nodes, err := buttonPromptPayload(prompt, replyTo("prompt1"), time.Unix(1760000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	// The payload is refused without this node, and nothing in the payload
	// itself reveals that it is missing, so the send must carry it.
	if len(nodes) != 1 || nodes[0].Tag != "biz" {
		t.Fatalf("the send carries no business node: %#v", nodes)
	}
	interactive := payload.GetInteractiveMessage()
	if interactive == nil {
		t.Fatalf("the prompt was not sent as an interactive message: %#v", payload)
	}
	if payload.GetButtonsMessage() != nil || payload.GetListMessage() != nil || payload.GetViewOnceMessage() != nil {
		t.Fatalf("the prompt used a shape a linked device may not send: %#v", payload)
	}
	if interactive.GetBody().GetText() != "Approve the transfer?" || interactive.GetFooter().GetText() != "ref 4821" {
		t.Fatalf("body or footer wrong: %#v", interactive)
	}
	if interactive.GetContextInfo().GetStanzaID() != "prompt1" {
		t.Fatalf("the reply was dropped: %#v", interactive.GetContextInfo())
	}
	flow := interactive.GetNativeFlowMessage()
	if flow.GetMessageVersion() != 1 {
		t.Fatalf("message version = %d", flow.GetMessageVersion())
	}
	if len(flow.GetButtons()) != 2 {
		t.Fatalf("buttons = %#v", flow.GetButtons())
	}
	for _, button := range flow.GetButtons() {
		if button.GetName() != "quick_reply" {
			t.Fatalf("button name = %q", button.GetName())
		}
	}
	// A title carrying quotation marks must be escaped by the marshaller, not
	// allowed to rewrite the parameter document.
	var params struct {
		DisplayText string `json:"display_text"`
		ID          string `json:"id"`
	}
	if err := json.Unmarshal([]byte(flow.GetButtons()[0].GetButtonParamsJSON()), &params); err != nil {
		t.Fatalf("button parameters are not valid JSON: %v (%s)", err, flow.GetButtons()[0].GetButtonParamsJSON())
	}
	if params.ID != "yes" || params.DisplayText != `He said "go"` {
		t.Fatalf("button parameters = %#v", params)
	}
}

func TestAPromptWithoutAFooterSendsNone(t *testing.T) {
	prompt, err := model.NewButtonPrompt("Approve?", "", []model.PromptButton{{ID: "yes", Title: "Approve"}})
	if err != nil {
		t.Fatal(err)
	}
	payload, _, err := buttonPromptPayload(prompt, nil, time.Unix(1760000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if payload.GetInteractiveMessage().GetFooter() != nil {
		t.Fatalf("an empty footer was sent: %#v", payload.GetInteractiveMessage().GetFooter())
	}
}

// WhatsApp refuses an interactive message that arrives without this node, and
// the library this project builds on writes it only for the legacy shapes.
func TestTheStanzaCarriesTheNativeFlowNode(t *testing.T) {
	prompt, err := model.NewButtonPrompt("Approve?", "", []model.PromptButton{{ID: "yes", Title: "Approve"}})
	if err != nil {
		t.Fatal(err)
	}
	_, nodes, err := buttonPromptPayload(prompt, nil, time.Unix(1760000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %#v", nodes)
	}
	node := nodes[0]
	if node.Tag != "biz" {
		t.Fatalf("tag = %q", node.Tag)
	}
	for key, want := range map[string]string{"actual_actors": "2", "host_storage": "2", "privacy_mode_ts": "1760000000"} {
		if got, _ := node.Attrs[key].(string); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	children, ok := node.Content.([]waBinary.Node)
	if !ok {
		t.Fatalf("node content = %#v", node.Content)
	}
	if len(children) != 2 || children[0].Tag != "interactive" || children[1].Tag != "quality_control" {
		t.Fatalf("children = %#v", children)
	}
	if got, _ := children[0].Attrs["type"].(string); got != "native_flow" {
		t.Fatalf("interactive type = %q", got)
	}
	if got, _ := children[0].Attrs["v"].(string); got != "1" {
		t.Fatalf("interactive v = %q", got)
	}
	// Declaring the sender a third party is deliberate: nothing here pretends
	// to be WhatsApp's own software.
	if got, _ := children[1].Attrs["source_type"].(string); got != "third_party" {
		t.Fatalf("quality control source = %q", got)
	}
	flow, ok := children[0].Content.([]waBinary.Node)
	if !ok || len(flow) != 1 || flow[0].Tag != "native_flow" {
		t.Fatalf("interactive content = %#v", children[0].Content)
	}
	if got, _ := flow[0].Attrs["name"].(string); got != "quick_reply" {
		t.Fatalf("flow name = %q", got)
	}
	if got, _ := flow[0].Attrs["v"].(string); got != "9" {
		t.Fatalf("flow v = %q", got)
	}
}

func TestATapIsReadAsAReplyCarryingTheChosenTitle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		message   *waE2E.Message
		wantBody  string
		wantID    string
		wantTitle string
	}{
		{"native flow", &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
			Body:        &waE2E.InteractiveResponseMessage_Body{Text: proto.String("Approve")},
			ContextInfo: replyTo("prompt1"),
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
				NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
					Name:       proto.String("quick_reply"),
					ParamsJSON: proto.String(`{"display_text":"Approve","id":"yes"}`),
				}},
		}}, "Approve", "yes", "Approve"},
		// A prompt sent by a business account is answered in the legacy shapes,
		// so those are read too.
		{"legacy buttons", &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{
			SelectedButtonID: proto.String("yes"),
			ContextInfo:      replyTo("prompt1"),
			Response:         &waE2E.ButtonsResponseMessage_SelectedDisplayText{SelectedDisplayText: "Approve"},
		}}, "Approve", "yes", "Approve"},
		{"legacy buttons with no display text", &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{
			SelectedButtonID: proto.String("yes"), ContextInfo: replyTo("prompt1"),
		}}, "yes", "yes", ""},
		{"legacy list", &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{
			Title:             proto.String("Approve"),
			ContextInfo:       replyTo("prompt1"),
			SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{SelectedRowID: proto.String("yes")},
		}}, "Approve", "yes", "Approve"},
		{"legacy list with no title", &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{
			ContextInfo:       replyTo("prompt1"),
			SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{SelectedRowID: proto.String("yes")},
		}}, "yes", "yes", ""},
		// WhatsApp has not settled on one shape: the same tap arrives as a
		// template reply on some clients.
		{"template reply", &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{
			SelectedID: proto.String("yes"), SelectedDisplayText: proto.String("Approve"),
			SelectedIndex: proto.Uint32(0), ContextInfo: replyTo("prompt1"),
		}}, "Approve", "yes", "Approve"},
		// An answer whose parameters cannot be read is still an answer: the
		// title survives and the missing identifier is visible to the caller.
		{"unreadable parameters", &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
			Body:        &waE2E.InteractiveResponseMessage_Body{Text: proto.String("Approve")},
			ContextInfo: replyTo("prompt1"),
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
				NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
					ParamsJSON: proto.String("not json"),
				}},
			// The title falls back to the answer's own body, so the choice is
			// still named even when its identifier is lost.
		}}, "Approve", "", "Approve"},
	} {
		m := messageFromEvent(tapEvent(t, tc.message))
		if m.Kind != "text" {
			t.Fatalf("%s: kind = %q", tc.name, m.Kind)
		}
		if m.Body != tc.wantBody {
			t.Fatalf("%s: body = %q, want %q", tc.name, m.Body, tc.wantBody)
		}
		if m.ReplyTo != "prompt1" {
			t.Fatalf("%s: the tap does not quote the prompt: reply_to = %q", tc.name, m.ReplyTo)
		}
		answer, tapped := buttonTap(tc.message)
		if !tapped {
			t.Fatalf("%s: the tap was not reported", tc.name)
		}
		if answer.ID != tc.wantID {
			t.Fatalf("%s: selected = %q, want %q", tc.name, answer.ID, tc.wantID)
		}
		// The title must survive even where the identifier cannot be read.
		if answer.Title != tc.wantTitle {
			t.Fatalf("%s: title = %q, want %q", tc.name, answer.Title, tc.wantTitle)
		}
	}
}

func TestAnOrdinaryMessageIsNotReportedAsATap(t *testing.T) {
	if _, tapped := buttonTap(&waE2E.Message{Conversation: proto.String("Approve")}); tapped {
		t.Fatal("a text message was reported as a tap")
	}
	if _, tapped := buttonTap(nil); tapped {
		t.Fatal("an absent message was reported as a tap")
	}
}
