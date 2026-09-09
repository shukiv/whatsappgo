package service

import (
	"context"
	"github.com/shukiv/whatsappgo/internal/notify"
	"log"
)

// Called only after a new send succeeds and is stored, never for history,
// delivery receipts, edits, or echoes from other linked devices.
func (s *Service) playOutgoingSound() {
	settings, err := s.store.NotificationSettings(context.Background())
	if err != nil || !settings["outgoing_sound"] {
		return
	}
	go func() {
		if err := notify.PlaySound(context.Background(), "outgoing"); err != nil {
			log.Printf("outgoing sound: %v", err)
		}
	}()
}
