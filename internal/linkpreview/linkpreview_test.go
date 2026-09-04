package linkpreview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestResolveBuildsOGPreviewAndThumbnail(t *testing.T) {
	var picture bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 120, G: 40, B: 220, A: 255})
		}
	}
	if err := png.Encode(&picture, img); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "cdn.example" {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(picture.Bytes()))}, nil
		}
		html := `<html><head>
<meta property="og:title" content="Yahoo | Mail, Weather, Search">
<meta property="og:description" content="Latest news coverage and more.">
<meta property="og:image" content="https://cdn.example/card.png">
</head></html>`
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(html))}, nil
	})}
	preview, err := resolveWithClient(context.Background(), "look https://yahoo.com now", client)
	if err != nil {
		t.Fatal(err)
	}
	if preview.URL != "https://yahoo.com" || preview.Title != "Yahoo | Mail, Weather, Search" || preview.Description == "" {
		t.Fatalf("wrong preview: %#v", preview)
	}
	if preview.ThumbnailMIME != "image/jpeg" || len(preview.Thumbnail) == 0 {
		t.Fatalf("thumbnail was not converted to JPEG: %#v", preview)
	}
}

func TestExtractURLTrimsSentencePunctuation(t *testing.T) {
	if got := extractURL("See (https://example.com/path?q=1). Thanks"); got != "https://example.com/path?q=1" {
		t.Fatalf("extracted %q", got)
	}
}

func TestResolveYouTubeUsesOEmbedThumbnail(t *testing.T) {
	var picture bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&picture, img); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "www.youtube.com":
			if req.URL.Path != "/oembed" {
				t.Fatalf("YouTube page was fetched instead of oEmbed: %s", req.URL)
			}
			body := `{"title":"The real video title","author_name":"Example channel","thumbnail_url":"https://i.ytimg.com/vi/ub1O8H02j4E/hqdefault.jpg"}`
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		case "i.ytimg.com":
			if req.URL.Path != "/vi/ub1O8H02j4E/maxresdefault.jpg" {
				t.Fatalf("low-resolution YouTube thumbnail requested: %s", req.URL)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(picture.Bytes())), Request: req}, nil
		default:
			t.Fatalf("unexpected preview request: %s", req.URL)
			return nil, nil
		}
	})}
	preview, err := resolveWithClient(context.Background(), "https://youtu.be/ub1O8H02j4E?si=test", client)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Title != "The real video title" || preview.Description != "Example channel" {
		t.Fatalf("wrong oEmbed metadata: %#v", preview)
	}
	if preview.ThumbnailMIME != "image/jpeg" || len(preview.Thumbnail) == 0 {
		t.Fatalf("oEmbed thumbnail was not cached: %#v", preview)
	}
	decoded, _, err := image.Decode(bytes.NewReader(preview.Thumbnail))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() < 640 || decoded.Bounds().Dy() < 360 {
		t.Fatalf("thumbnail was reduced to %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestResolveYouTubeFallsBackWhenMaxResolutionIsUnavailable(t *testing.T) {
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 480, 360))); err != nil {
		t.Fatal(err)
	}
	maxResolutionRequested := false
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "www.youtube.com":
			body := `{"title":"Video","author_name":"Channel","thumbnail_url":"https://i.ytimg.com/vi/video-id/hqdefault.jpg"}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		case "i.ytimg.com":
			if strings.HasSuffix(req.URL.Path, "/maxresdefault.jpg") {
				maxResolutionRequested = true
				return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("missing")), Request: req}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(picture.Bytes())), Request: req}, nil
		default:
			return nil, io.ErrUnexpectedEOF
		}
	})}
	preview, err := resolveWithClient(context.Background(), "https://youtu.be/video-id", client)
	if err != nil {
		t.Fatal(err)
	}
	if !maxResolutionRequested {
		t.Fatal("max-resolution image was not attempted")
	}
	if len(preview.Thumbnail) == 0 {
		t.Fatal("oEmbed fallback thumbnail was lost")
	}
}

func TestIsPublicIPRejectsAddressesThatMustNotBeReached(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"10.1.2.3", "172.16.0.1", "192.168.1.1", "fd00::1", // private
		"169.254.169.254", "fe80::1", // link-local, incl. cloud metadata
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", // the same, mapped into IPv6
		"0.0.0.0", "0.1.2.3", // this network
		"100.64.0.1", "100.127.255.255", // carrier-grade NAT and overlay networks
		"192.0.0.1", "198.18.0.1", "240.0.0.1", "255.255.255.255", // reserved
		"224.0.0.1", // multicast
	}
	for _, address := range blocked {
		if isPublicIP(net.ParseIP(address)) {
			t.Errorf("%s was treated as a public address", address)
		}
	}
	for _, address := range []string{"1.1.1.1", "8.8.8.8", "93.184.216.34", "2606:4700::1111"} {
		if !isPublicIP(net.ParseIP(address)) {
			t.Errorf("%s should be reachable", address)
		}
	}
	if isPublicIP(nil) {
		t.Error("a missing address was treated as public")
	}
}

func TestPublicClientKeepsAProxyOutAndBoundsHTTP2(t *testing.T) {
	client := publicClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("the preview client does not use an http.Transport")
	}
	// A proxy would connect onward itself, so the private-address check would
	// only ever see the proxy's address.
	if transport.Proxy != nil {
		t.Error("the preview client would honour a proxy from the environment")
	}
	// HTTP/2 is negotiated, because hosts behind some networks refuse an
	// http/1.1-only offer, but its framing layer is bounded: a hostile server
	// can otherwise hold a client goroutine on control frames while no request
	// is outstanding, which the request timeout does not cover.
	if transport.HTTP2 == nil {
		t.Fatal("the preview client leaves HTTP/2 framing unbounded")
	}
	if transport.HTTP2.SendPingTimeout <= 0 || transport.HTTP2.PingTimeout <= 0 {
		t.Error("an HTTP/2 connection that stops answering is never closed")
	}
	if transport.HTTP2.WriteByteTimeout <= 0 {
		t.Error("an HTTP/2 connection that stops reading is never closed")
	}
	if transport.HTTP2.MaxReadFrameSize <= 0 {
		t.Error("a single HTTP/2 frame can make the client buffer without limit")
	}
	if client.Timeout <= 0 {
		t.Error("the preview client has no overall timeout")
	}
}
