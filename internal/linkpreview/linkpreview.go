package linkpreview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/shukiv/whatsappgo/internal/model"
)

const (
	maxHTMLBytes  = 1 << 20
	maxImageBytes = 5 << 20
	maxRedirects  = 5
)

var webURL = regexp.MustCompile(`https?://[^\s<>"']+`)

// Resolve fetches the first public HTTP(S) URL in text and returns the page's
// Open Graph card. Conversation rendering never calls this: fetching happens
// only while the user is actively composing a link.
func Resolve(ctx context.Context, text string) (model.LinkPreview, error) {
	rawURL := extractURL(text)
	if rawURL == "" {
		return model.LinkPreview{}, nil
	}
	return resolveWithClient(ctx, rawURL, publicClient())
}

func extractURL(text string) string {
	match := webURL.FindString(text)
	return strings.TrimRight(match, ".,;:!?)]}")
}

func resolveWithClient(ctx context.Context, text string, client *http.Client) (model.LinkPreview, error) {
	rawURL := extractURL(text)
	if rawURL == "" {
		return model.LinkPreview{}, nil
	}
	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return model.LinkPreview{}, err
	}
	if err := validateWebURL(pageURL); err != nil {
		return model.LinkPreview{}, err
	}
	if isYouTubeURL(pageURL) {
		return resolveYouTube(ctx, rawURL, client)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return model.LinkPreview{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) WhatsAppGo/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := client.Do(req)
	if err != nil {
		return model.LinkPreview{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.LinkPreview{}, fmt.Errorf("preview page returned %s", resp.Status)
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, maxHTMLBytes))
	if err != nil {
		return model.LinkPreview{}, err
	}
	resolvedPageURL := pageURL
	if resp.Request != nil && resp.Request.URL != nil {
		resolvedPageURL = resp.Request.URL
	}
	preview, imageURL := metadataFromDocument(resolvedPageURL, doc)
	preview.URL = rawURL
	if preview.Title == "" && preview.Description == "" {
		return model.LinkPreview{}, errors.New("page has no preview metadata")
	}
	if imageURL != "" {
		if thumbnail, imageErr := fetchThumbnail(ctx, client, imageURL); imageErr == nil {
			preview.Thumbnail = thumbnail
			preview.ThumbnailMIME = "image/jpeg"
		}
	}
	return preview, nil
}

// IsYouTube reports whether text contains a public YouTube video URL. It is
// exported so the one-time local history migration can remain deliberately
// limited to YouTube instead of contacting every site mentioned in a chat.
func IsYouTube(text string) bool {
	rawURL := extractURL(text)
	parsed, err := url.Parse(rawURL)
	return err == nil && isYouTubeURL(parsed)
}

func isYouTubeURL(parsed *url.URL) bool {
	if parsed == nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	return host == "youtu.be" || host == "youtube.com" || strings.HasSuffix(host, ".youtube.com")
}

func resolveYouTube(ctx context.Context, rawURL string, client *http.Client) (model.LinkPreview, error) {
	endpoint := &url.URL{Scheme: "https", Host: "www.youtube.com", Path: "/oembed"}
	query := endpoint.Query()
	query.Set("url", rawURL)
	query.Set("format", "json")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return model.LinkPreview{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) WhatsAppGo/1.0")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return model.LinkPreview{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.LinkPreview{}, fmt.Errorf("YouTube oEmbed returned %s", resp.Status)
	}
	var metadata struct {
		Title        string `json:"title"`
		AuthorName   string `json:"author_name"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&metadata); err != nil {
		return model.LinkPreview{}, err
	}
	preview := model.LinkPreview{URL: rawURL, Title: cleanText(metadata.Title), Description: cleanText(metadata.AuthorName)}
	if preview.Title == "" {
		return model.LinkPreview{}, errors.New("YouTube oEmbed has no title")
	}
	if metadata.ThumbnailURL != "" {
		if thumbnail, imageErr := fetchThumbnail(ctx, client, metadata.ThumbnailURL); imageErr == nil {
			preview.Thumbnail = thumbnail
			preview.ThumbnailMIME = "image/jpeg"
		}
	}
	return preview, nil
}

func metadataFromDocument(base *url.URL, doc *html.Node) (model.LinkPreview, string) {
	values := make(map[string]string)
	var pageTitle string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "meta" {
			var key, content string
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "property", "name":
					key = strings.ToLower(strings.TrimSpace(attr.Val))
				case "content":
					content = cleanText(attr.Val)
				}
			}
			if key != "" && content != "" && values[key] == "" {
				values[key] = content
			}
		}
		if node.Type == html.ElementNode && node.Data == "title" && node.FirstChild != nil && pageTitle == "" {
			pageTitle = cleanText(node.FirstChild.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	title := first(values["og:title"], values["twitter:title"], pageTitle)
	description := first(values["og:description"], values["twitter:description"], values["description"])
	imageURL := first(values["og:image"], values["og:image:url"], values["twitter:image"])
	if parsed, err := url.Parse(imageURL); err == nil && imageURL != "" {
		imageURL = base.ResolveReference(parsed).String()
	} else {
		imageURL = ""
	}
	return model.LinkPreview{Title: title, Description: description}, imageURL
}

func fetchThumbnail(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if err := validateWebURL(parsed); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) WhatsAppGo/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("preview image returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, errors.New("preview image is too large")
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	thumb := scaledImage(source, 320, 180)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, thumb, &jpeg.Options{Quality: 78}); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func scaledImage(source image.Image, maxWidth, maxHeight int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	scale := min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height), 1)
	targetWidth := max(1, int(float64(width)*scale))
	targetHeight := max(1, int(float64(height)*scale))
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + x*width/targetWidth
			sourceY := bounds.Min.Y + y*height/targetHeight
			target.Set(x, y, color.RGBAModel.Convert(source.At(sourceX, sourceY)))
		}
	}
	return target
}

func publicClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if port != "80" && port != "443" {
			return nil, errors.New("preview URL uses a disallowed port")
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, resolved := range addresses {
			if isPublicIP(resolved.IP) {
				return dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
			}
		}
		return nil, errors.New("preview URL does not resolve to a public address")
	}
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return errors.New("too many preview redirects")
		}
		return validateWebURL(req.URL)
	}
	return client
}

func validateWebURL(parsed *url.URL) error {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return errors.New("preview URL must be HTTP or HTTPS")
	}
	if parsed.User != nil {
		return errors.New("preview URL must not contain credentials")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !isPublicIP(ip) {
		return errors.New("preview URL points to a private address")
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() && !ip.IsMulticast() && !ip.IsUnspecified()
}

func cleanText(value string) string { return strings.Join(strings.Fields(value), " ") }

func first(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
