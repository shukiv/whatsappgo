package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/notify"
)

// maxTrackedAlerts bounds the deduplication table.
const maxTrackedAlerts = 256

func freshAlert(timestamp time.Time) bool {
	age := time.Since(timestamp)
	return !timestamp.IsZero() && age >= -30*time.Second && age < 5*time.Minute
}

// Status updates already arrive as messages, but are not conversations. Keep
// their opt-in alerts separate from message/group alerts and their click target
// on the Status page. The caller only passes newly persisted messages.
func (c *Client) notifyStatusUpdate(evt *waEvents.Message, msg model.Message) {
	if msg.ChatJID != types.StatusBroadcastJID.String() || msg.FromMe || msg.Revoked || msg.Edited ||
		evt.Info.IsFromMe || evt.IsEdit || evt.SourceWebMsg != nil || evt.Info.IsNewsletterStatus ||
		!freshAlert(evt.Info.Timestamp) || c.isOwnIdentity(evt.Info.Sender) {
		return
	}
	switch msg.Kind {
	case "text", "image", "video", "audio":
	default:
		return // No protocol envelopes, unknown kinds or view-once previews.
	}
	sender := evt.Info.Sender.ToNonAD()
	if msg.ID == "" || sender.IsEmpty() || (sender.Server != types.DefaultUserServer && sender.Server != types.HiddenUserServer) {
		return
	}
	ctx := context.Background()
	prefs, err := c.store.NotificationSettings(ctx)
	if err != nil || !prefs["statuses"] {
		return
	}
	jid := c.store.CanonicalChatJID(ctx, sender.String())
	chat, err := c.store.GetChat(ctx, jid)
	if (err != nil && !errors.Is(err, sql.ErrNoRows)) || chat.MutedUntil > time.Now().UnixMilli() {
		return
	}
	if !c.claimAlert("status:" + jid + ":" + msg.ID) {
		return
	}
	body := "New status update"
	if prefs["previews"] {
		preview := []rune(strings.TrimSpace(notificationBody(msg, false)))
		if len(preview) > 240 {
			preview = append(preview[:240], '…')
		}
		if len(preview) > 0 {
			body = "Status: " + string(preview)
		}
	}
	notification := notify.Message{
		ChatJID: types.StatusBroadcastJID.String(),
		Title:   notificationTitle(chat, msg.SenderName, jid), Body: body,
		IconPath: chat.AvatarPath, Silent: !prefs["sounds"] || !prefs["statuses_sound"],
	}
	handled := "0"
	if c.notifier != nil && c.notifier.Presents() {
		handled = "1"
	}
	c.emit(gateway.Event{Name: "notification.received", Data: map[string]string{
		"chat_jid": notification.ChatJID, "title": notification.Title, "body": notification.Body,
		"avatar_path": notification.IconPath, "handled": handled,
	}})
	if c.notifier != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := c.notifier.Notify(ctx, notification); err != nil {
				log.Printf("deliver status alert: %v", err)
				c.notificationFailed(notification)
			}
		}()
	}
}

func (c *Client) notifyIdentityChange(evt *waEvents.IdentityChange) {
	if !freshAlert(evt.Timestamp) || evt.JID.IsEmpty() || c.isOwnIdentity(evt.JID) {
		return
	}
	ctx := context.Background()
	prefs, err := c.store.NotificationSettings(ctx)
	if err != nil || !prefs["security"] {
		return
	}
	jid := c.store.CanonicalChatJID(ctx, evt.JID.ToNonAD().String())
	// Implicit decrypt notices and the explicit device-change event can describe
	// the same change. One quiet alert per contact per five-minute window.
	if !c.claimAlert("security:" + jid) {
		return
	}
	c.deliverAlert(jid, "", "Security code changed. This can happen after reinstalling WhatsApp or changing phones. Verify the code in WhatsApp on your phone.", true)
}

func (c *Client) isOwnIdentity(jid types.JID) bool {
	if c.wa == nil || c.wa.Store == nil {
		return false
	}
	jid = jid.ToNonAD()
	return (c.wa.Store.ID != nil && c.wa.Store.ID.ToNonAD() == jid) || (!c.wa.Store.LID.IsEmpty() && c.wa.Store.LID.ToNonAD() == jid)
}

