package whatsapp

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func validatePoll(question string, options []string) error {
	if !utf8.ValidString(question) || strings.TrimSpace(question) == "" || utf8.RuneCountInString(question) > 255 {
		return errors.New("enter a question of 1–255 characters")
	}
	if len(options) < 2 || len(options) > 12 {
		return errors.New("a poll needs 2–12 options")
	}
	seen := map[string]bool{}
	for _, option := range options {
		key := strings.ToLower(strings.TrimSpace(option))
		if !utf8.ValidString(option) || key == "" || utf8.RuneCountInString(option) > 100 || seen[key] {
			return errors.New("use distinct, nonempty options of at most 100 characters")
		}
		seen[key] = true
	}
	return nil
}

func (c *Client) pollChat(ctx context.Context, value string) (types.JID, error) {
	jid, err := types.ParseJID(value)
	if err != nil || jid.IsEmpty() || (jid.Server != types.GroupServer && jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
		return types.EmptyJID, errors.New("select a person or group chat")
	}
	if jid.Server == types.GroupServer {
		info, err := c.readGroup(ctx, value)
		if err != nil {
			return jid, err
		}
		allowed := false
		for _, person := range info.Participants {
			m := c.describeGroupMember(ctx, person)
			allowed = allowed || (m.IsSelf && (!info.IsAnnounce || m.IsAdmin))
		}
		if !allowed || info.Suspended || info.IsParent {
			return jid, errors.New("you cannot send to this group right now")
		}
	}
	return jid, nil
}

func (c *Client) createPoll(ctx context.Context, p featureParams) (any, error) {
	if err := validatePoll(p.Question, p.Options); err != nil {
		return nil, err
	}
	chat, err := c.pollChat(ctx, p.ChatJID)
	if err != nil {
		return nil, err
	}
	count := 1
	if p.Multiple {
		count = 0
	}
	raw := c.wa.BuildPollCreation(p.Question, p.Options, count)
	resp, err := c.wa.SendMessage(ctx, chat, raw)
	if err != nil {
		return nil, err
	}
	sender := resp.Sender.ToNonAD().String()
	if resp.Sender.IsEmpty() {
		sender = c.selfJID()
	}
	m := model.Message{ID: resp.ID, ChatJID: chat.String(), SenderJID: sender, Timestamp: resp.Timestamp.UnixMilli(), Kind: "poll", Body: p.Question, FromMe: true, Status: "sent"}
	if err = c.store.UpsertMessage(ctx, m, "", false); err != nil {
		return nil, errors.New("poll sent but local saving failed; refresh history before trying again")
	}
	c.rememberMediaPayload(m, raw)
	c.emit(gateway.Event{Name: "message.upsert", Data: m})
	return map[string]any{"id": m.ID}, nil
}

func voteHashes(vote *waE2E.PollVoteMessage) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, hash := range vote.GetSelectedOptions() {
		value := hex.EncodeToString(hash)
		if len(hash) == 32 && !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result
}

func (c *Client) receivePollVote(ctx context.Context, evt *waEvents.Message) {
	u := evt.Message.GetPollUpdateMessage()
	if u == nil || u.GetPollCreationMessageKey().GetID() == "" {
		return
	}
	raw, err := proto.Marshal(evt.Message)
	if err != nil {
		return
	}
	v := model.PollVote{ChatJID: evt.Info.Chat.String(), PollID: u.GetPollCreationMessageKey().GetID(), Sender: c.reactionSenderJID(evt.Info.Sender, evt.Info.IsFromMe),
		Timestamp: u.GetSenderTimestampMS(), OriginalChat: evt.Info.Chat.String(), OriginalSender: evt.Info.Sender.String(), FromMe: evt.Info.IsFromMe, UpdateID: evt.Info.ID, Raw: raw, Pending: true}
	if v.Timestamp <= 0 {
		v.Timestamp = evt.Info.Timestamp.UnixMilli()
	}
	if decoded, err := c.wa.DecryptPollVote(ctx, evt); err == nil {
		v.Options = voteHashes(decoded)
		v.Pending = false
		v.Raw = nil
	}
	if err = c.store.UpsertChat(ctx, model.Chat{JID: v.ChatJID, IsGroup: evt.Info.IsGroup}); err != nil {
		return
	}
	if err = c.store.SavePollVote(ctx, v); err == nil {
		c.emit(gateway.Event{Name: "poll.updated", Data: map[string]string{"chat_jid": v.ChatJID, "message_id": v.PollID}})
	}
}

func (c *Client) pollVoterKey(ctx context.Context, value string) string {
	jid, err := types.ParseJID(value)
	if err == nil && jid.Server == types.HiddenUserServer && c.wa != nil && c.wa.Store != nil && c.wa.Store.LIDs != nil {
		if phone, err := c.wa.Store.LIDs.GetPNForLID(ctx, jid); err == nil && !phone.IsEmpty() {
			value = phone.ToNonAD().String()
		}
	}
	return c.store.CanonicalChatJID(ctx, value)
}

