package gateway

import "context"

// GroupPhotoEditor is optional; the return value is the confirmed local avatar.
type GroupPhotoEditor interface {
	SetGroupPhoto(context.Context, string, string, bool) (string, error)
}
