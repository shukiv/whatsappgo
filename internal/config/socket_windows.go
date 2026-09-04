//go:build windows

package config

// SocketAddress returns the address the daemon listens on and the clients dial.
// Windows has no filesystem sockets in the desktop client: Qt's QLocalSocket is
// a named pipe there, so the daemon has to live in the pipe namespace too.
//
// The value is the bare pipe name, without the \\.\pipe\ prefix. Qt's
// QLocalServer takes a bare name and builds the native path itself, and the Go
// listener does the same in internal/rpc, so both ends agree on one string.
//
// The pipe namespace is machine-wide, so the name carries the current user:
// two people signed in at once must not fight over one name. Access control
// does not come from the name — the listener attaches an owner-only security
// descriptor — but a shared name would leave the second daemon unable to start.
func SocketAddress(_, profile string) string {
	name := "whatsappgo-" + PipeUserSegment()
	if profile != "default" {
		name += "-" + profile
	}
	return name
}
