package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewerComparesVersions(t *testing.T) {
	cases := []struct {
		current, candidate string
		want               bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"0.1.0", "0.1.1", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.2.0", "v1.1.9", false},
		{"v0.9.0", "v1.0.0", true},
		// Two digits are not one: 0.10.0 is newer than 0.9.0, and a string
		// comparison says the opposite.
		{"v0.9.0", "v0.10.0", true},
		{"v0.10.0", "v0.9.0", false},
		// A working copy is not a release and is never behind one.
		{"dev", "v9.9.9", false},
		{"v1.0.0", "", false},
		{"v1.0.0", "not-a-version", false},
	}
	for _, c := range cases {
		if got := Newer(c.current, c.candidate); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.current, c.candidate, got, c.want)
		}
	}
}

func TestLatestReadsThePublishedRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/name/releases/latest" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if accept := r.Header.Get("Accept"); accept != "application/vnd.github+json" {
			t.Errorf("unexpected Accept header %q", accept)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://example.test/releases/v1.2.3",
			"body":"  what changed  ","draft":false,"prerelease":false,"published_at":"2026-09-05T10:00:00Z"}`))
	}))
	defer server.Close()

	release, err := Latest(context.Background(), server.Client(), server.URL, "owner/name")
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "v1.2.3" || release.URL != "https://example.test/releases/v1.2.3" || release.Notes != "what changed" {
		t.Fatalf("unexpected release: %#v", release)
	}
	if release.PublishedAt.IsZero() {
		t.Fatal("the release has no publication time")
	}
}

func TestLatestIgnoresDraftsAndPreReleases(t *testing.T) {
	// The release workflow creates a draft and leaves it that way until a
	// person publishes it. Offering an unpublished build to everyone would
	// defeat the point of the draft.
	for _, body := range []string{
		`{"tag_name":"v2.0.0","draft":true}`,
		`{"tag_name":"v2.0.0-rc1","prerelease":true}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		release, err := Latest(context.Background(), server.Client(), server.URL, "owner/name")
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if release.Version != "" {
			t.Fatalf("an unpublished release was reported: %#v", release)
		}
	}
}

func TestLatestTreatsNoReleasesAsNoNews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	release, err := Latest(context.Background(), server.Client(), server.URL, "owner/name")
	if err != nil || release.Version != "" {
		t.Fatalf("a repository without releases should be quiet: %#v, %v", release, err)
	}
}

func TestLatestReportsAServerThatFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := Latest(context.Background(), server.Client(), server.URL, "owner/name"); err == nil {
		t.Fatal("a failing server was reported as success")
	}
}

func TestLatestExplainsASpentAllowance(t *testing.T) {
	reset := time.Now().Add(23 * time.Minute).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded for 203.0.113.7."}`))
	}))
	defer server.Close()

	_, err := Latest(context.Background(), server.Client(), server.URL, "owner/name")
	if err == nil {
		t.Fatal("a refused check reported success")
	}
	until, limited := RateLimitedUntil(err)
	if !limited {
		t.Fatalf("a spent allowance was not recognised: %v", err)
	}
	if until.Unix() != reset {
		t.Fatalf("the wait was read as %v, want %v", until, time.Unix(reset, 0))
	}
	// "github answered 403 Forbidden" tells the reader nothing they can do.
	if !strings.Contains(err.Error(), "try again in about 23 minutes") {
		t.Fatalf("the wait was not explained: %q", err)
	}
}

func TestLatestRepeatsWhatGitHubSaidAboutAFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Repository access blocked"}`))
	}))
	defer server.Close()

	_, err := Latest(context.Background(), server.Client(), server.URL, "owner/name")
	if err == nil {
		t.Fatal("a refused check reported success")
	}
	if _, limited := RateLimitedUntil(err); limited {
		t.Fatalf("a refusal with no allowance headers was read as a rate limit: %v", err)
	}
	if !strings.Contains(err.Error(), "Repository access blocked") {
		t.Fatalf("github's own explanation was dropped: %q", err)
	}
}

func TestLatestHonoursASecondaryLimitsRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := Latest(context.Background(), server.Client(), server.URL, "owner/name")
	until, limited := RateLimitedUntil(err)
	if !limited {
		t.Fatalf("a secondary limit was not recognised: %v", err)
	}
	if wait := time.Until(until); wait < 90*time.Second || wait > 130*time.Second {
		t.Fatalf("the wait GitHub asked for was not kept: %v", wait)
	}
}
