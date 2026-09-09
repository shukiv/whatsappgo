package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"

	"github.com/shukiv/whatsappgo/internal/gateway"
	"github.com/shukiv/whatsappgo/internal/model"
)

// privacyChoices is the set each setting accepts, taken from whatsmeow's own
// documentation of the protocol. A value outside it is rejected here rather
// than sent: WhatsApp answers a bad value with an opaque error, and the reader
// would have no way to tell which setting was at fault.
var privacyChoices = map[types.PrivacySettingType][]types.PrivacySetting{
	types.PrivacySettingTypeCallAdd:  {types.PrivacySettingAll, types.PrivacySettingKnown},
	types.PrivacySettingTypeMessages: {types.PrivacySettingAll, types.PrivacySettingContacts},
	types.PrivacySettingTypeDefense:  {types.PrivacySettingOnStandard, types.PrivacySettingOff},
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
	"about":         types.PrivacySettingTypeStatus,
	"read_receipts": types.PrivacySettingTypeReadReceipts,
	"group_add":     types.PrivacySettingTypeGroupAdd,
	"call_add":      types.PrivacySettingTypeCallAdd,
	"messages":      types.PrivacySettingTypeMessages,
	"defense":       types.PrivacySettingTypeDefense,
}

func privacyModel(settings types.PrivacySettings) model.PrivacySettings {
	return model.PrivacySettings{
		LastSeen:     string(settings.LastSeen),
		Online:       string(settings.Online),
		ProfilePhoto: string(settings.Profile),
		Status:       string(settings.Status),
		About:        string(settings.Status),
		ReadReceipts: string(settings.ReadReceipts),
		GroupAdd:     string(settings.GroupAdd),
		CallAdd:      string(settings.CallAdd),
		Messages:     string(settings.Messages),
		Defense:      string(settings.Defense),
	}
}

func (c *Client) SetProfileName(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 25 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return errors.New("profile name must be one line with 1–25 characters")
	}
	if c.wa == nil || !c.wa.IsConnected() {
		return errors.New("not connected")
	}
	if err := c.wa.SendAppState(ctx, appstate.BuildSettingPushName(name)); err != nil {
		return err
	}
	c.setStatus(func(s *model.ConnectionStatus) { s.UserName = name })
	return nil
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
	if c.wa == nil || !c.wa.IsConnected() {
		return model.PrivacySettings{}, errors.New("not connected")
	}
	// Selecting this value without an exception-list editor can reuse an old
	// list invisibly. Preserve reading it, but do not offer a misleading edit.
	if wanted == types.PrivacySettingContactBlacklist {
		return model.PrivacySettings{}, errors.New("edit privacy exception lists in WhatsApp on your phone")
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
	if c.wa == nil || !c.wa.IsConnected() {
		return errors.New("not connected")
	}
	if err := c.wa.SetStatusMessage(ctx, types.SetStatusInput{Text: &trimmed}); err != nil {
		return err
	}
	c.emit(gateway.Event{Name: "profile.changed", Data: map[string]any{"about": trimmed}})
	return nil
}

func (c *Client) OwnProfile(ctx context.Context) (model.OwnProfile, error) {
	if c.wa == nil || !c.wa.IsConnected() {
		return model.OwnProfile{}, errors.New("not connected")
	}
	jid, err := types.ParseJID(c.selfJID())
	if err != nil {
		return model.OwnProfile{}, err
	}
	jid = jid.ToNonAD()
	info, err := c.wa.GetUserInfo(ctx, []types.JID{jid})
	if err != nil {
		return model.OwnProfile{}, err
	}
	user, found := info[jid]
	if !found {
		return model.OwnProfile{}, errors.New("profile information was not returned")
	}
	profile := model.OwnProfile{Name: c.Status().UserName, About: user.Status}
	// No photo / privacy denial should not prevent reading the name and About.
	profile.AvatarPath, _ = c.RefreshAvatar(ctx, jid.String())
	return profile, nil
}

func (c *Client) StatusAudience(ctx context.Context) (model.StatusAudience, error) {
	if c.wa == nil || !c.wa.IsConnected() {
		return model.StatusAudience{}, errors.New("not connected")
	}
	lists, err := c.wa.GetStatusPrivacy(ctx)
	if err != nil {
		return model.StatusAudience{}, err
	}
	if len(lists) == 0 {
		return model.StatusAudience{}, errors.New("status audience was not returned")
	}
	result := model.StatusAudience{Type: string(lists[0].Type), JIDs: []string{}}
	for _, jid := range lists[0].List {
		result.JIDs = append(result.JIDs, jid.ToNonAD().String())
	}
	return result, nil
}

func (c *Client) SetDefaultDisappearingTimer(ctx context.Context, duration time.Duration) error {
	if duration != 0 && duration != 24*time.Hour && duration != 7*24*time.Hour && duration != 90*24*time.Hour {
		return errors.New("default timer must be off, 24 hours, 7 days or 90 days")
	}
	if c.wa == nil || !c.wa.IsConnected() {
		return errors.New("not connected")
	}
	return c.wa.SetDefaultDisappearingTimer(ctx, duration)
}
