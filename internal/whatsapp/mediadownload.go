package whatsapp

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/url"
	"strings"

	"go.mau.fi/whatsmeow"
)

// mediaPathDownloader fetches one attachment from a known path on the media
// host. It mirrors whatsmeow's own signature so the library method can stand in
// for it directly, and so a test can stand in for the library.
type mediaPathDownloader func(ctx context.Context, directPath string, encFileHash, fileHash, mediaKey []byte, mediaType whatsmeow.MediaType, allowNoHash bool, file whatsmeow.File) error

// needsEncHashRecovery reports whether an attachment arrived with the key its
// bytes were encrypted with but without the hash of those encrypted bytes.
// whatsmeow reads that pair as unencrypted media and throws the key away, then
// hashes the still-encrypted download against the hash of the decrypted file,
// which can never match. Attachments in that state are arranged for here.
func needsEncHashRecovery(media whatsmeow.DownloadableMessage) bool {
	return len(media.GetMediaKey()) > 0 && len(media.GetFileEncSHA256()) == 0
}

// fetchMedia downloads an attachment the ordinary way, unless it is one of the
// attachments the library cannot download on its own.
func (c *Client) fetchMedia(ctx context.Context, media whatsmeow.DownloadableMessage, file whatsmeow.File) error {
	if needsEncHashRecovery(media) {
		return c.downloadFromPath(ctx, media.GetDirectPath(), media, file)
	}
	download := c.downloadToFile
	if download == nil {
		if c.wa == nil {
			return errors.New("WhatsApp is disconnected")
		}
		download = c.wa.DownloadToFile
	}
	return download(ctx, media, file)
}

// downloadFromPath downloads an attachment from a path on the media host, which
// is what a refreshed path from a media retry hands back.
//
// An attachment missing the hash of its encrypted bytes is fetched twice. The
// first fetch asks for the bytes with no key and no hashes, the one arrangement
// the library leaves alone, and exists only to learn that hash. The second
// fetch is the ordinary one, so the key, the message authentication code and
// the hash of the decrypted file are all checked by the library exactly as they
// are for every other attachment.
func (c *Client) downloadFromPath(ctx context.Context, directPath string, media whatsmeow.DownloadableMessage, file whatsmeow.File) error {
	download := c.downloadPathToFile
	if download == nil {
		if c.wa == nil {
			return errors.New("WhatsApp is disconnected")
		}
		download = func(ctx context.Context, directPath string, encFileHash, fileHash, mediaKey []byte, mediaType whatsmeow.MediaType, allowNoHash bool, file whatsmeow.File) error {
			return c.wa.DownloadMediaWithPathToFile(ctx, directPath, encFileHash, fileHash, mediaKey, mediaType, "", allowNoHash, file)
		}
	}
	mediaType := whatsmeow.GetMediaType(media)
	encHash := media.GetFileEncSHA256()
	if needsEncHashRecovery(media) {
		learned, err := encryptedMediaHash(ctx, download, directPath, mediaType, file)
		if err != nil {
			return err
		}
		encHash = learned
	}
	return download(ctx, directPath, encHash, media.GetFileSHA256(), media.GetMediaKey(), mediaType, false, file)
}

// mediaURLPath reports the path on the media host named by the message's own
// url, when that is a different upload from the direct path.
//
// WhatsApp sends both and the library only ever reads the direct path, so the
// url is a second address for the same attachment that nothing was using. The
// two can come apart: a direct path refreshed by the phone points at a fresh
// upload, while the url still points at the one the message's key opens.
func mediaURLPath(media whatsmeow.DownloadableMessage) string {
	addressed, ok := media.(interface{ GetURL() string })
	if !ok {
		return ""
	}
	parsed, err := url.Parse(addressed.GetURL())
	if err != nil || parsed.Path == "" {
		return ""
	}
	path := parsed.EscapedPath()
	// mms3 belongs to the whole url and not to a path on a media host, which
	// is what the library is about to build a url out of.
	kept := make([]string, 0, 8)
	for _, part := range strings.Split(parsed.RawQuery, "&") {
		if part != "" && !strings.HasPrefix(part, "mms3=") {
			kept = append(kept, part)
		}
	}
	if len(kept) > 0 {
		path += "?" + strings.Join(kept, "&")
	}
	if path == media.GetDirectPath() {
		return ""
	}
	return path
}

// emptyFile rewinds an attachment being downloaded, so the next attempt starts
// on a file holding nothing rather than appending to what failed.
func emptyFile(file whatsmeow.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err := file.Seek(0, io.SeekStart)
	return err
}

// encryptedMediaHash reports the hash of the encrypted attachment and leaves
// the file empty, ready for the download that follows.
func encryptedMediaHash(ctx context.Context, download mediaPathDownloader, directPath string, mediaType whatsmeow.MediaType, file whatsmeow.File) ([]byte, error) {
	if err := download(ctx, directPath, nil, nil, nil, mediaType, true, file); err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	if err := emptyFile(file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
