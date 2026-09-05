package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"

	"github.com/shukiv/whatsappgo/internal/bugreport"
	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/linkpreview"
	"github.com/shukiv/whatsappgo/internal/mediaformat"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/store"
)

type Service struct {
	store       *store.Store
	gateway     gateway.Gateway
	events      *events.Broker
	unsubscribe func()

	version  string
	accounts int
	started  time.Time
	reporter bugreport.Submitter
	updates  updateState
}

func New(st *store.Store, gw gateway.Gateway, broker *events.Broker) *Service {
	s := &Service{
		store:    st,
		gateway:  gw,
		events:   broker,
		version:  "dev",
		accounts: 1,
		started:  time.Now(),
		reporter: bugreport.NewCLISubmitter(),
	}
	s.unsubscribe = gw.Subscribe(func(evt gateway.Event) { broker.Publish(events.Event{Name: evt.Name, Data: evt.Data}) })
	return s
}

// Describe records what a bug report should say about this build. The daemon
// knows its version and how many accounts exist; the service does not discover
// either on its own.
func (s *Service) Describe(version string, accounts int) {
	s.version = version
	s.accounts = accounts
}

// SetBugReporter replaces how reports are filed. Only tests use it.
func (s *Service) SetBugReporter(reporter bugreport.Submitter) { s.reporter = reporter }

func (s *Service) bugReportEnvironment() bugreport.Environment {
	status := s.gateway.Status()
	return bugreport.Describe(s.version, status.Connected, status.LoggedIn, s.accounts, s.started)
}

func (s *Service) Close() {
	if s.unsubscribe != nil {
		s.unsubscribe()
	}
}

func (s *Service) normalizeStickerMessages(ctx context.Context, messages []model.Message) {
	for index := range messages {
		message := &messages[index]
		if message.Kind != "sticker" {
			continue
		}
		if converted, err := mediaformat.StickerPNG(message.MediaPath); err == nil && converted != message.MediaPath {
			message.MediaPath = converted
			_ = s.store.UpdateMediaPath(ctx, message.ChatJID, message.ID, converted)
		}
		if converted, err := mediaformat.StickerPNG(message.MediaThumbnail); err == nil && converted != message.MediaThumbnail {
			message.MediaThumbnail = converted
			_ = s.store.UpdateMediaThumbnail(ctx, message.ChatJID, message.ID, converted)
		}
	}
}

