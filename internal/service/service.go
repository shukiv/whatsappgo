package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shuki/whatsappgo/internal/events"
	"github.com/shuki/whatsappgo/internal/gateway"
	"github.com/shuki/whatsappgo/internal/model"
	"github.com/shuki/whatsappgo/internal/store"
)

type Service struct {
	store       *store.Store
	gateway     gateway.Gateway
	events      *events.Broker
	unsubscribe func()
}

func New(st *store.Store, gw gateway.Gateway, broker *events.Broker) *Service {
	s := &Service{store: st, gateway: gw, events: broker}
	s.unsubscribe = gw.Subscribe(func(evt gateway.Event) { broker.Publish(events.Event{Name: evt.Name, Data: evt.Data}) })
	return s
}

func (s *Service) Close() {
	if s.unsubscribe != nil {
		s.unsubscribe()
	}
}

type chatListParams struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Query  string `json:"query"`
}
type messageListParams struct {
	ChatJID string `json:"chat_jid"`
	Before  int64  `json:"before"`
	Limit   int    `json:"limit"`
}
type historyRequestParams struct {
	ChatJID string `json:"chat_jid"`
	Limit   int    `json:"limit"`
}
type downloadParams struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
}
type sendTextParams struct {
	ChatJID string `json:"chat_jid"`
	Text    string `json:"text"`
	ReplyTo string `json:"reply_to"`
}
type sendMediaParams struct {
	ChatJID string `json:"chat_jid"`
	Path    string `json:"path"`
	Caption string `json:"caption"`
	ReplyTo string `json:"reply_to"`
	Voice   bool   `json:"voice"`
}
type reactionParams struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	SenderJID string `json:"sender_jid"`
	Emoji     string `json:"emoji"`
}
type editParams struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	SenderJID string `json:"sender_jid"`
	Text      string `json:"text"`
}
type readParams struct {
	ChatJID    string   `json:"chat_jid"`
	SenderJID  string   `json:"sender_jid"`
	MessageIDs []string `json:"message_ids"`
	Timestamp  int64    `json:"timestamp"`
}
type typingParams struct {
	ChatJID string `json:"chat_jid"`
	Typing  bool   `json:"typing"`
}
type searchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}
type phoneParams struct {
	Phone string `json:"phone"`
}

