package whatsapp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skip2/go-qrcode"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/linkpreview"
	"github.com/shukiv/whatsappgo/internal/mediastore"
	"github.com/shukiv/whatsappgo/internal/model"
	"github.com/shukiv/whatsappgo/internal/notify"
	"github.com/shukiv/whatsappgo/internal/store"
)

type Client struct {
	wa        *whatsmeow.Client
	container *sqlstore.Container
	store     *store.Store
	media     *mediastore.Store
	// baseCtx ends when the daemon shuts down, so background collection stops
	// with it rather than outliving the process's other work.
	baseCtx  context.Context
	notifier notify.Notifier
	mediaDir string

	mu              sync.RWMutex
	status          model.ConnectionStatus
	nextSub         uint64
	subs            map[uint64]func(gateway.Event)
	pairing         bool
	closed          bool
	historyRequests map[string]time.Time
	// Set while the post-connection sweep is running, so a reconnect does not
	// start a second one beside it.
	connectSweepRunning bool
	// The reaction backfill outlives the sweep that started it, so a
	// reconnect must not set a second one going beside it: it would ask
	// WhatsApp for the same twenty-five pages again.
	reactionBackfilling atomic.Bool
	resolveLinkPreview  func(context.Context, string) (model.LinkPreview, error)
	mediaRetries        map[types.MessageID]*mediaRetryWaiter

	// These hooks keep the expired-media recovery path deterministic in tests.
	// Production clients leave them nil and use the whatsmeow methods below.
	downloadToFile         func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error
	requestMediaRetryPath  func(context.Context, model.Message, whatsmeow.DownloadableMessage) (string, error)
	downloadWithPathToFile func(context.Context, string, whatsmeow.DownloadableMessage, whatsmeow.File) error
	sendReactionMessage    func(context.Context, types.JID, types.JID, types.MessageID, string) (whatsmeow.SendResponse, error)
	sendPeerMessage        func(context.Context, *waE2E.Message) (whatsmeow.SendResponse, error)
	fetchAppState          func(context.Context, appstate.WAPatchName, bool, bool) error
}

func New(ctx context.Context, deviceDB, mediaDir string, st *store.Store, media *mediastore.Store, notifier notify.Notifier) (*Client, error) {
	if notifier == nil {
		notifier = notify.Noop{}
	}
	configureDeviceIdentity()
	db, err := sql.Open("sqlite", "file:"+deviceDB+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	container := sqlstore.NewWithDB(db, "sqlite", nil)
	if err := container.Upgrade(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("upgrade device store: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		container.Close()
		return nil, fmt.Errorf("load device: %w", err)
	}
	wa := whatsmeow.NewClient(device, nil)
	// Required for the one-time call-history recovery sync. Without this,
	// whatsmeow intentionally suppresses generic app-state events on snapshots.
	wa.EmitAppStateEventsOnFullSync = true
	c := &Client{wa: wa, container: container, store: st, media: media, notifier: notifier, mediaDir: mediaDir, baseCtx: ctx, subs: make(map[uint64]func(gateway.Event)), historyRequests: make(map[string]time.Time), mediaRetries: make(map[types.MessageID]*mediaRetryWaiter), resolveLinkPreview: linkpreview.Resolve}
	c.status = model.ConnectionStatus{State: "disconnected", LoggedIn: wa.Store.ID != nil, LastChange: model.NowMillis()}
	if wa.Store.ID != nil {
		c.status.UserJID = wa.Store.ID.String()
		c.status.UserName = wa.Store.PushName
	}
	wa.AddEventHandler(c.handleEvent)
	return c, nil
}

func (c *Client) Status() model.ConnectionStatus { c.mu.RLock(); defer c.mu.RUnlock(); return c.status }

func (c *Client) setStatus(change func(*model.ConnectionStatus)) {
	c.mu.Lock()
	change(&c.status)
	c.status.LastChange = model.NowMillis()
	snapshot := c.status
	c.mu.Unlock()
	c.emit(gateway.Event{Name: "connection.changed", Data: snapshot})
}

func (c *Client) Subscribe(fn func(gateway.Event)) func() {
	c.mu.Lock()
	c.nextSub++
	id := c.nextSub
	c.subs[id] = fn
	c.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { c.mu.Lock(); delete(c.subs, id); c.mu.Unlock() }) }
}
func (c *Client) emit(evt gateway.Event) {
	c.mu.RLock()
	fns := make([]func(gateway.Event), 0, len(c.subs))
	for _, fn := range c.subs {
		fns = append(fns, fn)
	}
	c.mu.RUnlock()
	for _, fn := range fns {
		fn(evt)
	}
}