type chatListParams struct {
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
	Query    string `json:"query"`
	Archived bool   `json:"archived"`
}
type messageListParams struct {
	ChatJID  string `json:"chat_jid"`
	Before   int64  `json:"before"`
	BeforeID string `json:"before_id"`
	Limit    int    `json:"limit"`
	Refresh  bool   `json:"refresh"`
}
type sharedListParams struct {
	ChatJID  string `json:"chat_jid"`
	Category string `json:"category"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
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
	ChatJID      string            `json:"chat_jid"`
	Text         string            `json:"text"`
	ReplyTo      string            `json:"reply_to"`
	ReplyChatJID string            `json:"reply_chat_jid"`
	LinkPreview  model.LinkPreview `json:"link_preview"`
}
type linkPreviewParams struct {
	Text string `json:"text"`
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
type messagePinParams struct {
	ChatJID         string `json:"chat_jid"`
	MessageID       string `json:"message_id"`
	SenderJID       string `json:"sender_jid"`
	DurationSeconds int64  `json:"duration_seconds"`
}
type messageStarParams struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	SenderJID string `json:"sender_jid"`
	FromMe    bool   `json:"from_me"`
	Starred   bool   `json:"starred"`
}
type messageForwardParams struct {
	ChatJID   string `json:"chat_jid"`
	MessageID string `json:"message_id"`
	ToChatJID string `json:"to_chat_jid"`
}
type starredListParams struct {
	Limit int `json:"limit"`
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
type playedParams struct {
	ChatJID   string `json:"chat_jid"`
	SenderJID string `json:"sender_jid"`
	MessageID string `json:"message_id"`
	Timestamp int64  `json:"timestamp"`
}
type chatFlagParams struct {
	ChatJID string `json:"chat_jid"`
	Value   bool   `json:"value"`
	// DurationSeconds only applies to chat.mute. Zero means "until undone",
	// which is what the menu's third mute choice asks for.
	DurationSeconds int64 `json:"duration_seconds"`
}
type typingParams struct {
	ChatJID string `json:"chat_jid"`
	Typing  bool   `json:"typing"`
}
type presenceParams struct {
	ChatJID string `json:"chat_jid"`
}
type searchParams struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
	// Set when the search is scoped to one conversation, as the panel that
	// searches inside an open chat does.
	ChatJID string `json:"chat_jid"`
}
type phoneParams struct {
	Phone string `json:"phone"`
}
type saveContactParams struct {
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

// Handle answers one request.
//
// Errors reach the reader as a message in the interface, so a protocol
// library's wording for its own internal state does not belong there:
// whatsmeow returns "websocket not connected" whenever the connection is down,
// which on the pairing page put a red "websocket not connected" over the QR
// code while the client was reconnecting on its own.
func (s *Service) Handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	result, err := s.handle(ctx, method, raw)
	return result, readable(err)
}

// readable rewrites the errors that say something about the connection rather
// than about the request.
func readable(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, whatsmeow.ErrNotConnected):
		return errors.New("not connected to WhatsApp yet - this will work once the connection is back")
	case errors.Is(err, whatsmeow.ErrIQTimedOut):
		return errors.New("WhatsApp did not answer in time - try again in a moment")
	}
	return err
}

func (s *Service) handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	switch method {
	case "rpc.discover":
		return discoveryResult(), nil
	case "status.get":
		return s.gateway.Status(), nil
	case "update.status":
		return s.updateStatus(), nil
	case "update.check":
		// The settings page asks for this when the reader presses the button,
		// so the answer is the fresh one rather than whatever the last tick
		// found.
		s.checkForUpdate(ctx)
		return s.updateStatus(), nil
	case "bugreport.environment":
		// The desktop shows this to the reader before anything is sent, so
		// nobody files a report without seeing what travels with it.
		environment := s.bugReportEnvironment()
		return map[string]any{"fields": environment, "rendered": environment.Render(),
			"repository": bugreport.Repository}, nil
	case "bugreport.submit":
		var params struct {
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := decode(raw, &params); err != nil {
			return nil, err
		}
		subject, body, err := bugreport.Validate(params.Subject, params.Body)
		if err != nil {
			return nil, err
		}
		report := body + "\n\n" + s.bugReportEnvironment().Render()
		url, err := s.reporter.Submit(ctx, subject, report)
		if err != nil {
			return nil, err
		}
		return map[string]any{"url": url}, nil
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
		if p.Archived {
			return s.store.ListArchivedChats(ctx, p.Limit, p.Offset, p.Query)
		}
		return s.store.ListChats(ctx, p.Limit, p.Offset, p.Query)
	case "chats.search":
		var p searchParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.store.SearchChats(ctx, p.Query, p.Limit)
	case "chats.archived_count":
		count, err := s.store.ArchivedChatCount(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]int{"count": count}, nil
	case "chats.unread_count":
		count, err := s.store.UnreadMessageCount(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]int{"count": count}, nil
	case "chat.info":
		var p messageListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.ChatJID) == "" {
			return nil, errors.New("chat_jid is required")
		}
		info, err := s.store.ChatInfo(ctx, p.ChatJID)
		if err != nil {
			return nil, err
		}
		s.normalizeStickerMessages(ctx, info.Preview)
		return info, nil
	case "media.shared":
		var p sharedListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		page, err := s.store.ListAllSharedMessages(ctx, p.Category, p.Offset, p.Limit)
		if err != nil {
			return nil, err
		}
		s.normalizeStickerMessages(ctx, page.Messages)
		return page, nil
	case "chat.shared":
		var p sharedListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		page, err := s.store.ListSharedMessages(ctx, p.ChatJID, p.Category, p.Offset, p.Limit)
		if err != nil {
			return nil, err
		}
		s.normalizeStickerMessages(ctx, page.Messages)
		return page, nil
	// "chat.read" already marks messages read for receipts; this one changes
	// the conversation's own read state, which WhatsApp tracks separately.
	case "labels.list":
		labels, err := s.store.ListLabels(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"labels": labels}, nil
	case "label.create":
		var p struct {
			Name  string `json:"name"`
			Color int    `json:"color"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		label, err := s.gateway.CreateLabel(ctx, p.Name, p.Color)
		if err != nil {
			return nil, err
		}
		return label, nil
	case "chat.label":
		var p struct {
			ChatJID string `json:"chat_jid"`
			LabelID string `json:"label_id"`
			Value   bool   `json:"value"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.LabelID == "" {
			return nil, errors.New("chat_jid and label_id are required")
		}
		if err := s.gateway.SetChatLabeled(ctx, p.ChatJID, p.LabelID, p.Value); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "chat.labels":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		ids, err := s.store.ChatLabelIDs(ctx, p.ChatJID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"label_ids": ids}, nil
	case "contact.block":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if err := s.gateway.SetContactBlocked(ctx, p.ChatJID, p.Value); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "contacts.blocked":
		blocked, err := s.gateway.BlockedContacts(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"blocked": blocked}, nil
	case "group.create":
		var p struct {
			Name         string   `json:"name"`
			Participants []string `json:"participants"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		chat, err := s.gateway.CreateGroup(ctx, p.Name, p.Participants)
		if err != nil {
			return nil, err
		}
		return chat, nil
	case "chats.mark_all_read":
		cleared, err := s.gateway.MarkAllChatsRead(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "cleared": cleared}, nil
	case "chat.delete":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if err := s.gateway.DeleteChat(ctx, p.ChatJID); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "chat.clear":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if err := s.gateway.ClearChat(ctx, p.ChatJID); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "chat.disappearing":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if err := s.gateway.SetChatDisappearing(ctx, p.ChatJID, int64(p.DurationSeconds)); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "channel.create":
		var p struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.gateway.CreateChannel(ctx, p.Name, p.Description)
	case "channel.follow_link":
		var p struct {
			Link string `json:"link"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.gateway.FollowChannelLink(ctx, p.Link)
	case "community.create":
		var p struct {
			Name string `json:"name"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.gateway.CreateCommunity(ctx, p.Name)
	case "group.join_link":
		var p struct {
			Link string `json:"link"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.gateway.JoinGroupLink(ctx, p.Link)
	case "channel.follow", "channel.mute":
		var p struct {
			JID   string `json:"jid"`
			Value bool   `json:"value"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.JID == "" {
			return nil, errors.New("jid is required")
		}
		var err error
		if method == "channel.follow" {
			err = s.gateway.SetChannelFollowed(ctx, p.JID, p.Value)
		} else {
			err = s.gateway.SetChannelMuted(ctx, p.JID, p.Value)
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "status.post":
		var p struct {
			Text       string `json:"text"`
			Path       string `json:"path"`
			Caption    string `json:"caption"`
			Background int    `json:"background"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.Path != "" {
			return s.gateway.PostMediaStatus(ctx, p.Path, p.Caption)
		}
		if p.Text == "" {
			return nil, errors.New("text or path is required")
		}
		return s.gateway.PostTextStatus(ctx, p.Text, p.Background)
	case "privacy.get":
		settings, err := s.gateway.PrivacySettings(ctx)
		if err != nil {
			return nil, err
		}
		return settings, nil
	case "privacy.set":
		var p struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.Name == "" || p.Value == "" {
			return nil, errors.New("name and value are required")
		}
		settings, err := s.gateway.SetPrivacySetting(ctx, p.Name, p.Value)
		if err != nil {
			return nil, err
		}
		return settings, nil
	case "profile.set_about":
		var p struct {
			Text string `json:"text"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if err := s.gateway.SetAbout(ctx, p.Text); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "chat.export":
		var p struct {
			ChatJID string `json:"chat_jid"`
			Path    string `json:"path"`
		}
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.Path == "" {
			return nil, errors.New("chat_jid and path are required")
		}
		written, err := s.gateway.ExportChat(ctx, p.ChatJID, p.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "path": written}, nil
	case "chat.favorite":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		if err := s.gateway.SetChatFavorite(ctx, p.ChatJID, p.Value); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	case "chat.pin", "chat.mute", "chat.archive", "chat.set_read":
		var p chatFlagParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		var err error
		switch method {
		case "chat.pin":
			err = s.gateway.SetChatPinned(ctx, p.ChatJID, p.Value)
		case "chat.mute":
			if p.DurationSeconds < 0 {
				return nil, errors.New("duration_seconds cannot be negative")
			}
			err = s.gateway.SetChatMuted(ctx, p.ChatJID, p.Value, time.Duration(p.DurationSeconds)*time.Second)
		case "chat.archive":
			err = s.gateway.SetChatArchived(ctx, p.ChatJID, p.Value)
		default:
			err = s.gateway.SetChatRead(ctx, p.ChatJID, p.Value)
		}
		return okResult(), err
	case "statuses.list":
		return s.store.ListStatusGroups(ctx, time.Now().Add(-24*time.Hour).UnixMilli(), 200)
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
		page, err := s.store.ListMessagesBefore(ctx, p.ChatJID, p.Before, p.BeforeID, p.Limit)
		if err != nil {
			return nil, err
		}
		s.normalizeStickerMessages(ctx, page.Messages)
		return page, nil
	case "messages.search":
		var p searchParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.store.SearchMessages(ctx, p.ChatJID, p.Query, p.Limit)
	case "contacts.list":
		var p searchParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.store.SearchContacts(ctx, p.Query, p.Limit)
	case "link.preview":
		var p linkPreviewParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		preview, err := linkpreview.Resolve(ctx, p.Text)
		if err != nil {
			// Missing or blocked metadata is a normal composer state, not a
			// daemon error or a toast-worthy failure.
			return model.LinkPreview{}, nil
		}
		return preview, nil
	case "link.preview.refresh":
		var p downloadParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		return s.gateway.RefreshLinkPreview(ctx, p.ChatJID, p.MessageID)
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
	case "history.refresh":
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
		return okResult(), s.gateway.RefreshHistory(ctx, p.ChatJID, p.Limit)
	case "message.download":
		var p downloadParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		return s.gateway.DownloadMedia(ctx, p.ChatJID, p.MessageID)
	case "message.played":
		var p playedParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		if p.Timestamp <= 0 {
			p.Timestamp = time.Now().UnixMilli()
		}
		if err := s.gateway.MarkPlayed(ctx, p.ChatJID, p.SenderJID, p.MessageID, p.Timestamp); err != nil {
			return nil, err
		}
		if err := s.store.UpdateReceipt(ctx, p.ChatJID, []string{p.MessageID}, "played", p.Timestamp); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "message.receipt", Data: map[string]any{
			"chat_jid": p.ChatJID, "message_ids": []string{p.MessageID}, "status": "played", "timestamp": p.Timestamp,
		}})
		return okResult(), nil
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
		msg, err := s.gateway.SendText(ctx, gateway.TextRequest{
			ChatJID: p.ChatJID, Text: p.Text, ReplyTo: p.ReplyTo,
			ReplyChatJID: p.ReplyChatJID, Preview: p.LinkPreview,
		})
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
	case "message.pin":
		var p messagePinParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		duration := time.Duration(p.DurationSeconds) * time.Second
		if duration != 24*time.Hour && duration != 7*24*time.Hour && duration != 30*24*time.Hour {
			return nil, errors.New("duration_seconds must be 86400, 604800, or 2592000")
		}
		return okResult(), s.gateway.PinMessage(ctx, p.ChatJID, p.MessageID, p.SenderJID, duration)
	case "message.unpin":
		var p messagePinParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		return okResult(), s.gateway.UnpinMessage(ctx, p.ChatJID, p.MessageID, p.SenderJID)
	case "message.star":
		var p messageStarParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" {
			return nil, errors.New("chat_jid and message_id are required")
		}
		// The gateway emits message.starred itself, and every gateway event is
		// forwarded to the broker, so publishing here would deliver it twice.
		if err := s.gateway.SetMessageStarred(ctx, p.ChatJID, p.MessageID, p.SenderJID, p.FromMe, p.Starred); err != nil {
			return nil, err
		}
		return okResult(), nil
	case "messages.starred":
		var p starredListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		items, err := s.store.ListStarredMessages(ctx, p.Limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	case "message.forward":
		var p messageForwardParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" || p.MessageID == "" || p.ToChatJID == "" {
			return nil, errors.New("chat_jid, message_id, and to_chat_jid are required")
		}
		sent, err := s.gateway.ForwardMessage(ctx, p.ChatJID, p.MessageID, p.ToChatJID)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertMessage(ctx, sent, "", false); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "message.upsert", Data: sent})
		return sent, nil
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
	case "contact.save":
		var p saveContactParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		p.Phone = strings.TrimSpace(p.Phone)
		p.Name = strings.TrimSpace(p.Name)
		if p.Phone == "" || p.Name == "" {
			return nil, errors.New("phone and name are required")
		}
		chat, err := s.gateway.ResolvePhone(ctx, p.Phone)
		if err != nil {
			return nil, err
		}
		chat.Title = p.Name
		if err := s.store.UpsertChat(ctx, chat); err != nil {
			return nil, err
		}
		if err := s.store.SetLocalChatTitle(ctx, chat.JID, p.Name); err != nil {
			return nil, err
		}
		s.events.Publish(events.Event{Name: "chat.updated", Data: map[string]string{"jid": chat.JID, "title": p.Name}})
		return chat, nil
	case "chat.avatar":
		var p messageListParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		var path string
		var err error
		if p.Refresh {
			path, err = s.gateway.RefreshAvatar(ctx, p.ChatJID)
		} else {
			path, err = s.gateway.FetchAvatar(ctx, p.ChatJID)
		}
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
	case "contact.presence.subscribe":
		var p presenceParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.ChatJID == "" {
			return nil, errors.New("chat_jid is required")
		}
		return okResult(), s.gateway.SubscribePresence(ctx, p.ChatJID)
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
