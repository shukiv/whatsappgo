package whatsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var _ gateway.InteractiveFeatures = (*Client)(nil)

type featureParams struct {
	ChatJID     string   `json:"chat_jid"`
	MessageID   string   `json:"message_id"`
	Link        string   `json:"link"`
	ExpectedJID string   `json:"expected_jid"`
	ChildJID    string   `json:"child_jid"`
	Action      string   `json:"action"`
	Description string   `json:"description"`
	Previous    *string  `json:"previous"`
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Multiple    bool     `json:"multiple"`
}

func (c *Client) InteractiveFeature(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	var p featureParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if method == "poll.info" {
		return c.pollInfo(ctx, p.ChatJID, p.MessageID)
	}
	if method == "event.info" {
		return c.eventInfo(ctx, p.ChatJID, p.MessageID)
	}
	if c.wa == nil || !c.wa.IsConnected() || !c.wa.IsLoggedIn() {
		return nil, errors.New("connect to WhatsApp before using this feature")
	}
	switch method {
	case "poll.create":
		return c.createPoll(ctx, p)
	case "poll.vote":
		return c.votePoll(ctx, p)
	case "group.invite_preview":
		return c.previewGroupInvite(ctx, p.Link)
	case "group.join_previewed":
		return c.joinPreviewedGroup(ctx, p)
	case "group.invite_qr":
		link, err := c.GroupInviteLink(ctx, p.ChatJID, false)
		if err != nil {
			return nil, err
		}
		png, err := qrcode.Encode(link, qrcode.Medium, 384)
		return map[string]any{"link": link, "image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)}, err
	case "community.info":
		return c.communityDetails(ctx, p.ChatJID)
	case "community.description":
		return c.editCommunityDescription(ctx, p)
	case "community.link":
		return c.changeCommunityLink(ctx, p)
	case "status.audience_contacts":
		audience, err := c.StatusAudience(ctx)
		if err != nil {
			return nil, err
		}
		people := []model.GroupMember{}
		for _, value := range audience.JIDs {
			jid, err := types.ParseJID(value)
			if err != nil {
				continue
			}
			person := types.GroupParticipant{JID: jid}
			if jid.Server == types.HiddenUserServer && c.wa.Store.LIDs != nil {
				person.PhoneNumber, _ = c.wa.Store.LIDs.GetPNForLID(ctx, jid)
			}
			people = append(people, c.describeGroupMember(ctx, person))
		}
		return map[string]any{"type": audience.Type, "people": people}, nil
	}
	return nil, errors.New("unsupported interactive feature")
}

var groupInviteCode = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

func validatedGroupInvite(value string) (string, error) {
	value = strings.TrimSpace(value)
	if groupInviteCode.MatchString(value) {
		return value, nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "chat.whatsapp.com") || u.User != nil {
		return "", errors.New("use an https://chat.whatsapp.com/ invite link or its invite code")
	}
	code := strings.TrimPrefix(u.Path, "/")
	if !groupInviteCode.MatchString(code) {
		return "", errors.New("the group invite code is invalid")
	}
	return code, nil
}

func (c *Client) previewGroupInvite(ctx context.Context, link string) (map[string]any, error) {
	code, err := validatedGroupInvite(link)
	if err != nil {
		return nil, err
	}
	info, err := c.wa.GetGroupInfoFromLink(ctx, code)
	if err != nil {
		return nil, err
	}
	if info == nil || info.JID.IsEmpty() || info.IsParent {
		return nil, errors.New("this is not an available group invitation")
	}
	return map[string]any{"jid": info.JID.String(), "name": info.Name, "description": info.Topic,
		"participant_count": info.ParticipantCount, "approval_required": info.IsJoinApprovalRequired}, nil
}

func (c *Client) joinPreviewedGroup(ctx context.Context, p featureParams) (map[string]any, error) {
	preview, err := c.previewGroupInvite(ctx, p.Link)
	if err != nil {
		return nil, err
	}
	if p.ExpectedJID == "" || preview["jid"] != p.ExpectedJID {
		return nil, errors.New("the invitation changed; preview it again before joining")
	}
	code, _ := validatedGroupInvite(p.Link)
	jid, err := c.wa.JoinGroupWithLink(ctx, code)
	if err != nil {
		return nil, err
	}
	// A successful join request is not necessarily membership. Do not open a
	// fake conversation when admins still need to approve the request.
	joined := false
	if info, readErr := c.readGroup(ctx, jid.String()); readErr == nil {
		for _, person := range info.Participants {
			if c.describeGroupMember(ctx, person).IsSelf {
				joined = true
				break
			}
		}
	}
	if joined {
		chat := model.Chat{JID: jid.String(), Title: preview["name"].(string), IsGroup: true}
		if err := c.store.UpsertChat(ctx, chat); err != nil {
			return nil, err
		}
		c.emit(gateway.Event{Name: "chat.updated", Data: chat})
	}
	return map[string]any{"jid": jid.String(), "name": preview["name"], "joined": joined}, nil
}

func (c *Client) communityAdmin(ctx context.Context, info *types.GroupInfo) bool {
	if info == nil || !info.IsParent || info.Suspended {
		return false
	}
	for _, p := range info.Participants {
		m := c.describeGroupMember(ctx, p)
		if m.IsSelf && m.IsAdmin {
			return true
		}
	}
	return false
}

func (c *Client) communityDetails(ctx context.Context, jid string) (map[string]any, error) {
	info, err := c.readGroup(ctx, jid)
	if err != nil {
		return nil, err
	}
	if !info.IsParent {
		return nil, errors.New("select a community")
	}
	groups, err := c.wa.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}
	linked, available := []map[string]any{}, []map[string]any{}
	subgroups, err := c.wa.GetSubGroups(ctx, info.JID)
	if err != nil {
		return nil, err
	}
	for _, group := range subgroups {
		if group != nil {
			linked = append(linked, map[string]any{"jid": group.JID.String(), "name": group.Name, "announcement": group.IsDefaultSubGroup})
		}
	}
	for _, group := range groups {
		if group == nil || group.IsParent || group.Suspended {
			continue
		}
		row := map[string]any{"jid": group.JID.String(), "name": group.Name, "announcement": group.IsDefaultSubGroup}
		if group.LinkedParentJID.IsEmpty() && !group.IsDefaultSubGroup {
			for _, p := range group.Participants {
				m := c.describeGroupMember(ctx, p)
				if m.IsSelf && m.IsAdmin {
					available = append(available, row)
					break
				}
			}
		}
	}
	return map[string]any{"jid": info.JID.String(), "name": info.Name, "description": info.Topic, "can_manage": c.communityAdmin(ctx, info), "linked": linked, "available": available}, nil
}

