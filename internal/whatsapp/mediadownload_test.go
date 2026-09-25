package whatsapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"

	"github.com/shukiv/whatsappgo/internal/model"
	localstore "github.com/shukiv/whatsappgo/internal/store"
)

// fakeMediaHost answers the way whatsmeow's own path download answers,
// including the ways it refuses. Everything it returns was read off
// download-to-file.go, so a test that passes here is a test against the
// library's contract rather than against a convenient stand-in.
type fakeMediaHost struct {
	t      *testing.T
	cipher []byte // what the media host serves
	plain  []byte // what those bytes decrypt to
	key    []byte // the key that decrypts them
	refuse map[string]error
	// fetches counts trips to the host, so a second download shows up.
	fetches int
}

func (h *fakeMediaHost) download(_ context.Context, directPath string, encFileHash, fileHash, mediaKey []byte, mediaType whatsmeow.MediaType, allowNoHash bool, file whatsmeow.File) error {
	h.fetches++
	if err := h.refuse[directPath]; err != nil {
		return err
	}
	if mediaType == "" {
		h.t.Fatalf("download of %q did not say what kind of attachment it is", directPath)
	}
	if info, err := file.Stat(); err != nil {
		h.t.Fatal(err)
	} else if info.Size() != 0 {
		h.t.Fatalf("download of %q started on a file already holding %d bytes", directPath, info.Size())
	}
	// A key with no hash of the encrypted bytes: the library drops the key,
	// downloads the bytes unencrypted and finds no message authentication
	// code to check them with.
	if len(mediaKey) > 0 && len(encFileHash) == 0 {
		return whatsmeow.ErrInvalidMediaHMAC
	}
	served := sha256.Sum256(h.cipher)
	if len(encFileHash) == 0 {
		if _, err := file.Write(h.cipher); err != nil {
			return err
		}
		if allowNoHash && len(fileHash) == 0 {
			return nil
		}
		// Treated as unencrypted: the served bytes are hashed as they are.
		if !bytes.Equal(served[:], fileHash) {
			return whatsmeow.ErrInvalidUnencryptedMediaSHA256
		}
		return nil
	}
	if !bytes.Equal(encFileHash, served[:]) {
		return whatsmeow.ErrInvalidMediaEncSHA256
	}
	if !bytes.Equal(mediaKey, h.key) {
		return whatsmeow.ErrInvalidMediaHMAC
	}
	if _, err := file.Write(h.plain); err != nil {
		return err
	}
	decrypted := sha256.Sum256(h.plain)
	if len(fileHash) > 0 && !bytes.Equal(fileHash, decrypted[:]) {
		return whatsmeow.ErrInvalidMediaSHA256
	}
	return nil
}

// storedMessage puts one media message and its payload in a fresh store.
func storedMessage(t *testing.T, msg model.Message, raw *waE2E.Message) (*localstore.Store, context.Context) {
	t.Helper()
	st, err := localstore.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.UpsertMessage(ctx, msg, "Marta", false); err != nil {
		t.Fatal(err)
	}
	encoded, err := proto.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMediaPayload(ctx, msg.ChatJID, msg.ID, encoded); err != nil {
		t.Fatal(err)
	}
	return st, ctx
}

func TestAttachmentWithoutTheHashOfItsEncryptedBytesStillArrives(t *testing.T) {
	plain := []byte("the picture itself")
	decrypted := sha256.Sum256(plain)
	msg := model.Message{ID: "no-enc-hash", ChatJID: "marta@lid", SenderJID: "marta@lid", Timestamp: 1, Kind: "image", MediaMIME: "image/jpeg", Status: "received"}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		DirectPath: proto.String("/keyed"),
		MediaKey:   []byte("the-key"),
		FileSHA256: decrypted[:],
	}}
	st, ctx := storedMessage(t, msg, raw)
	host := &fakeMediaHost{t: t, cipher: []byte("scrambled bytes"), plain: plain, key: []byte("the-key")}
	c := &Client{store: st, mediaDir: t.TempDir(), downloadPathToFile: host.download}

	result, err := c.downloadMedia(ctx, msg, raw.GetImageMessage(), raw)
	if err != nil {
		t.Fatalf("an attachment missing the hash of its encrypted bytes did not arrive: %v", err)
	}
	data, err := os.ReadFile(result.MediaPath)
	if err != nil || !bytes.Equal(data, plain) {
		t.Fatalf("wrong file on disk: %q err=%v", data, err)
	}
	if host.fetches != 2 {
		t.Fatalf("the hash was learned in %d trips to the host, want 2", host.fetches)
	}
}

