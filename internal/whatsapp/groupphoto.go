package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"go.mau.fi/whatsmeow/types"
)

var _ gateway.GroupPhotoEditor = (*Client)(nil)

type groupPhotoAPI interface {
	SetGroupPhoto(context.Context, types.JID, []byte) (string, error)
}

func (c *Client) applyGroupPhoto(ctx context.Context, api groupPhotoAPI, info *types.GroupInfo, path string, remove bool) ([]byte, string, error) {
	if (path == "") != remove {
		return nil, "", errors.New("provide a photo path, or remove=true without a path")
	}
	if info == nil || info.JID.User == "" || info.JID.Server != types.GroupServer || !c.canEditGroupInfo(info) {
		return nil, "", errors.New("you no longer have permission to edit this group's photo")
	}
	var data []byte
	if !remove {
		var err error
		data, err = readProfilePhoto(path)
		if err != nil {
			return nil, "", err
		}
	}
	id, err := api.SetGroupPhoto(ctx, info.JID, data)
	if err == nil && !remove && id == "" {
		err = errors.New("WhatsApp did not confirm the new photo; reopen group info before trying again")
	}
	return data, id, err
}

// Cache the confirmed bytes, not the caller's original. A fresh rounded filename
// also invalidates QML's URL cache. Removal only touches this group's avatar cache.
func cacheGroupPhoto(mediaDir, jid string, data []byte, id string) (string, error) {
	dir := filepath.Join(mediaDir, "avatars")
	path := filepath.Join(dir, safeName(jid)+".jpg")
	if len(data) == 0 {
		targets := []string{path, path + ".full-id"}
		entries, err := os.ReadDir(dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// Match this exact basename literally: a JID must never become a glob
		// capable of selecting another group's cached photos.
		prefix := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + "-round"
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".png") {
				targets = append(targets, filepath.Join(dir, entry.Name()))
			}
		}
		for _, target := range targets {
			if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
		}
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "group-photo-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	if err = os.WriteFile(path+".full-id", []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}
	return roundedAvatar(path)
}

func (c *Client) SetGroupPhoto(ctx context.Context, jid, path string, remove bool) (string, error) {
	if (path == "") != remove {
		return "", errors.New("provide a photo path, or remove=true without a path")
	}
	info, err := c.readGroup(ctx, jid)
	if err != nil {
		return "", err
	}
	data, id, err := c.applyGroupPhoto(ctx, c.wa, info, path, remove)
	// Even a lost acknowledgement can conceal an applied server change.
	defer c.emit(gateway.Event{Name: "group.updated", Data: map[string]string{"jid": info.JID.String()}})
	if err != nil {
		return "", err
	}
	avatar, err := cacheGroupPhoto(c.mediaDir, info.JID.String(), data, id)
	if err == nil {
		err = c.store.UpdateChatAvatar(ctx, info.JID.String(), avatar)
	}
	if err != nil {
		return "", fmt.Errorf("photo changed on WhatsApp, but the local avatar could not be refreshed: %w", err)
	}
	c.emit(gateway.Event{Name: "chat.updated", Data: map[string]string{"jid": info.JID.String(), "avatar_path": avatar}})
	return avatar, nil
}
