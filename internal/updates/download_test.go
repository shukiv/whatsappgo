package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// releaseServer serves one asset and a SHA256SUMS file over TLS, the way the
// real release does.
type releaseServer struct {
	*httptest.Server
	body     []byte
	sums     string
	name     string
	redirect string
}

func newReleaseServer(t *testing.T, name string, body []byte) *releaseServer {
	t.Helper()
	rs := &releaseServer{body: body, name: name}
	digest := sha256.Sum256(body)
	rs.sums = fmt.Sprintf("%s  %s\n", hex.EncodeToString(digest[:]), name)
	rs.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(rs.sums))
		case "/asset":
			w.Header().Set("Content-Length", fmt.Sprint(len(rs.body)))
			_, _ = w.Write(rs.body)
		case "/redirect":
			http.Redirect(w, r, rs.redirect, http.StatusFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *releaseServer) release() Release {
	return Release{
		Version: "v2.0.0",
		Assets: []Asset{
			{Name: rs.name, URL: rs.URL + "/asset", Size: int64(len(rs.body))},
			{Name: "SHA256SUMS", URL: rs.URL + "/SHA256SUMS", Size: int64(len(rs.sums))},
		},
	}
}

func (rs *releaseServer) downloader(t *testing.T) Downloader {
	t.Helper()
	return Downloader{
		Client: rs.Client(),
		Dir:    t.TempDir(),
		// Two httptest servers share a hostname and differ only by port, so
		// the allow list here carries the port.
		Hosts: []string{strings.TrimPrefix(rs.URL, "https://")},
	}
}

func TestAssetNameCoversEveryPlatformTheReleaseBuilds(t *testing.T) {
	// These names are what the release workflow uploads. If a build script
	// renames an artifact, the updater downloads nothing, and this is where
	// that shows up rather than in somebody's failed update.
	want := map[string]string{
		"linux/amd64":   "WhatsAppGo-x86_64.AppImage",
		"linux/arm64":   "WhatsAppGo-aarch64.AppImage",
		"windows/amd64": "WhatsAppGo-Setup.exe",
		"darwin/arm64":  "WhatsAppGo-arm64.dmg",
		"darwin/amd64":  "WhatsAppGo-amd64.dmg",
	}
	for platform, name := range want {
		parts := strings.Split(platform, "/")
		if got := AssetName(parts[0], parts[1]); got != name {
			t.Errorf("%s: wanted %q, got %q", platform, name, got)
		}
	}
	if AssetName("plan9", "amd64") != "" {
		t.Error("a platform with no release artifact claimed one")
	}

	// The names above are only right while the packaging scripts agree.
	scripts := map[string]string{
		"../../packaging/appimage/build.sh":      "WhatsAppGo-${tool_arch}.AppImage",
		"../../packaging/windows/whatsappgo.iss": "OutputBaseFilename=WhatsAppGo-Setup",
		"../../packaging/macos/build.sh":         "WhatsAppGo-${goarch}.dmg",
		"../../.github/workflows/release.yml":    "WhatsAppGo-windows-x64.zip",
	}
	for path, needle := range scripts {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), needle) {
			t.Errorf("%s no longer contains %q, so the asset names may have moved", path, needle)
		}
	}
}

func TestFetchWritesTheAssetItVerified(t *testing.T) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		t.Skip("no release artifact for this platform")
	}
	server := newReleaseServer(t, name, []byte("an installer, more or less"))
	downloader := server.downloader(t)

	var lastReceived, lastTotal int64
	path, err := downloader.Fetch(context.Background(), server.release(),
		runtime.GOOS, runtime.GOARCH, func(received, total int64) {
			lastReceived, lastTotal = received, total
		})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != name {
		t.Fatalf("wrote %q, wanted the asset's own name", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "an installer, more or less" {
		t.Fatalf("the file does not hold what was served: %q", body)
	}
	if lastReceived != int64(len(body)) || lastTotal != int64(len(body)) {
		t.Fatalf("progress ended at %d/%d", lastReceived, lastTotal)
	}
}

func TestFetchRefusesAnAssetThatDoesNotMatchItsChecksum(t *testing.T) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		t.Skip("no release artifact for this platform")
	}
	server := newReleaseServer(t, name, []byte("the installer that was signed for"))
	// One byte of the served file differs from the one SHA256SUMS describes.
	server.body = []byte("the installer that was signed foR")
	downloader := server.downloader(t)

	path, err := downloader.Fetch(context.Background(), server.release(), runtime.GOOS, runtime.GOARCH, nil)
	if err == nil {
		t.Fatal("a file that does not match its checksum was accepted")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("unhelpful error: %v", err)
	}
	if path != "" {
		t.Fatalf("a rejected download still returned a path: %q", path)
	}
	// Nothing unverified is left behind for anybody to run by accident.
	left, _ := os.ReadDir(downloader.Dir)
	if len(left) != 0 {
		t.Fatalf("the rejected download was kept: %v", left)
	}
}

func TestFetchRefusesAnAssetWithNoChecksumAtAll(t *testing.T) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		t.Skip("no release artifact for this platform")
	}
	server := newReleaseServer(t, name, []byte("body"))
	release := server.release()
	release.Assets = release.Assets[:1] // no SHA256SUMS
	downloader := server.downloader(t)

	if _, err := downloader.Fetch(context.Background(), release, runtime.GOOS, runtime.GOARCH, nil); err == nil {
		t.Fatal("an unverifiable release was accepted")
	}
}

func TestFetchRefusesAHostItWasNotToldToTrust(t *testing.T) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		t.Skip("no release artifact for this platform")
	}
	server := newReleaseServer(t, name, []byte("body"))
	downloader := server.downloader(t)
	downloader.Hosts = []string{"github.com"}

	if _, err := downloader.Fetch(context.Background(), server.release(), runtime.GOOS, runtime.GOARCH, nil); err == nil {
		t.Fatal("an asset was fetched from a host outside the allow list")
	}
}

func TestFetchFollowsGitHubToItsOwnDownloadHostOnly(t *testing.T) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		t.Skip("no release artifact for this platform")
	}
	// github.com answers an asset address with a redirect to its object store,
	// so redirects have to be followed - but only to a host on the list. A
	// release whose asset address has been pointed somewhere else is exactly
	// the case this refuses.
	elsewhere := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("something else entirely"))
	}))
	defer elsewhere.Close()
	server := newReleaseServer(t, name, []byte("body"))
	server.redirect = elsewhere.URL + "/asset"

	release := server.release()
	release.Assets[0].URL = server.URL + "/redirect"
	downloader := server.downloader(t)

	_, err := downloader.Fetch(context.Background(), release, runtime.GOOS, runtime.GOARCH, nil)
	if err == nil {
		t.Fatal("a redirect to an untrusted host was followed")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Fatalf("the redirect failed for some other reason: %v", err)
	}
}

func TestFetchRefusesAnAssetBiggerThanItSaidItWas(t *testing.T) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		t.Skip("no release artifact for this platform")
	}
	server := newReleaseServer(t, name, []byte("body"))
	downloader := server.downloader(t)
	downloader.MaxBytes = 2

	if _, err := downloader.Fetch(context.Background(), server.release(), runtime.GOOS, runtime.GOARCH, nil); err == nil {
		t.Fatal("a download past the size limit was accepted")
	}
}
