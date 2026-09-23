package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shukiv/whatsappgo/internal/events"
	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// scheduleSweep is how often the daemon looks for messages whose time has
// come. A minute would show on the clock; this is close enough to the second
// the reader asked for without keeping the machine busy.
const scheduleSweep = 10 * time.Second

// scheduleHorizon is as far ahead as a message may be written. A year is past
// anything anybody plans in a chat window, and a date typed with an extra
// digit is a message that would otherwise sit in the queue forever.
const scheduleHorizon = 365 * 24 * time.Hour

// scheduleParams is what the window sends to put a message in the queue.
type scheduleParams struct {
	ID      string `json:"id"`
	ChatJID string `json:"chat_jid"`
	Text    string `json:"text"`
	SendAt  int64  `json:"send_at"`
}

// scheduleMessage validates what arrived and stores it. Everything here is
// typed by somebody: the conversation has to be one this app can write to, the
// text has to say something, and the time has to be ahead of now and inside
// the horizon.
func (s *Service) scheduleMessage(ctx context.Context, p scheduleParams) (model.ScheduledMessage, error) {
	if err := model.ValidateSendTarget(p.ChatJID); err != nil {
		return model.ScheduledMessage{}, err
	}
	text := strings.TrimSpace(p.Text)
	if text == "" {
		return model.ScheduledMessage{}, errors.New("text is required")
	}
	now := s.now()
	if p.SendAt <= now.UnixMilli() {
		return model.ScheduledMessage{}, errors.New("that time has already passed; pick one in the future")
	}
	if p.SendAt > now.Add(scheduleHorizon).UnixMilli() {
		return model.ScheduledMessage{}, errors.New("that is more than a year away; pick a nearer time")
	}
	id := strings.TrimSpace(p.ID)
	if id == "" {
		id = fmt.Sprintf("scheduled-%d", now.UnixNano())
	}
	msg := model.ScheduledMessage{
		ID: id, ChatJID: p.ChatJID, Text: p.Text,
		SendAt: p.SendAt, CreatedAt: now.UnixMilli(),
	}
	if err := s.store.ScheduleMessage(ctx, msg); err != nil {
		return model.ScheduledMessage{}, err
	}
	s.events.Publish(events.Event{Name: "schedule.updated", Data: map[string]any{"chat_jid": msg.ChatJID}})
	return msg, nil
}

// SendDueScheduledMessages sends everything that should already have gone and
// answers with how many went. It is called on a timer and once at startup: a
// message that came due while the app was closed goes out late rather than not
// at all, which is the only thing a program that is not always running can
// honestly offer.
func (s *Service) SendDueScheduledMessages(ctx context.Context) int {
	due, err := s.store.DueScheduledMessages(ctx, s.now().UnixMilli())
	if err != nil || len(due) == 0 {
		return 0
	}
	sent := 0
	for _, scheduled := range due {
		msg, err := s.gateway.SendText(ctx, gateway.TextRequest{ChatJID: scheduled.ChatJID, Text: scheduled.Text})
		if err != nil {
			// The words stay in the queue. A conversation that could not be
			// reached now is usually reachable later, and there is no copy of
			// this message anywhere else.
			_ = s.store.MarkScheduledFailed(ctx, scheduled.ID, err.Error())
			s.events.Publish(events.Event{Name: "schedule.updated", Data: map[string]any{
				"chat_jid": scheduled.ChatJID, "id": scheduled.ID, "error": err.Error(),
			}})
			continue
		}
		if err := s.store.MarkScheduledSent(ctx, scheduled.ID, msg.ID, s.now().UnixMilli()); err != nil {
			// The message has gone. Saying so failed, which is worth a line in
			// the log rather than sending it again on the next sweep.
			s.events.Publish(events.Event{Name: "daemon.error", Data: map[string]string{
				"message": "record a scheduled message as sent: " + err.Error(),
			}})
		}
		if err := s.store.UpsertMessage(ctx, msg, "", false); err == nil {
			s.events.Publish(events.Event{Name: "message.upsert", Data: msg})
		}
		s.events.Publish(events.Event{Name: "schedule.updated", Data: map[string]any{
			"chat_jid": scheduled.ChatJID, "id": scheduled.ID, "sent": true,
		}})
		sent++
	}
	return sent
}

// RunScheduler sends what is due until the context ends. The daemon starts it;
// it stops when the daemon stops, which is also when scheduled messages stop
// going out.
func (s *Service) RunScheduler(ctx context.Context) {
	s.SendDueScheduledMessages(ctx)
	ticker := time.NewTicker(scheduleSweep)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SendDueScheduledMessages(ctx)
		}
	}
}

// now is the clock, named so a test can hold it still.
func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}
