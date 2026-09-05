package updates

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// SumsAsset is the file the release workflow writes from the assets it
// uploaded. An asset whose checksum is not in it is not installed: this is a
// binary that will be run, fetched over a network, so "GitHub said so" is not
// on its own a good enough reason to execute it.
const SumsAsset = "SHA256SUMS"

// Asset is one file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// defaultHosts is where a GitHub release actually lives. An entry starting
// with a dot matches any host under that domain, which is what the redirect
// from github.com to its object store needs.
var defaultHosts = []string{"github.com", ".githubusercontent.com"}

// maxRedirects is more hops than GitHub has ever needed and few enough that a
// loop ends.
const maxRedirects = 5

// defaultMaxBytes bounds a download. The largest artifact is an AppImage of
// about 120 MB, so this leaves room to grow without letting a bad answer fill
// the disk.
const defaultMaxBytes = 400 << 20

// AssetName is the file this platform installs, exactly as the release
// workflow uploads it. An empty answer means this platform has no release
// artifact and can only be pointed at the release page.
func AssetName(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "linux/amd64":
		return "WhatsAppGo-x86_64.AppImage"
	case "linux/arm64":
		return "WhatsAppGo-aarch64.AppImage"
	case "windows/amd64":
		return "WhatsAppGo-Setup.exe"
	case "darwin/arm64":
		return "WhatsAppGo-arm64.dmg"
	case "darwin/amd64":
		return "WhatsAppGo-amd64.dmg"
	}
	return ""
}

// AssetFor finds the release's artifact for this platform.
func AssetFor(release Release, goos, goarch string) (Asset, bool) {
	name := AssetName(goos, goarch)
	if name == "" {
		return Asset{}, false
	}
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// Downloader fetches a release artifact and proves it is the file the release
// published before letting anybody near it.
type Downloader struct {
	Client   *http.Client
	Dir      string   // where the file is written; created if missing
	Hosts    []string // hosts allowed to serve it; empty means GitHub's
	MaxBytes int64    // refuse anything larger; zero means the default
}

// Fetch downloads this platform's artifact into Dir and returns its path.
//
// Everything about the file comes from a network answer, so nothing about it
// is trusted: the name on disk is the one this build expects rather than the
// one the server offered, the size is capped, every host in the redirect chain
// has to be on the allow list, and the contents have to match the checksum the
// release published. A file that fails any of that is deleted, not returned.
func (d Downloader) Fetch(ctx context.Context, release Release, goos, goarch string,
	progress func(received, total int64)) (string, error) {
	asset, found := AssetFor(release, goos, goarch)
	if !found {
		return "", fmt.Errorf("this release has no download for %s/%s", goos, goarch)
	}
	limit := d.MaxBytes
	if limit <= 0 {
		limit = defaultMaxBytes
	}
	if asset.Size > limit {
		return "", fmt.Errorf("%s is %d bytes, more than this can install", asset.Name, asset.Size)
	}

	client := d.redirectCheckedClient()
	sums, err := d.sums(ctx, client, release)
	if err != nil {
		return "", err
	}
	want, listed := checksumFor(sums, asset.Name)
	if !listed {
		return "", fmt.Errorf("the release does not publish a checksum for %s", asset.Name)
	}

	if err := os.MkdirAll(d.Dir, 0o700); err != nil {
		return "", err
	}
	body, size, err := d.open(ctx, client, asset.URL)
	if err != nil {
		return "", err
	}
	defer body.Close()
	if size > limit {
		return "", fmt.Errorf("%s is %d bytes, more than this can install", asset.Name, size)
	}

	part, err := os.CreateTemp(d.Dir, asset.Name+".part-*")
	if err != nil {
		return "", err
	}
	partPath := part.Name()
	digest, written, err := copyWithProgress(part, io.LimitReader(body, limit+1), size, progress)
	closeErr := part.Close()
	if err == nil && closeErr != nil {
		err = closeErr
	}
	if err == nil && written > limit {
		err = fmt.Errorf("%s is larger than the %d bytes this can install", asset.Name, limit)
	}
	if err == nil && digest != want {
		err = fmt.Errorf("%s does not match the checksum the release published", asset.Name)
	}
	if err != nil {
		_ = os.Remove(partPath)
		return "", err
	}

	// The name on disk is this build's own, never the one the server sent.
	path := filepath.Join(d.Dir, asset.Name)
	if err := os.Chmod(partPath, 0o700); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}
	if err := os.Rename(partPath, path); err != nil {
		_ = os.Remove(partPath)
		return "", err
	}
	return path, nil
}