func TestAnAttachmentThatArrivesDamagedIsStillRefused(t *testing.T) {
	// The recovered hash must not become a way around the checks the library
	// runs on everything else: these bytes decrypt to something other than
	// what the message says they should.
	wrong := sha256.Sum256([]byte("a different picture"))
	msg := model.Message{ID: "damaged", ChatJID: "marta@lid", SenderJID: "marta@lid", Timestamp: 1, Kind: "image", MediaMIME: "image/jpeg", Status: "received"}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		DirectPath: proto.String("/keyed"),
		MediaKey:   []byte("the-key"),
		FileSHA256: wrong[:],
	}}
	st, ctx := storedMessage(t, msg, raw)
	host := &fakeMediaHost{t: t, cipher: []byte("scrambled bytes"), plain: []byte("the picture itself"), key: []byte("the-key")}
	// The phone is asked once and has nothing better to offer, which is how
	// this ends when an attachment really is beyond saving.
	gone := errors.New("media no longer available on phone")
	c := &Client{
		store: st, mediaDir: t.TempDir(), downloadPathToFile: host.download,
		requestMediaRetryPath: func(context.Context, model.Message, whatsmeow.DownloadableMessage) (string, error) {
			return "", gone
		},
	}

	result, err := c.downloadMedia(ctx, msg, raw.GetImageMessage(), raw)
	if !errors.Is(err, gone) {
		t.Fatalf("a damaged attachment was accepted: %v %#v", err, result)
	}
	if result.MediaPath != "" {
		t.Fatalf("a damaged attachment was written to %q", result.MediaPath)
	}
}

func TestAnOrdinaryAttachmentIsFetchedOnce(t *testing.T) {
	plain := []byte("an ordinary picture")
	decrypted := sha256.Sum256(plain)
	cipher := []byte("scrambled bytes")
	encrypted := sha256.Sum256(cipher)
	msg := model.Message{ID: "ordinary", ChatJID: "marta@lid", SenderJID: "marta@lid", Timestamp: 1, Kind: "image", MediaMIME: "image/jpeg", Status: "received"}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		DirectPath:    proto.String("/ordinary"),
		MediaKey:      []byte("the-key"),
		FileEncSHA256: encrypted[:],
		FileSHA256:    decrypted[:],
	}}
	st, ctx := storedMessage(t, msg, raw)
	host := &fakeMediaHost{t: t, cipher: cipher, plain: plain, key: []byte("the-key")}
	c := &Client{
		store: st, mediaDir: t.TempDir(), downloadPathToFile: host.download,
		// Stand in for the library's own DownloadToFile, which is what an
		// ordinary attachment goes through.
		downloadToFile: func(ctx context.Context, media whatsmeow.DownloadableMessage, file whatsmeow.File) error {
			return host.download(ctx, media.GetDirectPath(), media.GetFileEncSHA256(), media.GetFileSHA256(), media.GetMediaKey(), whatsmeow.GetMediaType(media), false, file)
		},
	}

	result, err := c.downloadMedia(ctx, msg, raw.GetImageMessage(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(result.MediaPath); err != nil || !bytes.Equal(data, plain) {
		t.Fatalf("wrong file on disk: %q err=%v", data, err)
	}
	if host.fetches != 1 {
		t.Fatalf("an ordinary attachment cost %d trips to the host, want 1", host.fetches)
	}
	if needsEncHashRecovery(raw.GetImageMessage()) {
		t.Fatal("an attachment carrying both the key and the hash was treated as missing one")
	}
	if needsEncHashRecovery(&waE2E.ImageMessage{FileSHA256: decrypted[:]}) {
		t.Fatal("an attachment carrying no key at all was treated as needing a key")
	}
}

func TestARefreshedPathAlsoRecoversTheMissingHash(t *testing.T) {
	// The path a media retry hands back is downloaded by a different branch,
	// which used to hit exactly the same wall.
	plain := []byte("the sticker itself")
	decrypted := sha256.Sum256(plain)
	msg := model.Message{ID: "expired-sticker", ChatJID: "marta@lid", SenderJID: "marta@lid", Timestamp: 1, Kind: "image", MediaMIME: "image/webp", Status: "received"}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		DirectPath: proto.String("/expired"),
		MediaKey:   []byte("the-key"),
		FileSHA256: decrypted[:],
	}}
	st, ctx := storedMessage(t, msg, raw)
	host := &fakeMediaHost{
		t: t, cipher: []byte("scrambled bytes"), plain: plain, key: []byte("the-key"),
		refuse: map[string]error{"/expired": whatsmeow.ErrMediaDownloadFailedWith404},
	}
	asked := false
	c := &Client{
		store: st, mediaDir: t.TempDir(), downloadPathToFile: host.download,
		requestMediaRetryPath: func(context.Context, model.Message, whatsmeow.DownloadableMessage) (string, error) {
			asked = true
			return "/refreshed", nil
		},
	}

	result, err := c.downloadMedia(ctx, msg, raw.GetImageMessage(), raw)
	if err != nil {
		t.Fatalf("a refreshed attachment missing the hash did not arrive: %v", err)
	}
	if !asked {
		t.Fatal("the expired path was never refreshed")
	}
	if data, err := os.ReadFile(result.MediaPath); err != nil || !bytes.Equal(data, plain) {
		t.Fatalf("wrong file on disk: %q err=%v", data, err)
	}
}