func (c *Client) pollInfo(ctx context.Context, chat, id string) (any, error) {
	m, raw, err := c.interactivePayload(ctx, chat, id, "poll")
	if err != nil {
		return nil, err
	}
	poll := firstNonNilPoll(raw)
	if poll == nil {
		return nil, errors.New("unsupported poll variant")
	}
	names := []string{}
	for _, o := range poll.GetOptions() {
		names = append(names, o.GetOptionName())
	}
	if err := validatePoll(poll.GetName(), names); err != nil {
		return nil, errors.New("this poll variant cannot be displayed or voted on here")
	}
	votes, err := c.store.PollVotes(ctx, chat, id)
	if err != nil {
		return nil, err
	}
	latest := map[string]model.PollVote{}
	pending := false
	for _, v := range votes {
		if v.Pending && c.wa != nil {
			var update waE2E.Message
			if proto.Unmarshal(v.Raw, &update) == nil {
				originalChat, _ := types.ParseJID(v.OriginalChat)
				sender, _ := types.ParseJID(v.OriginalSender)
				evt := &waEvents.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: originalChat, Sender: sender, IsFromMe: v.FromMe, IsGroup: originalChat.Server == types.GroupServer}, ID: v.UpdateID, Timestamp: time.UnixMilli(v.Timestamp)}, Message: &update}
				if decoded, err := c.wa.DecryptPollVote(ctx, evt); err == nil {
					v.Options = voteHashes(decoded)
					v.Pending = false
					v.Raw = nil
					_ = c.store.SavePollVote(ctx, v)
				}
			}
		}
		key := c.pollVoterKey(ctx, v.Sender)
		if previous, ok := latest[key]; !ok || v.Timestamp > previous.Timestamp || (v.Timestamp == previous.Timestamp && previous.Pending && !v.Pending) {
			latest[key] = v
		}
	}
	counts := map[string]int{}
	selected := map[string]bool{}
	self := c.pollVoterKey(ctx, c.reactionSenderJID(types.EmptyJID, true))
	for key, v := range latest {
		if v.Pending {
			pending = true
			continue
		}
		for _, hash := range v.Options {
			counts[hash]++
			if key == self {
				selected[hash] = true
			}
		}
	}
	options := []map[string]any{}
	for i, hash := range whatsmeow.HashPollOptions(names) {
		h := hex.EncodeToString(hash)
		options = append(options, map[string]any{"name": names[i], "count": counts[h], "selected": selected[h]})
	}
	return map[string]any{"question": poll.GetName(), "options": options, "limit": poll.GetSelectableOptionsCount(), "pending": pending, "from_me": m.FromMe}, nil
}

func (c *Client) votePoll(ctx context.Context, p featureParams) (any, error) {
	m, raw, err := c.interactivePayload(ctx, p.ChatJID, p.MessageID, "poll")
	if err != nil {
		return nil, err
	}
	poll := firstNonNilPoll(raw)
	if poll == nil {
		return nil, errors.New("unsupported poll")
	}
	chat, err := c.pollChat(ctx, p.ChatJID)
	if err != nil {
		return nil, err
	}
	names := []string{}
	allowed := map[string]bool{}
	for _, o := range poll.GetOptions() {
		names = append(names, o.GetOptionName())
		allowed[o.GetOptionName()] = true
	}
	if err = validatePoll(poll.GetName(), names); err != nil {
		return nil, err
	}
	if len(p.Options) > len(names) || (poll.GetSelectableOptionsCount() > 0 && len(p.Options) > int(poll.GetSelectableOptionsCount())) {
		return nil, errors.New("too many options selected")
	}
	seen := map[string]bool{}
	for _, name := range p.Options {
		if !allowed[name] || seen[name] {
			return nil, errors.New("poll options changed; reopen the poll")
		}
		seen[name] = true
	}
	sender, err := types.ParseJID(m.SenderJID)
	if err != nil {
		return nil, err
	}
	info := &types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsFromMe: m.FromMe, IsGroup: chat.Server == types.GroupServer}, ID: m.ID}
	update, err := c.wa.BuildPollVote(ctx, info, p.Options)
	if err != nil {
		return nil, err
	}
	resp, err := c.wa.SendMessage(ctx, chat, update)
	if err != nil {
		return nil, err
	}
	v := model.PollVote{ChatJID: chat.String(), PollID: m.ID, Sender: c.reactionSenderJID(types.EmptyJID, true), Timestamp: update.GetPollUpdateMessage().GetSenderTimestampMS(), Options: voteHashes(&waE2E.PollVoteMessage{SelectedOptions: whatsmeow.HashPollOptions(p.Options)})}
	if err = c.store.SavePollVote(ctx, v); err != nil {
		return nil, errors.New("vote sent but not saved locally; reopen the poll before trying again")
	}
	c.emit(gateway.Event{Name: "poll.updated", Data: map[string]string{"chat_jid": chat.String(), "message_id": m.ID}})
	return map[string]any{"id": resp.ID}, nil
}