func (c *Client) Connect(ctx context.Context) error {
	if c.wa.IsConnected() {
		return nil
	}
	c.setStatus(func(s *model.ConnectionStatus) { s.State = "connecting"; s.LastError = "" })
	if err := c.wa.Connect(); err != nil {
		c.setStatus(func(s *model.ConnectionStatus) { s.State = "error"; s.LastError = err.Error() })
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (c *Client) StartPairing(ctx context.Context) error {
	c.mu.Lock()
	if c.pairing {
		c.mu.Unlock()
		return errors.New("pairing is already in progress")
	}
	if c.wa.Store.ID != nil {
		c.mu.Unlock()
		return errors.New("this account is already paired")
	}
	c.pairing = true
	c.mu.Unlock()
	if c.wa.IsConnected() {
		c.wa.Disconnect()
	}
	qrChan, err := c.wa.GetQRChannel(ctx)
	if err != nil {
		c.mu.Lock()
		c.pairing = false
		c.mu.Unlock()
		return err
	}
	if err := c.Connect(ctx); err != nil {
		c.mu.Lock()
		c.pairing = false
		c.mu.Unlock()
		return err
	}
	c.setStatus(func(s *model.ConnectionStatus) { s.State = "pairing" })
	go c.consumeQR(ctx, qrChan)
	return nil
}

func (c *Client) consumeQR(ctx context.Context, ch <-chan whatsmeow.QRChannelItem) {
	defer func() { c.mu.Lock(); c.pairing = false; c.mu.Unlock() }()
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-ch:
			if !ok {
				return
			}
			if item.Event == whatsmeow.QRChannelEventCode {
				png, err := qrcode.Encode(item.Code, qrcode.Medium, 384)
				if err != nil {
					c.emit(gateway.Event{Name: "pairing.error", Data: map[string]string{"message": err.Error()}})
					continue
				}
				c.emit(gateway.Event{Name: "pairing.qr", Data: map[string]any{"code": item.Code, "png_base64": base64.StdEncoding.EncodeToString(png), "expires_in": int(item.Timeout.Seconds())}})
			} else if item.Event == "success" {
				c.emit(gateway.Event{Name: "pairing.success"})
				return
			} else if item.Error != nil {
				c.emit(gateway.Event{Name: "pairing.error", Data: map[string]string{"message": item.Error.Error()}})
				return
			} else {
				c.emit(gateway.Event{Name: "pairing.state", Data: map[string]string{"state": item.Event}})
			}
		}
	}
}

func (c *Client) PairPhone(ctx context.Context, phone string) (string, error) {
	if c.wa.Store.ID != nil {
		return "", errors.New("this account is already paired")
	}
	if !c.wa.IsConnected() {
		qrChan, err := c.wa.GetQRChannel(ctx)
		if err != nil {
			return "", err
		}
		if err := c.Connect(ctx); err != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(15 * time.Second):
			return "", errors.New("timed out preparing phone pairing")
		case <-qrChan:
		}
	}
	code, err := c.wa.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err == nil {
		c.setStatus(func(s *model.ConnectionStatus) { s.State = "pairing" })
	}
	return code, err
}

func (c *Client) Disconnect() {
	c.wa.Disconnect()
	c.setStatus(func(s *model.ConnectionStatus) { s.State = "disconnected"; s.Connected = false })
}

func (c *Client) Logout(ctx context.Context) error {
	if err := c.wa.Logout(ctx); err != nil {
		return err
	}
	c.setStatus(func(s *model.ConnectionStatus) {
		*s = model.ConnectionStatus{State: "logged_out", LastChange: model.NowMillis()}
	})
	return nil
}