func TestBytesThatCannotBeOpenedAreAskedForAgain(t *testing.T) {
	// A path can go on serving bytes that the key on the message no longer
	// opens. Hashing them harder will never help; only the phone knows where
	// the attachment really is.
	plain := []byte("the sticker itself")
	decrypted := sha256.Sum256(plain)
	msg := model.Message{ID: "stale-key", ChatJID: "marta@lid", SenderJID: "marta@lid", Timestamp: 1, Kind: "image", MediaMIME: "image/webp", Status: "received"}
	raw := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		DirectPath: proto.String("/stale"),
		MediaKey:   []byte("the-key"),
		FileSHA256: decrypted[:],
	}}
	st, ctx := storedMessage(t, msg, raw)
	host := &fakeMediaHost{
		t: t, cipher: []byte("scrambled bytes"), plain: plain, key: []byte("the-key"),
		refuse: map[string]error{"/stale": whatsmeow.ErrInvalidMediaHMAC},
	}
	asked := false
	c := &Client{
		store: st, mediaDir: t.TempDir(), downloadPathToFile: host.download,
		requestMediaRetryPath: func(context.Context, model.Message, whatsmeow.DownloadableMessage) (string, error) {
			asked = true
			return "/fresh", nil
		},
	}

	result, err := c.downloadMedia(ctx, msg, raw.GetImageMessage(), raw)
	if err != nil {
		t.Fatalf("bytes that would not open were never asked for again: %v", err)
	}
	if !asked {
		t.Fatal("the phone was never asked where the attachment is")
	}
	if data, err := os.ReadFile(result.MediaPath); err != nil || !bytes.Equal(data, plain) {
		t.Fatalf("wrong file on disk: %q err=%v", data, err)
	}
	for _, refusal := range []error{
		whatsmeow.ErrInvalidMediaHMAC, whatsmeow.ErrInvalidMediaSHA256,
		whatsmeow.ErrInvalidMediaEncSHA256, whatsmeow.ErrInvalidUnencryptedMediaSHA256,
		whatsmeow.ErrMediaDownloadFailedWith404,
	} {
		if !isRefreshableMediaDownload(refusal) {
			t.Fatalf("%v was not worth asking the phone about", refusal)
		}
	}
	// A refusal the phone cannot mend must not turn into a request to it.
	if isRefreshableMediaDownload(errors.New("no route to host")) {
		t.Fatal("an unrelated failure was sent to the phone")
	}
}

func TestTheLibraryCannotDownloadAKeyedAttachmentWithNoHash(t *testing.T) {
	// The reason any of this exists. Asking for these bytes the ordinary way
	// fails, whichever way the failure is dressed up.
	host := &fakeMediaHost{t: t, cipher: []byte("scrambled bytes"), plain: []byte("the picture"), key: []byte("the-key")}
	file, err := os.CreateTemp(t.TempDir(), "download-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decrypted := sha256.Sum256(host.plain)
	err = host.download(context.Background(), "/keyed", nil, decrypted[:], host.key, whatsmeow.MediaImage, false, file)
	if !errors.Is(err, whatsmeow.ErrInvalidMediaHMAC) {
		t.Fatalf("the key was kept after all: %v", err)
	}
	// And the way the library actually reaches it, with the key thrown away.
	err = host.download(context.Background(), "/keyed", nil, decrypted[:], nil, whatsmeow.MediaImage, false, file)
	if !errors.Is(err, whatsmeow.ErrInvalidUnencryptedMediaSHA256) {
		t.Fatalf("the served bytes were not hashed as plain ones: %v", err)
	}
}