func (c *Client) editCommunityDescription(ctx context.Context, p featureParams) (any, error) {
	edit := model.GroupInfoEdit{Field: "description", Value: p.Description, Previous: p.Previous}
	if err := edit.Validate(); err != nil {
		return nil, err
	}
	info, err := c.readGroup(ctx, p.ChatJID)
	if err != nil {
		return nil, err
	}
	if !c.communityAdmin(ctx, info) {
		return nil, errors.New("only current community admins can edit the description")
	}
	if info.Topic != *p.Previous {
		return nil, errors.New("the description changed; copy your draft and reload before saving")
	}
	if err = c.wa.SetGroupTopic(ctx, info.JID, info.TopicID, "", p.Description); err != nil {
		return nil, err
	}
	c.emit(gateway.Event{Name: "community.updated", Data: map[string]string{"jid": p.ChatJID}})
	return map[string]bool{"ok": true}, nil
}

func (c *Client) changeCommunityLink(ctx context.Context, p featureParams) (any, error) {
	if p.Action != "link" && p.Action != "unlink" {
		return nil, errors.New("choose link or unlink")
	}
	parent, err := c.readGroup(ctx, p.ChatJID)
	if err != nil {
		return nil, err
	}
	if !c.communityAdmin(ctx, parent) {
		return nil, errors.New("only current community admins can manage groups")
	}
	child, err := c.readGroup(ctx, p.ChildJID)
	if err != nil {
		return nil, err
	}
	if child.IsParent || child.IsDefaultSubGroup || child.Suspended {
		return nil, errors.New("announcement, suspended and parent groups cannot be changed here")
	}
	if p.Action == "unlink" {
		if child.LinkedParentJID != parent.JID {
			return nil, errors.New("this group is no longer linked to this community")
		}
		err = c.wa.UnlinkGroup(ctx, parent.JID, child.JID)
	} else {
		if !child.LinkedParentJID.IsEmpty() {
			return nil, errors.New("this group already belongs to a community")
		}
		admin := false
		for _, person := range child.Participants {
			m := c.describeGroupMember(ctx, person)
			admin = admin || (m.IsSelf && m.IsAdmin)
		}
		if !admin {
			return nil, errors.New("you must also be an admin of the group you are linking")
		}
		err = c.wa.LinkGroup(ctx, parent.JID, child.JID)
	}
	if err != nil {
		return nil, err
	}
	c.emit(gateway.Event{Name: "community.updated", Data: map[string]string{"jid": p.ChatJID}})
	return map[string]bool{"ok": true}, nil
}

func (c *Client) interactivePayload(ctx context.Context, chat, id, kind string) (model.Message, *waE2E.Message, error) {
	m, err := c.store.GetMessage(ctx, chat, id)
	if err != nil {
		return m, nil, err
	}
	if m.Revoked || m.Kind != kind {
		return m, nil, errors.New("this message is no longer available")
	}
	data, ok, err := c.store.MediaPayload(ctx, chat, id)
	if err != nil {
		return m, nil, err
	}
	if !ok {
		return m, nil, errors.New("details were not saved on this device; refresh chat history and try again")
	}
	var raw waE2E.Message
	if err = proto.Unmarshal(data, &raw); err != nil {
		return m, nil, err
	}
	return m, &raw, nil
}

func (c *Client) eventInfo(ctx context.Context, chat, id string) (any, error) {
	_, raw, err := c.interactivePayload(ctx, chat, id, "event")
	if err != nil {
		return nil, err
	}
	e := raw.GetEventMessage()
	if e == nil {
		return nil, errors.New("unsupported event variant")
	}
	return map[string]any{"name": e.GetName(), "description": e.GetDescription(), "start": e.GetStartTime() * 1000,
		"end": e.GetEndTime() * 1000, "canceled": e.GetIsCanceled(), "location": firstNonEmpty(e.GetLocation().GetName(), e.GetLocation().GetAddress())}, nil
}
