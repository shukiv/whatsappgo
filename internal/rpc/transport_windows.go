//go:build windows

package rpc

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

const pipePrefix = `\\.\pipe\`

// pipePath accepts either a bare pipe name or a full native path, so the same
// value can be shared with the Qt client, which takes bare names.
func pipePath(name string) string {
	if strings.HasPrefix(name, pipePrefix) || strings.HasPrefix(name, `\\?\pipe\`) {
		return name
	}
	return pipePrefix + name
}

// listen binds the daemon's named pipe.
//
// Windows has no filesystem permissions to lean on here: the pipe namespace is
// machine-wide and a pipe created with a default security descriptor is
// reachable by every account on the machine. The descriptor below is the
// Windows equivalent of the 0600 socket on Unix - a protected DACL whose only
// entry grants full access to the account running the daemon, so no other user
// and no lower-integrity process can open the pipe and drive the WhatsApp
// session.
//
// ListenPipe asks for the first pipe instance, so a second daemon on the same
// name fails to start rather than silently splitting clients between two
// processes. That mirrors removeStaleSocket's "daemon is already listening"
// on Unix.
func listen(name string) (net.Listener, error) {
	descriptor, err := ownerOnlyDescriptor()
	if err != nil {
		return nil, err
	}
	return winio.ListenPipe(pipePath(name), &winio.PipeConfig{
		SecurityDescriptor: descriptor,
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
}

func dialContext(ctx context.Context, name string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, pipePath(name))
}

// ownerOnlyDescriptor builds "protected DACL, one allow-all entry for the
// current user" in SDDL. P suppresses inherited entries, so nothing an
// administrator has configured on a parent object widens it.
func ownerOnlyDescriptor() (string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return "D:P(A;;GA;;;" + user.User.Sid.String() + ")", nil
}

// watchListener has nothing to watch on Windows. A named pipe has no directory
// entry that a second process can unlink, which is the failure the Unix build
// guards against, and FILE_FLAG_FIRST_PIPE_INSTANCE already prevents a second
// daemon from taking the name. The returned channel never fires.
func watchListener(_ context.Context, _ string, _ time.Duration) <-chan struct{} {
	return make(chan struct{})
}