func (c *Client) SendText(ctx context.Context, req gateway.TextRequest) (model.Message, error) {
	chat, err := types.ParseJID(req.ChatJID)
	if err != nil {
		return model.Message{}, err
	}
	payload := &waE2E.Message{}
	if req.ReplyTo == "" && req.Preview.URL == "" {
		payload.Conversation = proto.String(req.Text)
	} else {
		quoteChatJID := req.ReplyChatJID
		if quoteChatJID == "" {
			quoteChatJID = req.ChatJID
		}
		extended := &waE2E.ExtendedTextMessage{Text: proto.String(req.Text), ContextInfo: c.replyContext(ctx, quoteChatJID, req.ReplyTo)}
		if req.Preview.URL != "" {
			extended.MatchedText = proto.String(req.Preview.URL)
			extended.Title = proto.String(req.Preview.Title)
			extended.Description = proto.String(req.Preview.Description)
			extended.JPEGThumbnail = req.Preview.Thumbnail
			if len(req.Preview.Thumbnail) > 0 {
				extended.PreviewType = waE2E.ExtendedTextMessage_IMAGE.Enum()
			}
		}
		payload.ExtendedTextMessage = extended
	}
	resp, err := c.wa.SendMessage(ctx, chat, payload)
	if err != nil {
		return model.Message{}, err
	}
	result := model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(), Timestamp: resp.Timestamp.UnixMilli(), Kind: "text", Body: req.Text, FromMe: true, Status: "sent", ReplyTo: req.ReplyTo, LinkURL: req.Preview.URL, LinkTitle: req.Preview.Title, LinkDescription: req.Preview.Description}
	// A reply sent from here describes what it answers, the same way a
	// received one does; without this the reader's own reply is the one bubble
	// with nothing above it.
	result = c.withReplyPreview(ctx, result)
	return c.withCachedOutgoingLinkPreview(result, req.Preview), nil
}

// historyRequestWindow is how long one page request suppresses an identical
// one. It also bounds how long the dedup record is worth keeping.
const historyRequestWindow = 30 * time.Second

// ErrHistoryRequestInFlight reports that an identical page was already
// requested and its answer has not arrived yet.
var ErrHistoryRequestInFlight = errors.New("a history page for this anchor was already requested")

func (c *Client) RequestHistory(ctx context.Context, chatJID string, count int) error {
	oldest, err := c.store.OldestMessage(ctx, chatJID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no local message is available to anchor history sync")
		}
		return err
	}
	return c.requestHistoryFrom(ctx, oldest, count, "older")
}

// RefreshHistory asks for the recent page again. This repairs delivery states
// that were stored before history status import existed, without guessing that
// a sent message was read: WhatsApp remains the source of truth.
func (c *Client) RefreshHistory(ctx context.Context, chatJID string, count int) error {
	newest, err := c.store.NewestMessage(ctx, chatJID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no local message is available to anchor history refresh")
		}
		return err
	}
	return c.requestHistoryFrom(ctx, newest, count, "refresh")
}

func (c *Client) requestHistoryFrom(ctx context.Context, anchor model.Message, count int, purpose string) error {
	chat, err := types.ParseJID(anchor.ChatJID)
	if err != nil {
		return err
	}
	sender, err := types.ParseJID(anchor.SenderJID)
	if err != nil || sender.IsEmpty() {
		if anchor.FromMe {
			sender, err = types.ParseJID(c.selfJID())
		} else {
			sender, err = chat, nil
		}
		if err != nil {
			return err
		}
	}
	if count <= 0 || count > 200 {
		count = 50
	}
	boundary := purpose + ":" + chat.String() + ":" + anchor.ID
	c.mu.Lock()
	if requestedAt, ok := c.historyRequests[boundary]; ok && time.Since(requestedAt) < historyRequestWindow {
		c.mu.Unlock()
		// Someone else asked for this exact page moments ago. Saying so lets
		// the caller wait for that answer instead of mistaking the silence
		// for "this conversation has no more history".
		return ErrHistoryRequestInFlight
	}
	// Entries older than the window can never suppress anything again, so they
	// are dropped here rather than accumulating for the life of the daemon.
	for key, requestedAt := range c.historyRequests {
		if time.Since(requestedAt) >= historyRequestWindow {
			delete(c.historyRequests, key)
		}
	}
	c.historyRequests[boundary] = time.Now()
	c.mu.Unlock()
	info := &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsFromMe: anchor.FromMe, IsGroup: chat.Server == types.GroupServer},
		ID:            types.MessageID(anchor.ID),
		Timestamp:     time.UnixMilli(anchor.Timestamp),
	}
	_, err = c.wa.SendPeerMessage(ctx, c.wa.BuildHistorySyncRequest(info, count))
	if err != nil {
		c.mu.Lock()
		delete(c.historyRequests, boundary)
		c.mu.Unlock()
	}
	return err
}

