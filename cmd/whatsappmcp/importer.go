package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shukiv/whatsappgo/internal/config"
	"github.com/shukiv/whatsappgo/internal/profile"
)

// importProfile installs an exported account on this machine, which is how a
// profile reaches a machine that never paired one: the account is exported
// where it lives, moved here, imported under a profile name, and then served
// by a daemon this command starts itself.
//
// It is a flag rather than a tool. Importing replaces the files a running
// daemon would have open, and it is a decision about a credential - neither
// is something a model should be able to do mid-conversation.
func importProfile(archive, name string, force bool) error {
	paths, err := config.ResolveProfile(name)
	if err != nil {
		return err
	}
	// A daemon already serving this profile has the databases open, and would
	// keep writing the ones being replaced.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if client, err := dialOnce(ctx, paths.Socket, 2*time.Second); err == nil {
		client.Close()
		return fmt.Errorf("a daemon is already serving profile %q; stop it before importing", name)
	}
	manifest, err := profile.Import(archive, paths, force)
	if err != nil {
		return err
	}
	kept := "history and identity"
	if manifest.Media {
		kept = "history, identity and attachments"
	}
	// This goes to stderr with everything else this command says about
	// itself: stdout carries the protocol and nothing else.
	fmt.Fprintf(os.Stderr, "imported profile %q (%s) into %s\n%s\n",
		name, kept, paths.DataDir, profile.Warning)
	return nil
}
