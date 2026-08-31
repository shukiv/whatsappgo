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
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	dataBase := os.Getenv("XDG_DATA_HOME")
	if dataBase == "" {
		dataBase = filepath.Join(home, ".local", "share")
	}
	cacheBase := os.Getenv("XDG_CACHE_HOME")
	if cacheBase == "" {
		cacheBase = filepath.Join(home, ".cache")
	}
	runtimeBase := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeBase == "" {
		return Paths{}, errors.New("XDG_RUNTIME_DIR is not set; a per-user runtime directory is required")
	}

	dataDir := filepath.Join(dataBase, "whatsappgo")
	cacheDir := filepath.Join(cacheBase, "whatsappgo")
	runtimeDir := filepath.Join(runtimeBase, "whatsappgo")
	socketName := "whatsappd.sock"
	if profile != "default" {
		dataDir = filepath.Join(dataDir, "profiles", profile)
		cacheDir = filepath.Join(cacheDir, "profiles", profile)
		socketName = "whatsappd-" + profile + ".sock"
	}
	return Paths{
		Profile:    profile,
		DataDir:    dataDir,
		CacheDir:   cacheDir,
		RuntimeDir: runtimeDir,
		DeviceDB:   filepath.Join(dataDir, "device.db"),
		MessageDB:  filepath.Join(dataDir, "messages.db"),
		MediaDir:   filepath.Join(cacheDir, "media"),
		Socket:     filepath.Join(runtimeDir, socketName),
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