func (c *Client) DownloadMedia(ctx context.Context, chatJID, messageID string) (model.Message, error) {
	msg, err := c.store.GetMessage(ctx, chatJID, messageID)
	if err != nil {
		return model.Message{}, err
	}
	if msg.MediaPath != "" {
		if _, statErr := os.Stat(msg.MediaPath); statErr == nil {
			converted := c.withDisplayableSticker(msg)
			if converted.MediaPath != msg.MediaPath {
				_ = c.store.UpsertMessage(ctx, converted, "", false)
			}
			return converted, nil
		}
	}
	// The attachment database is the durable copy. Clearing the media cache
	// therefore loses nothing: the file is written back from storage instead of
	// being fetched from WhatsApp again, which may no longer be possible.
	if c.media != nil {
		path := c.cachePath(msg)
		if restored, restoreErr := c.media.Materialise(ctx, msg.ChatJID, msg.ID, path); restoreErr == nil && restored {
			msg.MediaPath = path
			if info, statErr := os.Stat(path); statErr == nil {
				msg.MediaSize = info.Size()
			}
			msg = c.withDisplayableSticker(msg)
			if err := c.store.UpsertMessage(ctx, msg, "", false); err != nil {
				return model.Message{}, err
			}
			c.emit(gateway.Event{Name: "message.upsert", Data: msg})
			return msg, nil
		}
	}
	payload, available, err := c.store.MediaPayload(ctx, chatJID, messageID)
	if err != nil {
		return model.Message{}, err
	}
	if available {
		var raw waE2E.Message
		if err := proto.Unmarshal(payload, &raw); err != nil {
			return model.Message{}, err
		}
		downloadable := downloadableFromMessage(&raw)
		if downloadable == nil {
			return model.Message{}, errors.New("stored message does not contain downloadable media")
		}
		return c.downloadMedia(ctx, msg, downloadable, &raw)
	}
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return model.Message{}, err
	}
	sender, err := types.ParseJID(msg.SenderJID)
	if err != nil || sender.IsEmpty() {
		if msg.FromMe {
			sender, err = types.ParseJID(c.selfJID())
		} else {
			sender, err = chat, nil
		}
		if err != nil {
			return model.Message{}, err
		}
	}
	_, err = c.wa.SendPeerMessage(ctx, c.wa.BuildUnavailableMessageRequest(chat, sender, messageID))
	return msg, err
}

// SetChannelFollowed follows or leaves a channel. WhatsApp calls these
// newsletters on the wire.
func (c *Client) SetChannelFollowed(ctx context.Context, channelJID string, followed bool) error {
	target, err := types.ParseJID(channelJID)
	if err != nil {
		return err
	}
	if followed {
		err = c.wa.FollowNewsletter(ctx, target)
	} else {
		err = c.wa.UnfollowNewsletter(ctx, target)
	}
	if err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "channel.updated", Data: map[string]any{"jid": channelJID, "followed": followed}})
	return nil
}

// SetChannelMuted mutes or unmutes a channel's updates.
func (c *Client) SetChannelMuted(ctx context.Context, channelJID string, muted bool) error {
	target, err := types.ParseJID(channelJID)
	if err != nil {
		return err
	}
	if err := c.wa.NewsletterToggleMute(ctx, target, muted); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "channel.updated", Data: map[string]any{"jid": channelJID, "muted": muted}})
	return nil
}

func (c *Client) ListChannels(ctx context.Context) ([]model.Channel, error) {
	items, err := c.wa.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.Channel, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		channel := model.Channel{
			JID:             item.ID.String(),
			Name:            item.ThreadMeta.Name.Text,
			Description:     item.ThreadMeta.Description.Text,
			SubscriberCount: item.ThreadMeta.SubscriberCount,
			Verified:        item.ThreadMeta.VerificationState == types.NewsletterVerificationStateVerified,
		}
		if item.ViewerMeta != nil {
			channel.Muted = item.ViewerMeta.Mute == types.NewsletterMuteOn
		}
		result = append(result, channel)
	}
	return result, nil
}

func (c *Client) ListCommunities(ctx context.Context) ([]model.Community, error) {
	groups, err := c.wa.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.Community, 0)
	for _, group := range groups {
		if group == nil || !group.IsParent {
			continue
		}
		result = append(result, model.Community{
			JID:              group.JID.String(),
			Name:             group.Name,
			Description:      group.Topic,
			ParticipantCount: group.ParticipantCount,
		})
	}
	return result, nil
}

