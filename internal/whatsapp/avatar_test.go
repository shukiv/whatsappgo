package whatsapp

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundedAvatarCreatesTransparentCorners(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "avatar.png")
	source := image.NewNRGBA(image.Rect(0, 0, 40, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			source.Set(x, y, color.NRGBA{R: 200, A: 255})
		}
	}
	file, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, source); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	roundedPath, err := roundedAvatar(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	roundedFile, err := os.Open(roundedPath)
	if err != nil {
		t.Fatal(err)
	}
	rounded, err := png.Decode(roundedFile)
	roundedFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := rounded.Bounds().Size(); got.X != 30 || got.Y != 30 {
		t.Fatalf("unexpected dimensions: %v", got)
	}
	_, _, _, cornerAlpha := rounded.At(0, 0).RGBA()
	_, _, _, centerAlpha := rounded.At(15, 15).RGBA()
	if cornerAlpha != 0 || centerAlpha == 0 {
		t.Fatalf("unexpected alpha values: corner=%d center=%d", cornerAlpha, centerAlpha)
	}
}

func TestRoundedAvatarPathChangesWhenTheSourceIsRefreshed(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "avatar.png")
	writeAvatar := func(fill color.NRGBA) {
		t.Helper()
		file, err := os.Create(sourcePath)
		if err != nil {
			t.Fatal(err)
		}
		imageData := image.NewNRGBA(image.Rect(0, 0, 20, 20))
		for y := 0; y < 20; y++ {
			for x := 0; x < 20; x++ {
				imageData.Set(x, y, fill)
			}
		}
		if err := png.Encode(file, imageData); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	writeAvatar(color.NRGBA{R: 255, A: 255})
	first, err := roundedAvatar(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	writeAvatar(color.NRGBA{B: 255, A: 255})
	refreshedAt := time.Now().Add(time.Second)
	if err := os.Chtimes(sourcePath, refreshedAt, refreshedAt); err != nil {
		t.Fatal(err)
	}
	second, err := roundedAvatar(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("refreshed avatar kept the same cache URL: %q", first)
	}
}

func TestRoundedAvatarKeepsEnoughResolutionForTheProfileDrawer(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "large-avatar.png")
	source := image.NewNRGBA(image.Rect(0, 0, 480, 420))
	for y := 0; y < 420; y++ {
		for x := 0; x < 480; x++ {
			source.Set(x, y, color.NRGBA{G: 180, B: 120, A: 255})
		}
	}
	file, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, source); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	roundedPath, err := roundedAvatar(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	roundedFile, err := os.Open(roundedPath)
	if err != nil {
		t.Fatal(err)
	}
	rounded, err := png.Decode(roundedFile)
	roundedFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := rounded.Bounds().Dx(); got < 400 {
		t.Fatalf("profile avatar was reduced to %dpx", got)
	}
}

// A profile picture belongs to a person, not to one of their phones. A status
// sender arrives with its device still attached, and asking WhatsApp for that
// address is answered with "info query returned status 400: bad-request",
// which the window drew in red across the conversation.
func TestAvatarIsAskedForThePersonNotTheDevice(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		asked string
		want  string
	}{
		{"a status sender carries its device", "114916346343611:17@lid", "114916346343611@lid"},
		{"a phone number with a device", "972500000000:3@s.whatsapp.net", "972500000000@s.whatsapp.net"},
		{"a plain contact is left alone", "972500000000@s.whatsapp.net", "972500000000@s.whatsapp.net"},
		{"a group is left alone", "120363000000000000@g.us", "120363000000000000@g.us"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			subject, err := avatarSubject(testCase.asked)
			if err != nil {
				t.Fatal(err)
			}
			if got := subject.String(); got != testCase.want {
				t.Fatalf("asked WhatsApp for %q, wanted %q", got, testCase.want)
			}
			// Both spellings have to reach one cached file, or the same
			// picture is downloaded and stored twice.
			plain, err := avatarSubject(testCase.want)
			if err != nil {
				t.Fatal(err)
			}
			if safeName(subject.String()) != safeName(plain.String()) {
				t.Fatalf("%q and %q are cached apart", testCase.asked, testCase.want)
			}
		})
	}
	// An address the library cannot read is handed back as an error rather
	// than asked for. A bare word is not one of those: ParseJID reads it as a
	// server with no user, so the malformed case here is a real one.
	if _, err := avatarSubject("972500000000.1.2@s.whatsapp.net"); err == nil {
		t.Fatal("an address that cannot be read was accepted")
	}
}
