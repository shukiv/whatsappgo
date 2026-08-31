package whatsapp

import (
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	waStore "go.mau.fi/whatsmeow/store"
)

const deviceDisplayName = "WhatsAppGo"

// configureDeviceIdentity replaces whatsmeow's library branding in the
// companion registration payload. WhatsApp stores this metadata when a device
// is first linked, so an already-linked entry must be paired again to change.
func configureDeviceIdentity() {
	waStore.SetOSInfo(deviceDisplayName, [3]uint32{0, 1, 0})
	waStore.DeviceProps.PlatformType = waCompanionReg.DeviceProps_DESKTOP.Enum()
}