func (c *Client) SendMedia(ctx context.Context, req gateway.MediaRequest) (model.Message, error) {
	chat, err := types.ParseJID(req.ChatJID)
	if err != nil {
		return model.Message{}, err
	}
	f, err := os.Open(req.Path)
	if err != nil {
		return model.Message{}, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return model.Message{}, err
	}
	if stat.Size() > 2*1024*1024*1024 {
		return model.Message{}, errors.New("file exceeds 2 GiB limit")
	}
	mimeType := detectMIME(f, req.Path)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return model.Message{}, err
	}
	kind, mediaType := classifyMedia(mimeType, req.Document)
	tmp, err := os.CreateTemp(c.mediaDir, "upload-*")
	if err != nil {
		return model.Message{}, err
	}
	tmpName := tmp.Name()
	defer func() { tmp.Close(); os.Remove(tmpName) }()
	upload, err := c.wa.UploadReader(ctx, f, tmp, mediaType)
	if err != nil {
		return model.Message{}, err
	}
	contextInfo := c.replyContext(ctx, req.ChatJID, req.ReplyTo)
	if req.ForwardingScore > 0 {
		if contextInfo == nil {
			contextInfo = &waE2E.ContextInfo{}
		}
		contextInfo.IsForwarded = proto.Bool(true)
		contextInfo.ForwardingScore = proto.Uint32(uint32(req.ForwardingScore))
	}
	payload := buildMediaPayload(kind, mimeType, filepath.Base(req.Path), req.Caption, upload, contextInfo, req.Voice)
	resp, err := c.wa.SendMessage(ctx, chat, payload)
	if err != nil {
		return model.Message{}, err
	}
	localPath := req.Path
	if cached, err := c.cacheOutgoing(req.Path, chat.String(), string(resp.ID), kind); err == nil {
		localPath = cached
	}
	if c.media != nil {
		info := mediastore.Info{MIME: mimeType, Name: filepath.Base(req.Path), Size: stat.Size()}
		if err := c.media.PutFile(ctx, chat.String(), string(resp.ID), info, localPath); err != nil {
			c.emit(gateway.Event{Name: "daemon.error", Data: map[string]string{"message": "store sent attachment: " + err.Error()}})
		}
	}
	return c.withReplyPreview(ctx, model.Message{ID: string(resp.ID), ChatJID: chat.String(), SenderJID: c.selfJID(), Timestamp: resp.Timestamp.UnixMilli(), Kind: kind, Body: req.Caption, FromMe: true, Status: "sent", ReplyTo: req.ReplyTo, MediaMIME: mimeType, MediaName: filepath.Base(req.Path), MediaPath: localPath, MediaSize: stat.Size(), ForwardingScore: req.ForwardingScore}), nil
}

func (c *Client) cacheOutgoing(sourcePath, chatJID, messageID, kind string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	dir := filepath.Join(c.mediaDir, "sent", kind)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	targetPath := filepath.Join(dir, safeName(chatJID+"-"+messageID)+filepath.Ext(sourcePath))
	tmp, err := os.CreateTemp(dir, "copy-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, source); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (c *Client) SendReaction(ctx context.Context, chatJID, messageID, senderJID, emoji string) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	sender := types.EmptyJID
	if senderJID != "" {
		sender, err = types.ParseJID(senderJID)
		if err != nil {
			return err
		}
	}
	var resp whatsmeow.SendResponse
	if c.sendReactionMessage != nil {
		resp, err = c.sendReactionMessage(ctx, chat, sender, types.MessageID(messageID), emoji)
	} else {
		resp, err = c.wa.SendMessage(ctx, chat, c.wa.BuildReaction(chat, sender, types.MessageID(messageID), emoji))
	}
	if err != nil {
		return err
	}
	timestamp := resp.Timestamp.UnixMilli()
	if resp.Timestamp.IsZero() {
		timestamp = time.Now().UnixMilli()
	}
	return c.recordReaction(ctx, model.Reaction{
		ChatJID: chat.String(), MessageID: messageID,
		SenderJID: c.reactionSenderJID(resp.Sender, true),
		Emoji:     emoji, Timestamp: timestamp,
	})
}

func (c *Client) reactionSenderJID(sender types.JID, fromMe bool) string {
	if fromMe && c.wa != nil && c.wa.Store != nil && c.wa.Store.ID != nil {
		sender = *c.wa.Store.ID
	}
	return sender.ToNonAD().String()
}

func (c *Client) recordReaction(ctx context.Context, reaction model.Reaction) error {
	if err := c.store.UpsertReaction(ctx, reaction); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "message.reaction", Data: reaction})
	return nil
}

