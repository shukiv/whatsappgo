package store

import (
	"context"
	"fmt"
	"strconv"
)

var localDefaults = map[string]bool{
	"download_image": true, "download_video": true, "download_audio": true,
	"download_document": true, "download_sticker": true,
	"disable_link_previews": false,
}

// LocalSettings controls network work on this computer, not account privacy.
func (s *Store) LocalSettings(ctx context.Context) (map[string]bool, error) {
	return s.booleanSettings(ctx, "preferences.", localDefaults)
}

func (s *Store) SetLocalSetting(ctx context.Context, name string, value bool) error {
	if _, ok := localDefaults[name]; !ok {
		return fmt.Errorf("unknown local setting")
	}
	return s.SetMetadata(ctx, "preferences."+name, strconv.FormatBool(value))
}

func (s *Store) AutoDownloadAllowed(ctx context.Context, kind string) bool {
	settings, err := s.LocalSettings(ctx)
	return err == nil && settings["download_"+kind]
}

func (s *Store) LinkPreviewsAllowed(ctx context.Context) bool {
	settings, err := s.LocalSettings(ctx)
	return err == nil && !settings["disable_link_previews"]
}
