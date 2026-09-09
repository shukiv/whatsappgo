//go:build !linux

package notify

import (
	"context"
	"errors"
)

func PlaySound(context.Context, string) error {
	return errors.New("sound preview and outgoing sounds are currently supported on Linux")
}