func (c *Client) PinMessage(ctx context.Context, chatJID, messageID, senderJID string, duration time.Duration) error {
	if duration != 24*time.Hour && duration != 7*24*time.Hour && duration != 30*24*time.Hour {
		return errors.New("pin duration must be 24 hours, 7 days, or 30 days")
	}
	return c.sendMessagePin(ctx, chatJID, messageID, senderJID, duration, true)
}

func (c *Client) UnpinMessage(ctx context.Context, chatJID, messageID, senderJID string) error {
	return c.sendMessagePin(ctx, chatJID, messageID, senderJID, 0, false)
}

func (c *Client) sendMessagePin(ctx context.Context, chatJID, messageID, senderJID string, duration time.Duration, pinned bool) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	sender := types.EmptyJID
	if senderJID != "" {
		sender, err = types.ParseJID(senderJID)
		if err != nil {
			return err
		}
	}
	action := waE2E.PinInChatMessage_UNPIN_FOR_ALL
	if pinned {
		action = waE2E.PinInChatMessage_PIN_FOR_ALL
	}
	now := time.Now()
	message := &waE2E.Message{
		MessageContextInfo: &waE2E.MessageContextInfo{MessageAddOnDurationInSecs: proto.Uint32(uint32(duration / time.Second))},
		PinInChatMessage: &waE2E.PinInChatMessage{
			Key:  c.wa.BuildMessageKey(chat, sender, types.MessageID(messageID)),
			Type: action.Enum(), SenderTimestampMS: proto.Int64(now.UnixMilli()),
		},
		EventMessage: &waE2E.EventMessage{},
	}
	if _, err = c.wa.SendMessage(ctx, chat, message); err != nil {
		return err
	}
	if pinned {
		err = c.store.SetMessagePinned(ctx, chatJID, messageID, now.Add(duration).UnixMilli())
	} else {
		err = c.store.ClearMessagePin(ctx, chatJID)
	}
	if err == nil {
		c.emit(gateway.Event{Name: "message.pinned", Data: map[string]any{
			"chat_jid": chatJID, "message_id": messageID, "pinned": pinned,
		}})
	}
	return err
}

func (c *Client) EditText(ctx context.Context, chatJID, messageID, text string) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	_, err = c.wa.SendMessage(ctx, chat, c.wa.BuildEdit(chat, types.MessageID(messageID), &waE2E.Message{Conversation: proto.String(text)}))
	return err
}

func (c *Client) DeleteMessage(ctx context.Context, chatJID, messageID, senderJID string) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	sender := types.EmptyJID
	if senderJID != "" {
		sender, err = types.ParseJID(senderJID)
		if err != nil {
			return err
		}
	}
	_, err = c.wa.SendMessage(ctx, chat, c.wa.BuildRevoke(chat, sender, types.MessageID(messageID)))
	return err
}

func (c *Client) ResolvePhone(ctx context.Context, phone string) (model.Chat, error) {
	results, err := c.wa.IsOnWhatsApp(ctx, []string{phone})
	if err != nil {
		return model.Chat{}, err
	}
	for _, result := range results {
		if !result.IsIn {
			continue
		}
		title := result.JID.User
		if info, err := c.wa.Store.Contacts.GetContact(ctx, result.JID); err == nil {
			title = firstNonEmpty(info.FullName, info.FirstName, info.PushName, info.BusinessName, title)
		}
		return model.Chat{JID: result.JID.String(), Title: title}, nil
	}
	return model.Chat{}, errors.New("that phone number is not registered on WhatsApp")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (c *Client) FetchAvatar(ctx context.Context, jidString string) (string, error) {
	return c.fetchAvatar(ctx, jidString, false)
}

func (c *Client) RefreshAvatar(ctx context.Context, jidString string) (string, error) {
	return c.fetchAvatar(ctx, jidString, true)
}

func (c *Client) fetchAvatar(ctx context.Context, jidString string, refresh bool) (string, error) {
	dir := filepath.Join(c.mediaDir, "avatars")
	path := filepath.Join(dir, safeName(jidString)+".jpg")
	if info, err := os.Stat(path); !refresh && err == nil && info.Size() > 0 {
		if rounded, roundErr := roundedAvatar(path); roundErr == nil {
			return rounded, nil
		}
		return path, nil
	}
	jid, err := types.ParseJID(jidString)
	if err != nil {
		return "", err
	}
	var existingID string
	fullIDPath := path + ".full-id"
	if refresh {
		if data, readErr := os.ReadFile(fullIDPath); readErr == nil {
			existingID = strings.TrimSpace(string(data))
		}
	}
	info, err := c.wa.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
		// A refreshed avatar is used by the contact drawer and native viewer,
		// where WhatsApp's 96 px preview is visibly pixelated.
		Preview:    !refresh,
		ExistingID: existingID,
	})
	if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) || errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
		return "", nil
	}
	if err == nil && info == nil && refresh {
		if cached, statErr := os.Stat(path); statErr == nil && cached.Size() > 0 {
			if rounded, roundErr := roundedAvatar(path); roundErr == nil {
				return rounded, nil
			}
			return path, nil
		}
	}
	if err != nil || info == nil || info.URL == "" {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("avatar download returned %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 8*1024*1024 {
		return "", errors.New("avatar exceeds 8 MiB")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "avatar-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", err
	}
	if refresh && info.ID != "" {
		_ = os.WriteFile(fullIDPath, []byte(info.ID+"\n"), 0o600)
	}
	rounded, err := roundedAvatar(path)
	if err != nil {
		return path, nil
	}
	return rounded, nil
}