func (s *Service) Handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	switch method {
	case "status.get":
		return s.gateway.Status(), nil
	case "connection.connect":
		return okResult(), s.gateway.Connect(ctx)
	case "connection.disconnect":
		s.gateway.Disconnect()
		return okResult(), nil
	case "pairing.start":
		return okResult(), s.gateway.StartPairing(ctx)
	case "pairing.phone":
		var p phoneParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		p.Phone = strings.TrimSpace(p.Phone)
		if p.Phone == "" {
			return nil, errors.New("phone is required")
		}
		code, err := s.gateway.PairPhone(ctx, p.Phone)
		if err != nil {
			return nil, err
		}
		return map[string]string{"code": code}, nil
	case "account.logout":
		return okResult(), s.gateway.Logout(ctx)
	case "chats.list":
		var p chatListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.store.ListChats(ctx, p.Limit, p.Offset, p.Query)
	case "statuses.list":
		page, err := s.store.ListMessages(ctx, "status@broadcast", 0, 200)
		if err != nil {
			return nil, err
		}
		for i, j := 0, len(page.Messages)-1; i < j; i, j = i+1, j-1 {
			page.Messages[i], page.Messages[j] = page.Messages[j], page.Messages[i]
		}
		return page.Messages, nil
	case "calls.list":
		return s.store.ListCallLogs(ctx, 200)
	case "channels.list":
		return s.gateway.ListChannels(ctx)
	case "communities.list":
		return s.gateway.ListCommunities(ctx)
	case "messages.list":
		var p messageListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.store.ListMessages(ctx, p.ChatJID, p.Before, p.Limit)
	case "messages.search":
		var p searchParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.store.SearchMessages(ctx, p.Query, p.Limit)
	case "history.request":
		var p historyRequestParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if p.Limit <= 0 || p.Limit > 200 {
			p.Limit = 50
		}
		return okResult(), s.gateway.RequestHistory(ctx, p.ChatJID, p.Limit)
	case "message.download":
		var p downloadParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		return s.gateway.DownloadMedia(ctx, p.ChatJID, p.MessageID)
	case "message.send":
		var p sendTextParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.ChatJID) == "" {
			return nil, errors.New("chat_jid is required")
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, errors.New("text is required")
		}
		msg, err := s.gateway.SendText(ctx, p.ChatJID, p.Text, p.ReplyTo)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertMessage(ctx, msg, "", false); err != nil {
			return nil, fmt.Errorf("save outgoing message: %w", err)
		}
		s.events.Publish(events.Event{Name: "message.upsert", Data: msg})
		return msg, nil
	case "message.send_media":
		var p sendMediaParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.Path == "" {
			return nil, errors.New("chat_jid and path are required")
		}
		msg, err := s.gateway.SendMedia(ctx, gateway.MediaRequest{ChatJID: p.ChatJID, Path: p.Path, Caption: p.Caption, ReplyTo: p.ReplyTo, Voice: p.Voice})
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertMessage(ctx, msg, "", false); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "message.upsert", Data: msg})
		return msg, nil
	case "message.react":
		var p reactionParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		return okResult(), s.gateway.SendReaction(ctx, p.ChatJID, p.MessageID, p.SenderJID, p.Emoji)
	case "message.edit":
		var p editParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" || strings.TrimSpace(p.Text) == "" {
			return nil, errors.New("chat_jid, message_id, and text are required")
		}
		if err := s.gateway.EditText(ctx, p.ChatJID, p.MessageID, p.Text); err != nil {
			return nil, err
		}
		if err := s.store.EditMessage(ctx, p.ChatJID, p.MessageID, p.Text); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "message.edited", Data: map[string]any{"chat_jid": p.ChatJID, "message_id": p.MessageID, "body": p.Text}})
		return okResult(), nil
	case "message.delete":
		var p editParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		if err := s.gateway.DeleteMessage(ctx, p.ChatJID, p.MessageID, p.SenderJID); err != nil {
			return nil, err
		}
		if err := s.store.MarkRevoked(ctx, p.ChatJID, p.MessageID); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "message.revoked", Data: map[string]string{"chat_jid": p.ChatJID, "message_id": p.MessageID}})
		return okResult(), nil
	case "contact.resolve":
		var p phoneParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		p.Phone = strings.TrimSpace(p.Phone)
		if p.Phone == "" {
			return nil, errors.New("phone is required")
		}
		chat, err := s.gateway.ResolvePhone(ctx, p.Phone)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertChat(ctx, chat); err != nil {
			return nil, err
		}
		return chat, nil
	case "chat.avatar":
		var p messageListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		path, err := s.gateway.FetchAvatar(ctx, p.ChatJID)
		if err != nil {
			return nil, err
		}
		if path != "" {
			if err := s.store.UpdateChatAvatar(ctx, p.ChatJID, path); err != nil {
				return nil, err
			}
			s.events.Publish(events.Event{Name: "chat.updated", Data: map[string]string{"jid": p.ChatJID, "avatar_path": path}})
		}
		return map[string]string{"path": path}, nil
	case "chat.read":
		var p readParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if len(p.MessageIDs) > 0 {
			if err := s.gateway.MarkRead(ctx, p.ChatJID, p.SenderJID, p.MessageIDs, p.Timestamp); err != nil {
				return nil, err
			}
		}
		if err := s.store.MarkChatRead(ctx, p.ChatJID); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "chat.updated", Data: map[string]any{"jid": p.ChatJID, "unread_count": 0}})
		return okResult(), nil
	case "chat.typing":
		var p typingParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return okResult(), s.gateway.SetTyping(ctx, p.ChatJID, p.Typing)
	default:
		return nil, fmt.Errorf("unknown method %q", method)
	}
}

func decode(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		raw = []byte(`{}`)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
func okResult() map[string]bool { return map[string]bool{"ok": true} }

var _ = model.Message{}
