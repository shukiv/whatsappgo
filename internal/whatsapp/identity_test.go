package whatsapp

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waWa6"
	waStore "go.mau.fi/whatsmeow/store"
)

func TestConfigureDeviceIdentityBrandsNewPairings(t *testing.T) {
	originalDeviceProps := waStore.DeviceProps
	originalBasePayload := waStore.BaseClientPayload
	waStore.DeviceProps = proto.Clone(originalDeviceProps).(*waCompanionReg.DeviceProps)
	waStore.BaseClientPayload = proto.Clone(originalBasePayload).(*waWa6.ClientPayload)
	t.Cleanup(func() {
		waStore.DeviceProps = originalDeviceProps
		waStore.BaseClientPayload = originalBasePayload
	})

	configureDeviceIdentity()

	if got := waStore.DeviceProps.GetOs(); got != "WhatsAppGo" {
		t.Fatalf("pairing device name = %q, want WhatsAppGo", got)
	}
	if got := waStore.DeviceProps.GetPlatformType(); got != waCompanionReg.DeviceProps_DESKTOP {
		t.Fatalf("pairing platform = %s, want DESKTOP", got)
	}
	version := waStore.DeviceProps.GetVersion()
	if version.GetPrimary() != 0 || version.GetSecondary() != 1 || version.GetTertiary() != 0 {
		t.Fatalf("pairing version = %d.%d.%d, want 0.1.0", version.GetPrimary(), version.GetSecondary(), version.GetTertiary())
	}
}