func roundedAvatar(sourcePath string) (string, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(sourcePath)
	basePath := strings.TrimSuffix(sourcePath, ext)
	// Include the source generation in the returned path. QML image caching is
	// URL-based, so overwriting a stable filename would leave the old avatar on
	// screen even after WhatsApp returned a newer picture.
	targetPath := fmt.Sprintf("%s-round-%d.png", basePath, sourceInfo.ModTime().UnixNano())
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		return targetPath, nil
	}
	for _, stalePath := range avatarDerivedPaths(basePath) {
		if stalePath != targetPath {
			_ = os.Remove(stalePath)
		}
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	source, _, err := image.Decode(sourceFile)
	sourceFile.Close()
	if err != nil {
		return "", err
	}
	bounds := source.Bounds()
	side := bounds.Dx()
	if bounds.Dy() < side {
		side = bounds.Dy()
	}
	if side > 640 {
		side = 640
	}
	if side <= 0 {
		return "", errors.New("avatar has invalid dimensions")
	}

	output := image.NewNRGBA(image.Rect(0, 0, side, side))
	sourceSide := bounds.Dx()
	if bounds.Dy() < sourceSide {
		sourceSide = bounds.Dy()
	}
	startX := bounds.Min.X + (bounds.Dx()-sourceSide)/2
	startY := bounds.Min.Y + (bounds.Dy()-sourceSide)/2
	radius := float64(side) / 2
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			dx := float64(x) + 0.5 - radius
			dy := float64(y) + 0.5 - radius
			if dx*dx+dy*dy > radius*radius {
				continue
			}
			sourceX := startX + x*sourceSide/side
			sourceY := startY + y*sourceSide/side
			output.Set(x, y, source.At(sourceX, sourceY))
		}
	}

	tmp, err := os.CreateTemp(filepath.Dir(targetPath), "avatar-round-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if err := png.Encode(tmp, output); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func avatarDerivedPaths(basePath string) []string {
	paths, err := filepath.Glob(basePath + "-round*.png")
	if err != nil {
		return nil
	}
	return paths
}

func (c *Client) MarkRead(ctx context.Context, chatJID, senderJID string, ids []string, timestamp int64) error {
	return c.markReceipt(ctx, chatJID, senderJID, ids, timestamp)
}

func (c *Client) MarkPlayed(ctx context.Context, chatJID, senderJID, messageID string, timestamp int64) error {
	return c.markReceipt(ctx, chatJID, senderJID, []string{messageID}, timestamp, types.ReceiptTypePlayed)
}

func (c *Client) markReceipt(ctx context.Context, chatJID, senderJID string, ids []string, timestamp int64, receiptType ...types.ReceiptType) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	messageIDs := make([]types.MessageID, len(ids))
	for i, id := range ids {
		messageIDs[i] = types.MessageID(id)
	}
	at := time.UnixMilli(timestamp)
	if timestamp <= 0 {
		at = time.Now()
	}
	sender := types.EmptyJID
	if senderJID != "" {
		sender, err = types.ParseJID(senderJID)
		if err != nil {
			return err
		}
	}
	return c.wa.MarkRead(ctx, messageIDs, at, chat, sender, receiptType...)
}

func (c *Client) SubscribePresence(ctx context.Context, chatJID string) error {
	contact, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	if contact.Server == types.GroupServer || contact.Server == types.BroadcastServer {
		return nil
	}
	return c.wa.SubscribePresence(ctx, contact.ToNonAD())
}