func (c *Client) claimAlert(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.callAlerts == nil {
		c.callAlerts = make(map[string]time.Time)
	}
	for k, seen := range c.callAlerts {
		if time.Since(seen) > 5*time.Minute {
			delete(c.callAlerts, k)
		}
	}
	if _, seen := c.callAlerts[key]; seen {
		return false
	}
	// The table bounds memory, not alerts. A quiet status update must never be
	// the reason an incoming call goes unannounced, so a full table gives up
	// its oldest entry instead of refusing the new alert.
	for len(c.callAlerts) >= maxTrackedAlerts {
		oldestKey, oldest := "", time.Time{}
		for candidate, seen := range c.callAlerts {
			if oldest.IsZero() || seen.Before(oldest) {
				oldestKey, oldest = candidate, seen
			}
		}
		delete(c.callAlerts, oldestKey)
	}
	c.callAlerts[key] = time.Now()
	return true
}

// notificationFailed tells the window that a notification the daemon claimed
// to have presented never reached the screen, so the window presents its own.
// A notification server whose queue is full refuses every client this way, and
// the alternative is a reader who is told nothing at all.
func (c *Client) notificationFailed(notification notify.Message) {
	c.emit(gateway.Event{Name: "notification.received", Data: map[string]string{
		"chat_jid": notification.ChatJID, "title": notification.Title,
		"body": notification.Body, "avatar_path": notification.IconPath,
		"handled": "0",
	}})
}

func (c *Client) notifyReaction(evt *waEvents.Message, reaction model.Reaction) {
	if evt.Info.IsFromMe || evt.SourceWebMsg != nil || reaction.Emoji == "" || !freshAlert(evt.Info.Timestamp) {
		return
	}
	ctx := context.Background()
	target, err := c.store.GetMessage(ctx, reaction.ChatJID, reaction.MessageID)
	if err != nil || !target.FromMe || target.Revoked || reaction.ChatJID == "status@broadcast" {
		return
	}
	category := "messages"
	if evt.Info.IsGroup {
		category = "groups"
	}
	prefs, err := c.store.NotificationSettings(ctx)
	if err != nil || !prefs[category] || !prefs[category+"_reactions"] {
		return
	}
	body := "New reaction to your message"
	if prefs["previews"] {
		sender := c.withSenderName(ctx, model.Message{SenderJID: evt.Info.Sender.String()}).SenderName
		if sender == "" {
			sender = evt.Info.PushName
		}
		if sender == "" {
			sender = displayJID(evt.Info.Sender.String())
		}
		body = fmt.Sprintf("%s reacted %s to your message", sender, reaction.Emoji)
	}
	c.deliverAlert(reaction.ChatJID, evt.Info.PushName, body, !prefs["sounds"] || !prefs[category+"_sound"])
}

func (c *Client) notifyCall(meta types.BasicCallMeta) {
	if !freshAlert(meta.Timestamp) || meta.CallID == "" {
		return
	}
	caller := meta.CallCreator.ToNonAD()
	if caller.IsEmpty() {
		caller = meta.From.ToNonAD()
	}
	if caller.IsEmpty() || caller.String() == c.selfJID() {
		return
	}
	ctx := context.Background()
	prefs, err := c.store.NotificationSettings(ctx)
	if err != nil || !prefs["calls"] {
		return
	}
	// Group calls can emit both an offer and a notice. Keep a small, expiring
	// set so reconnects and duplicate offers cannot ring repeatedly.
	if !c.claimAlert("call:" + caller.String() + ":" + meta.CallID) {
		return
	}
	chat := caller.String()
	if !meta.GroupJID.IsEmpty() {
		chat = meta.GroupJID.String()
	}
	c.deliverAlert(chat, "", "Incoming WhatsApp call — answer on your phone", !prefs["sounds"] || !prefs["calls_sound"])
}

func (c *Client) deliverAlert(chatJID, fallbackTitle, body string, silent bool) {
	ctx := context.Background()
	chat, err := c.store.GetChat(ctx, chatJID)
	if err != nil {
		chat = model.Chat{JID: chatJID}
	}
	if chat.MutedUntil > time.Now().UnixMilli() {
		return
	}
	title := notificationTitle(chat, fallbackTitle, chatJID)
	handled := "0"
	if c.notifier != nil && c.notifier.Presents() {
		handled = "1"
	}
	c.emit(gateway.Event{Name: "notification.received", Data: map[string]string{
		"chat_jid": chatJID, "title": title, "body": body, "avatar_path": chat.AvatarPath, "handled": handled,
	}})
	if c.notifier == nil {
		return
	}
	notification := notify.Message{ChatJID: chatJID, Title: title, Body: body, IconPath: chat.AvatarPath, Silent: silent}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.notifier.Notify(ctx, notification); err != nil {
			log.Printf("deliver desktop alert: %v", err)
			c.notificationFailed(notification)
		}
	}()
}
