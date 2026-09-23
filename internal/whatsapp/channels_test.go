package whatsapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func channelJID(t *testing.T) types.JID {
	t.Helper()
	jid, err := types.ParseJID("120363421242390117@newsletter")
	if err != nil {
		t.Fatal(err)
	}
	return jid
}

func post(id, text string, at time.Time) *types.NewsletterMessage {
	return &types.NewsletterMessage{
		MessageID: types.MessageID(id),
		Type:      "text",
		Timestamp: at,
		Message:   &waE2E.Message{Conversation: proto.String(text)},
	}
}

// A followed channel used to open onto nothing: its posts are never delivered
// to a linked device, so until they are asked for there is nothing to show.
func TestAFollowedChannelOpensOntoItsPosts(t *testing.T) {
	c := newConflictClient(t)
	jid := channelJID(t)
	when := time.Unix(1790000000, 0)
	c.fetchNewsletterMessages = func(_ context.Context, asked types.JID, params *whatsmeow.GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error) {
		if asked != jid {
			t.Fatalf("asked the wrong channel: %s", asked)
		}
		if params == nil || params.Count <= 0 {
			t.Fatal("asked for no posts at all")
		}
		return []*types.NewsletterMessage{
			post("POST1", "first", when),
			post("POST2", "second", when.Add(time.Minute)),
		}, nil
	}
	c.newsletterInfo = func(context.Context, types.JID) (*types.NewsletterMetadata, error) {
		meta := &types.NewsletterMetadata{}
		meta.ThreadMeta.Name.Text = "Labtop"
		return meta, nil
	}

	got, err := c.ChannelMessages(context.Background(), jid.String(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d posts, want 2", len(got))
	}
	page, err := c.store.ListMessagesBefore(context.Background(), jid.String(), 0, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 2 {
		t.Fatalf("kept %d posts, want 2", len(page.Messages))
	}
	if page.Messages[0].Body != "first" || page.Messages[1].Body != "second" {
		t.Fatalf("the posts are not the ones that were read: %+v", page.Messages)
	}
	chat, err := c.store.GetChat(context.Background(), jid.String())
	if err != nil {
		t.Fatal(err)
	}
	// Without the channel's own name the conversation is headed by its
	// numeric address, which names nothing to a reader.
	if chat.Title != "Labtop" {
		t.Fatalf("the conversation is called %q, want the channel's name", chat.Title)
	}
}

// Reading the same channel twice is what happens every time it is opened. It
// must leave one copy of each post rather than a second conversation's worth.
func TestReadingAChannelTwiceKeepsOneCopyOfEachPost(t *testing.T) {
	c := newConflictClient(t)
	jid := channelJID(t)
	when := time.Unix(1790000000, 0)
	c.fetchNewsletterMessages = func(context.Context, types.JID, *whatsmeow.GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error) {
		return []*types.NewsletterMessage{post("POST1", "first", when)}, nil
	}
	for i := 0; i < 2; i++ {
		if _, err := c.ChannelMessages(context.Background(), jid.String(), 10); err != nil {
			t.Fatal(err)
		}
	}
	page, err := c.store.ListMessagesBefore(context.Background(), jid.String(), 0, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 {
		t.Fatalf("kept %d copies of one post, want 1", len(page.Messages))
	}
}

// Only a channel is read this way. A conversation address sent here would ask
// WhatsApp for a channel that does not exist.
func TestOnlyAChannelAddressIsReadAsAChannel(t *testing.T) {
	c := newConflictClient(t)
	c.fetchNewsletterMessages = func(context.Context, types.JID, *whatsmeow.GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error) {
		t.Fatal("a conversation was asked for as if it were a channel")
		return nil, nil
	}
	for _, target := range []string{"972500000000@s.whatsapp.net", "120363199566278436@g.us", "status@broadcast", "", "not-an-address"} {
		if _, err := c.ChannelMessages(context.Background(), target, 10); err == nil {
			t.Fatalf("%q was accepted as a channel", target)
		}
	}
}

// A channel that cannot be reached leaves nothing behind. An empty page is
// better than a conversation of half-read posts.
func TestAChannelThatCannotBeReachedStoresNothing(t *testing.T) {
	c := newConflictClient(t)
	jid := channelJID(t)
	c.fetchNewsletterMessages = func(context.Context, types.JID, *whatsmeow.GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error) {
		return nil, errors.New("disconnected")
	}
	if _, err := c.ChannelMessages(context.Background(), jid.String(), 10); err == nil {
		t.Fatal("a failure to read the channel was reported as success")
	}
	page, err := c.store.ListMessagesBefore(context.Background(), jid.String(), 0, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 0 {
		t.Fatalf("kept %d posts after a failed read", len(page.Messages))
	}
}

func group(jid, name string) *types.GroupInfo {
	parsed, _ := types.ParseJID(jid)
	info := &types.GroupInfo{JID: parsed}
	info.Name = name
	return info
}

func linkedTo(child *types.GroupInfo, parent string) *types.GroupInfo {
	parsed, _ := types.ParseJID(parent)
	child.LinkedParentJID = parsed
	return child
}

// Being in a community's group is how nearly everybody is in a community at
// all. Listing only the communities joined in their own right left this page
// empty for them.
func TestACommunityYouAreInThroughOneOfItsGroupsIsListed(t *testing.T) {
	c := newConflictClient(t)
	c.joinedGroups = func(context.Context) ([]*types.GroupInfo, error) {
		return []*types.GroupInfo{
			group("120363199566278436@g.us", "Servers"),
			linkedTo(group("120363405945307927@g.us", "Announcements"), "120363111111111111@g.us"),
		}, nil
	}
	c.groupInfo = func(_ context.Context, jid types.JID) (*types.GroupInfo, error) {
		if jid.String() != "120363111111111111@g.us" {
			t.Fatalf("looked up %s rather than the community", jid)
		}
		info := group(jid.String(), "Neighbourhood")
		info.Topic = "Everything local"
		info.ParticipantCount = 42
		return info, nil
	}
	got, err := c.ListCommunities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("listed %d communities, want 1: %+v", len(got), got)
	}
	if got[0].JID != "120363111111111111@g.us" || got[0].Name != "Neighbourhood" ||
		got[0].Description != "Everything local" || got[0].ParticipantCount != 42 {
		t.Fatalf("the community is listed without its details: %+v", got[0])
	}
}

// A community and several of its groups all name the same community. It
// belongs on the page once.
func TestACommunityIsListedOnceHoweverManyOfItsGroupsYouAreIn(t *testing.T) {
	c := newConflictClient(t)
	parent := group("120363111111111111@g.us", "Neighbourhood")
	parent.IsParent = true
	c.joinedGroups = func(context.Context) ([]*types.GroupInfo, error) {
		return []*types.GroupInfo{
			parent,
			linkedTo(group("120363405945307927@g.us", "Announcements"), "120363111111111111@g.us"),
			linkedTo(group("120363421808468557@g.us", "Chatter"), "120363111111111111@g.us"),
		}, nil
	}
	c.groupInfo = func(_ context.Context, jid types.JID) (*types.GroupInfo, error) {
		t.Fatalf("looked up %s, which was already listed", jid)
		return nil, nil
	}
	got, err := c.ListCommunities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("listed %d communities, want 1: %+v", len(got), got)
	}
	if got[0].Name != "Neighbourhood" {
		t.Fatalf("the community lost its name: %+v", got[0])
	}
}

// A community whose details cannot be read is still one this account is in.
// Dropping it is the empty page this change exists to fix.
func TestACommunityWhoseDetailsCannotBeReadIsStillListed(t *testing.T) {
	c := newConflictClient(t)
	c.joinedGroups = func(context.Context) ([]*types.GroupInfo, error) {
		return []*types.GroupInfo{
			linkedTo(group("120363405945307927@g.us", "Announcements"), "120363111111111111@g.us"),
		}, nil
	}
	c.groupInfo = func(context.Context, types.JID) (*types.GroupInfo, error) {
		return nil, errors.New("not available")
	}
	got, err := c.ListCommunities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("listed %d communities, want 1", len(got))
	}
	if got[0].JID != "120363111111111111@g.us" || got[0].Name == "" {
		t.Fatalf("the community is listed with nothing to identify it: %+v", got[0])
	}
}
