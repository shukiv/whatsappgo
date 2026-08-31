package linkpreview

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
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
	img := image.NewRGBA(image.Rect(0, 0, 16, 9))
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
}
