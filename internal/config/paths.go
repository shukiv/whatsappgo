package config

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

type Paths struct {
	Profile    string
	DataDir    string
	CacheDir   string
	RuntimeDir string
	DeviceDB   string
	MessageDB  string
	MediaDB    string
	MediaDir   string
	Socket     string
}

func Resolve() (Paths, error) {
	return ResolveProfile("default")
}

var validProfile = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func ResolveProfile(profile string) (Paths, error) {
	if !validProfile.MatchString(profile) {
		return Paths{}, errors.New("profile must contain 1-32 lowercase letters, numbers, underscores, or hyphens")
	}
	// The XDG variables stay honoured on every platform. They are how the
	// tests and the sandboxed runs redirect a whole profile, and honouring
	// them costs nothing where the platform has its own convention.
	dataBase := os.Getenv("XDG_DATA_HOME")
	if dataBase == "" {
		base, err := defaultDataBase()
		if err != nil {
			return Paths{}, err
		}
		dataBase = base
	}
	cacheBase := os.Getenv("XDG_CACHE_HOME")
	if cacheBase == "" {
		base, err := defaultCacheBase()
		if err != nil {
			return Paths{}, err
		}
		cacheBase = base
	}
	runtimeBase, err := runtimeBaseDir()
	if err != nil {
		return Paths{}, err
	}

	dataDir := filepath.Join(dataBase, "whatsappgo")
	cacheDir := filepath.Join(cacheBase, "whatsappgo")
	runtimeDir := filepath.Join(runtimeBase, "whatsappgo")
	if profile != "default" {
		dataDir = filepath.Join(dataDir, "profiles", profile)
		cacheDir = filepath.Join(cacheDir, "profiles", profile)
	}
	return Paths{
		Profile:    profile,
		DataDir:    dataDir,
		CacheDir:   cacheDir,
		RuntimeDir: runtimeDir,
		DeviceDB:   filepath.Join(dataDir, "device.db"),
		MessageDB:  filepath.Join(dataDir, "messages.db"),
		MediaDB:    filepath.Join(dataDir, "media.db"),
		MediaDir:   filepath.Join(cacheDir, "media"),
		Socket:     SocketAddress(runtimeDir, profile),
	}, nil
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.DataDir, p.CacheDir, p.RuntimeDir, p.MediaDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}