func (c *Client) SetTyping(ctx context.Context, chatJID string, typing bool) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}
	state := types.ChatPresencePaused
	if typing {
		state = types.ChatPresenceComposing
	}
	return c.wa.SendChatPresence(ctx, chat, state, types.ChatPresenceMediaText)
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	for id, waiter := range c.mediaRetries {
		delete(c.mediaRetries, id)
		waiter.err = errors.New("WhatsApp client closed while refreshing media")
		close(waiter.done)
	}
	c.mu.Unlock()
	c.wa.Disconnect()
	return c.container.Close()
}

func (c *Client) selfJID() string {
	if c.wa.Store.ID == nil {
		return ""
	}
	return c.wa.Store.ID.String()
}

func (c *Client) replyContext(ctx context.Context, chatJID, replyTo string) *waE2E.ContextInfo {
	if replyTo == "" {
		return nil
	}
	info := &waE2E.ContextInfo{StanzaID: proto.String(replyTo), RemoteJID: proto.String(chatJID)}
	if quoted, err := c.store.GetMessage(ctx, chatJID, replyTo); err == nil {
		if quoted.SenderJID != "" {
			info.Participant = proto.String(quoted.SenderJID)
		}
		if payload, ok, _ := c.store.MediaPayload(ctx, chatJID, replyTo); ok {
			var raw waE2E.Message
			if proto.Unmarshal(payload, &raw) == nil {
				info.QuotedMessage = &raw
			}
		}
		text := quoted.Body
		if text == "" {
			text = quoted.Kind
		}
		if info.QuotedMessage == nil {
			info.QuotedMessage = &waE2E.Message{Conversation: proto.String(text)}
		}
	}
	return info
}

func detectMIME(f *os.File, path string) string {
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); byExt != "" {
		return byExt
	}
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return http.DetectContentType(buf[:n])
}
func classifyMedia(m string, document bool) (string, whatsmeow.MediaType) {
	if document {
		return "document", whatsmeow.MediaDocument
	}
	switch {
	case strings.HasPrefix(m, "image/"):
		return "image", whatsmeow.MediaImage
	case strings.HasPrefix(m, "video/"):
		return "video", whatsmeow.MediaVideo
	case strings.HasPrefix(m, "audio/"):
		return "audio", whatsmeow.MediaAudio
	default:
		return "document", whatsmeow.MediaDocument
	}
}

func buildMediaPayload(kind, mimeType, name, caption string, u whatsmeow.UploadResponse, ctx *waE2E.ContextInfo, voice bool) *waE2E.Message {
	switch kind {
	case "image":
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{URL: &u.URL, DirectPath: &u.DirectPath, MediaKey: u.MediaKey, FileEncSHA256: u.FileEncSHA256, FileSHA256: u.FileSHA256, FileLength: &u.FileLength, Mimetype: &mimeType, Caption: &caption, ContextInfo: ctx}}
	case "video":
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{URL: &u.URL, DirectPath: &u.DirectPath, MediaKey: u.MediaKey, FileEncSHA256: u.FileEncSHA256, FileSHA256: u.FileSHA256, FileLength: &u.FileLength, Mimetype: &mimeType, Caption: &caption, ContextInfo: ctx}}
	case "audio":
		ptt := voice
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{URL: &u.URL, DirectPath: &u.DirectPath, MediaKey: u.MediaKey, FileEncSHA256: u.FileEncSHA256, FileSHA256: u.FileSHA256, FileLength: &u.FileLength, Mimetype: &mimeType, PTT: &ptt, ContextInfo: ctx}}
	default:
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{URL: &u.URL, DirectPath: &u.DirectPath, MediaKey: u.MediaKey, FileEncSHA256: u.FileEncSHA256, FileSHA256: u.FileSHA256, FileLength: &u.FileLength, Mimetype: &mimeType, FileName: &name, Caption: &caption, ContextInfo: ctx}}
	}
}

// beginConnectSweep claims the right to run the post-connection sweep. It
// reports false when one is already running, so a reconnect does not start a
// second sweep alongside it.
func (c *Client) beginConnectSweep() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connectSweepRunning {
		return false
	}
	c.connectSweepRunning = true
	return true
}

func (c *Client) endConnectSweep() {
	c.mu.Lock()
	c.connectSweepRunning = false
	c.mu.Unlock()
}
