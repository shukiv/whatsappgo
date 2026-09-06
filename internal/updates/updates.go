// Package updates reports whether a newer WhatsAppGo release exists.
//
// The check asks GitHub for the newest published release of the project's own
// repository and compares its tag with the version this build was stamped
// with. Nothing is downloaded and nothing is installed here: the daemon only
// learns that a newer version exists, and the desktop asks the reader what to
// do about it.
package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Interval is how often the daemon looks. GitHub allows sixty anonymous
// requests an hour from one address, so this is far below anything it minds,
// and a release that is three hours old is still news.
const Interval = 3 * time.Hour

// Release is the newest release of the project, as GitHub describes it.
type Release struct {
	Version     string    `json:"version"`
	URL         string    `json:"url"`
	Notes       string    `json:"notes"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets,omitempty"`
}

// Latest returns the newest published release of repo ("owner/name").
//
// A draft or a pre-release is not an answer: the release workflow publishes a
// draft first and a person decides when it becomes a release, so a draft is
// precisely the state that must not reach anyone.
// maxReleaseBytes bounds the release description GitHub sends back.
const maxReleaseBytes = 1 << 20

func Latest(ctx context.Context, client *http.Client, baseURL, repo string) (Release, error) {
	if repo == "" {
		return Release{}, errors.New("a repository is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimSuffix(baseURL, "/"), repo), nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "whatsappgo")
	response, err := client.Do(request)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		// A repository with no published release yet, which is not a failure
		// worth reporting to anyone.
		return Release{}, nil
	}
	if response.StatusCode != http.StatusOK {
		return Release{}, describeFailure(response)
	}
	var payload struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		Body        string    `json:"body"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	// A release description is a few kilobytes. Reading whatever arrives
	// would let one bad answer grow this process without limit.
	if err := json.NewDecoder(io.LimitReader(response.Body, maxReleaseBytes)).Decode(&payload); err != nil {
		return Release{}, err
	}
	if payload.Draft || payload.Prerelease {
		return Release{}, nil
	}
	release := Release{
		Version:     strings.TrimSpace(payload.TagName),
		URL:         strings.TrimSpace(payload.HTMLURL),
		Notes:       strings.TrimSpace(payload.Body),
		PublishedAt: payload.PublishedAt,
	}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, Asset{
			Name: strings.TrimSpace(asset.Name),
			URL:  strings.TrimSpace(asset.URL),
			Size: asset.Size,
		})
	}
	return release, nil
}

// RateLimitError says that GitHub is refusing checks from this address for a
// while. It is not a fault in the application or a reason to alarm anybody:
// sixty anonymous requests an hour are shared by everyone behind one address,
// and the only answer is to wait.
type RateLimitError struct {
	// Until is when GitHub says it will answer again. Zero when it did not
	// say, which is treated as a short wait rather than an unknown one.
	Until time.Time
}

func (e *RateLimitError) Error() string {
	wait := time.Until(e.Until).Round(time.Minute)
	if e.Until.IsZero() || wait <= 0 {
		return "GitHub is limiting checks from this address; try again shortly"
	}
	return fmt.Sprintf("GitHub is limiting checks from this address; try again in about %s",
		humanDuration(wait))
}

// RateLimitedUntil reports when a failed check may be retried.
func RateLimitedUntil(err error) (time.Time, bool) {
	var limited *RateLimitError
	if !errors.As(err, &limited) {
		return time.Time{}, false
	}
	return limited.Until, true
}

func humanDuration(wait time.Duration) string {
	if wait < time.Hour {
		minutes := int(wait.Minutes())
		if minutes <= 1 {
			return "a minute"
		}
		return strconv.Itoa(minutes) + " minutes"
	}
	hours := int((wait + 30*time.Minute).Hours())
	if hours <= 1 {
		return "an hour"
	}
	return strconv.Itoa(hours) + " hours"
}

// describeFailure turns an answer that is not a release into an error worth
// showing. GitHub says "403 Forbidden" for an exhausted allowance, which reads
// like the application was refused rather than asked to wait.
func describeFailure(response *http.Response) error {
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
		if until, limited := rateLimitReset(response.Header); limited {
			return &RateLimitError{Until: until}
		}
	}
	// GitHub explains itself in the body; "403 Forbidden" alone tells the
	// reader nothing they can act on.
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxReleaseBytes)).Decode(&payload); err == nil {
		if explanation := strings.TrimSpace(payload.Message); explanation != "" {
			return fmt.Errorf("github answered %s: %s", response.Status, explanation)
		}
	}
	return fmt.Errorf("github answered %s", response.Status)
}

// rateLimitReset reads the headers GitHub answers a spent allowance with. The
// primary limit reports no requests remaining and when the window turns over;
// the secondary one asks for a number of seconds instead.
func rateLimitReset(header http.Header) (time.Time, bool) {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header.Get("Retry-After"))); err == nil && seconds >= 0 {
		return time.Now().Add(time.Duration(seconds) * time.Second), true
	}
	if strings.TrimSpace(header.Get("X-RateLimit-Remaining")) != "0" {
		return time.Time{}, false
	}
	reset, err := strconv.ParseInt(strings.TrimSpace(header.Get("X-RateLimit-Reset")), 10, 64)
	if err != nil || reset <= 0 {
		// Refused for running out of requests, without saying when that ends.
		return time.Time{}, true
	}
	return time.Unix(reset, 0), true
}

// Newer reports whether candidate is a later version than current.
//
// A build that was not stamped with a version - "dev", or anything else that
// does not parse - is a working copy, and telling somebody who is running
// their own build to update is noise.
func Newer(current, candidate string) bool {
	currentParts, currentOK := parse(current)
	candidateParts, candidateOK := parse(candidate)
	if !currentOK || !candidateOK {
		return false
	}
	for i := range currentParts {
		if candidateParts[i] != currentParts[i] {
			return candidateParts[i] > currentParts[i]
		}
	}
	return false
}

// parse reads major.minor.patch out of a tag, with or without its leading v
// and with or without a suffix like -rc1, which is ignored: a suffix marks a
// pre-release, and Latest never returns one.
func parse(version string) ([3]int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if trimmed == "" {
		return [3]int{}, false
	}
	if cut := strings.IndexAny(trimmed, "-+"); cut >= 0 {
		trimmed = trimmed[:cut]
	}
	fields := strings.Split(trimmed, ".")
	if len(fields) > 3 {
		return [3]int{}, false
	}
	var parsed [3]int
	for i, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 0 {
			return [3]int{}, false
		}
		parsed[i] = value
	}
	return parsed, true
}
