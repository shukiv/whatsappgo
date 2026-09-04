package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// privacyChoices is the set each setting accepts, taken from whatsmeow's own
// documentation of the protocol. A value outside it is rejected here rather
// than sent: WhatsApp answers a bad value with an opaque error, and the reader
// would have no way to tell which setting was at fault.
var privacyChoices = map[types.PrivacySettingType][]types.PrivacySetting{
	types.PrivacySettingTypeLastSeen: {
		types.PrivacySettingAll, types.PrivacySettingContacts,
		types.PrivacySettingContactBlacklist, types.PrivacySettingNone,
	},
	types.PrivacySettingTypeOnline: {
		types.PrivacySettingAll, types.PrivacySettingMatchLastSeen,
	},
	types.PrivacySettingTypeProfile: {
		types.PrivacySettingAll, types.PrivacySettingContacts,
		types.PrivacySettingContactBlacklist, types.PrivacySettingNone,
	},
	types.PrivacySettingTypeStatus: {
		types.PrivacySettingAll, types.PrivacySettingContacts,
		types.PrivacySettingContactBlacklist, types.PrivacySettingNone,
	},
	types.PrivacySettingTypeReadReceipts: {
		types.PrivacySettingAll, types.PrivacySettingNone,
	},
	types.PrivacySettingTypeGroupAdd: {
		types.PrivacySettingAll, types.PrivacySettingContacts,
		types.PrivacySettingContactBlacklist, types.PrivacySettingNone,
	},
}

// privacyNames maps the wire names onto the RPC's own, so the desktop never has
// to know that "last" means last seen or that "groupadd" is one word.
var privacyNames = map[string]types.PrivacySettingType{
	"last_seen":     types.PrivacySettingTypeLastSeen,
	"online":        types.PrivacySettingTypeOnline,
	"profile_photo": types.PrivacySettingTypeProfile,
	"status":        types.PrivacySettingTypeStatus,
	"read_receipts": types.PrivacySettingTypeReadReceipts,
	"group_add":     types.PrivacySettingTypeGroupAdd,
}

func privacyModel(settings types.PrivacySettings) model.PrivacySettings {
	return model.PrivacySettings{
		LastSeen:     string(settings.LastSeen),
		Online:       string(settings.Online),
		ProfilePhoto: string(settings.Profile),
		About:        string(settings.Profile),
		Status:       string(settings.Status),
		ReadReceipts: string(settings.ReadReceipts),
		GroupAdd:     string(settings.GroupAdd),
		CallAdd:      string(settings.CallAdd),
	}
}

// PrivacySettings reads the account's current privacy choices.
func (c *Client) PrivacySettings(ctx context.Context) (model.PrivacySettings, error) {
	if c.wa == nil {
		return model.PrivacySettings{}, errors.New("not connected")
	}
	return privacyModel(c.wa.GetPrivacySettings(ctx)), nil
}

// SetPrivacySetting changes one privacy choice and returns the settings as they
// stand afterwards, which is what WhatsApp answers with.
func (c *Client) SetPrivacySetting(ctx context.Context, name, value string) (model.PrivacySettings, error) {
	setting, ok := privacyNames[strings.TrimSpace(name)]
	if !ok {
		return model.PrivacySettings{}, fmt.Errorf("unknown privacy setting %q", name)
	}
	wanted := types.PrivacySetting(strings.TrimSpace(value))
	allowed := false
	for _, candidate := range privacyChoices[setting] {
		if candidate == wanted {
			allowed = true
			break
		}
	}
	if !allowed {
		return model.PrivacySettings{}, fmt.Errorf("%q is not a value %s accepts", value, name)
	}
	settings, err := c.wa.SetPrivacySetting(ctx, setting, wanted)
	if err != nil {
		return model.PrivacySettings{}, err
	}
	result := privacyModel(settings)
	c.emit(gateway.Event{Name: "privacy.changed", Data: map[string]any{"settings": result}})
	return result, nil
}

// SetAbout changes the account's "about" text, the line WhatsApp shows under a
// profile.
func (c *Client) SetAbout(ctx context.Context, text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return errors.New("about text is required")
	}
	if len([]rune(trimmed)) > 139 {
		return errors.New("about text is limited to 139 characters")
	}
	if err := c.wa.SetStatusMessage(ctx, types.SetStatusInput{Text: &trimmed}); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "profile.changed", Data: map[string]any{"about": trimmed}})
	return nil
}
