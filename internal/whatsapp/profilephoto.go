package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"io"
	"os"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"go.mau.fi/whatsmeow/types"
)

// SetProfilePhoto deliberately has no recipient parameter. An empty target on
// the picture IQ updates our own account, not a group or another contact.
func (c *Client) SetProfilePhoto(ctx context.Context, path string, remove bool) error {
	if (path == "") != remove {
		return errors.New("provide a photo path, or remove=true without a path")
	}
	var data []byte
	if !remove {
		var err error
		data, err = readProfilePhoto(path)
		if err != nil {
			return err
		}
	}
	if c.wa == nil || !c.wa.IsConnected() {
		return errors.New("not connected")
	}
	if _, err := c.wa.SetGroupPhoto(ctx, types.EmptyJID, data); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "profile.changed", Data: map[string]any{"photo": true}})
	return nil
}

func readProfilePhoto(path string) ([]byte, error) {
	const limit = 2 * 1024 * 1024
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("profile photo must be a regular JPEG file under 2 MiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("profile photo must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("profile photo exceeds 2 MiB")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != 640 || config.Height != 640 {
		return nil, errors.New("profile photo must be a prepared 640×640 JPEG")
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		return nil, errors.New("profile photo JPEG is incomplete or invalid")
	}
	return data, nil
}
