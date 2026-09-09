package gateway

import (
	"context"
	"time"

	"github.com/shukiv/whatsappgo/internal/model"
)

// ProfileEditor is optional for offline and test gateways.
type ProfileEditor interface {
	SetProfileName(context.Context, string) error
}

// ProfilePhotoEditor changes only the connected account's photo. Removing it
// requires an explicit flag and no path; uploads use a prepared square JPEG.
type ProfilePhotoEditor interface {
	SetProfilePhoto(context.Context, string, bool) error
}

type AccountSettingsReader interface {
	OwnProfile(context.Context) (model.OwnProfile, error)
	StatusAudience(context.Context) (model.StatusAudience, error)
}

type DefaultTimerEditor interface {
	SetDefaultDisappearingTimer(context.Context, time.Duration) error
}
