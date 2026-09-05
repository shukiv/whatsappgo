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
}

// Latest returns the newest published release of repo ("owner/name").
//
// A draft or a pre-release is not an answer: the release workflow publishes a
// draft first and a person decides when it becomes a release, so a draft is
// precisely the state that must not reach anyone.
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
		return Release{}, fmt.Errorf("github answered %s", response.Status)
	}
	var payload struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		Body        string    `json:"body"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return Release{}, err
	}
	if payload.Draft || payload.Prerelease {
		return Release{}, nil
	}
	return Release{
		Version:     strings.TrimSpace(payload.TagName),
		URL:         strings.TrimSpace(payload.HTMLURL),
		Notes:       strings.TrimSpace(payload.Body),
		PublishedAt: payload.PublishedAt,
	}, nil
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
