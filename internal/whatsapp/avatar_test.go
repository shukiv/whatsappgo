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
