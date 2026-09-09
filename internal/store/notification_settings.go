package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// NotificationSettings are local to this linked-device profile, not WhatsApp
// account privacy settings. Store each switch independently to avoid lost
// updates when separate clients change different preferences concurrently.
func (s *Store) NotificationSettings(ctx context.Context) (map[string]bool, error) {
	return s.booleanSettings(ctx, "notifications.", notificationDefaults)
}

var notificationDefaults = map[string]bool{
	"messages": true, "groups": true, "previews": true, "sounds": true,
	"messages_sound": true, "groups_sound": true,
	"messages_reactions": false, "groups_reactions": false,
	"outgoing_sound": false, "calls": true, "calls_sound": true,
	"statuses": false, "statuses_sound": true,
	"security": false,
}

func (s *Store) booleanSettings(ctx context.Context, prefix string, defaults map[string]bool) (map[string]bool, error) {
	settings := make(map[string]bool, len(defaults))
	for key, value := range defaults {
		settings[key] = value
	}
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM metadata WHERE key LIKE ?`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(key, prefix)
		if _, known := defaults[name]; !known {
			continue
		}
		on, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid notification preference")
		}
		settings[name] = on
	}
	return settings, rows.Err()
}

func (s *Store) SetNotificationSetting(ctx context.Context, name string, value bool) error {
	if _, ok := notificationDefaults[name]; !ok {
		return fmt.Errorf("unknown notification setting")
	}
	return s.SetMetadata(ctx, "notifications."+name, strconv.FormatBool(value))
}