// sums downloads the release's checksum file.
func (d Downloader) sums(ctx context.Context, client *http.Client, release Release) ([]byte, error) {
	for _, asset := range release.Assets {
		if asset.Name != SumsAsset {
			continue
		}
		body, _, err := d.open(ctx, client, asset.URL)
		if err != nil {
			return nil, err
		}
		defer body.Close()
		// A checksum file for a handful of assets is a few hundred bytes.
		return io.ReadAll(io.LimitReader(body, 64<<10))
	}
	return nil, fmt.Errorf("the release publishes no %s, so nothing can be verified", SumsAsset)
}

// open makes the request, having checked where it is going.
func (d Downloader) open(ctx context.Context, client *http.Client, rawURL string) (io.ReadCloser, int64, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("unusable download address: %w", err)
	}
	if err := d.allow(target); err != nil {
		return nil, 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("User-Agent", "whatsappgo")
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, 0, fmt.Errorf("github answered %s for %s", response.Status, target.Path)
	}
	return response.Body, response.ContentLength, nil
}

// allow reports whether an address may be fetched at all.
func (d Downloader) allow(target *url.URL) error {
	if target.Scheme != "https" {
		return fmt.Errorf("refusing a download over %s", target.Scheme)
	}
	hosts := d.Hosts
	if len(hosts) == 0 {
		hosts = defaultHosts
	}
	host := strings.ToLower(target.Hostname())
	for _, allowed := range hosts {
		switch {
		case strings.HasPrefix(allowed, "."):
			if strings.HasSuffix(host, allowed) {
				return nil
			}
		case strings.Contains(allowed, ":"):
			// An entry with a port has to match the port as well, which is
			// what the tests need to tell two loopback servers apart.
			if strings.ToLower(target.Host) == allowed {
				return nil
			}
		case host == allowed:
			return nil
		}
	}
	return fmt.Errorf("refusing a download from the host %q", target.Host)
}

// redirectCheckedClient copies the caller's client so that every hop of a
// redirect is checked without changing the client the caller handed over.
func (d Downloader) redirectCheckedClient() *http.Client {
	client := &http.Client{}
	if d.Client != nil {
		copied := *d.Client
		client = &copied
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("too many redirects")
		}
		return d.allow(request.URL)
	}
	return client
}

// copyWithProgress writes the body out and hashes it on the way past.
func copyWithProgress(out io.Writer, in io.Reader, total int64,
	progress func(received, total int64)) (string, int64, error) {
	hash := sha256.New()
	buffer := make([]byte, 128<<10)
	var written int64
	for {
		read, err := in.Read(buffer)
		if read > 0 {
			hash.Write(buffer[:read])
			if _, writeErr := out.Write(buffer[:read]); writeErr != nil {
				return "", written, writeErr
			}
			written += int64(read)
			if progress != nil {
				progress(written, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", written, err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), written, nil
}

// checksumFor reads one entry out of a sha256sum file.
func checksumFor(sums []byte, name string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// sha256sum marks a file it read in binary mode with a leading star.
		if strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		digest := strings.ToLower(fields[0])
		if len(digest) != sha256.Size*2 {
			return "", false
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return "", false
		}
		return digest, true
	}
	return "", false
}
