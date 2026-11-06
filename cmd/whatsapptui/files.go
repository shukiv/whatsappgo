package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// The kinds of file the desktop client's file dialogs show for each item of
// its attach menu. A document is any file.
var (
	photoAndVideoFiles = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".heic", ".heif",
		".mp4", ".mov", ".mkv", ".webm", ".avi", ".3gp"}
	audioFiles      = []string{".mp3", ".m4a", ".ogg", ".opus", ".wav", ".aac", ".flac"}
	stickerPictures = []string{".jpg", ".jpeg", ".png"}
)

// browsing is a file being chosen: what for, the kinds of file shown, and
// what is done with the one chosen.
type browsing struct {
	title  string
	kind   string   // what the kinds are called, as in "only photos and videos"
	kinds  []string // extensions shown; none means every file
	all    bool     // every file shown after all, as the dialog's "All files"
	hidden bool     // names that start with a dot shown
	chosen func(path string) tea.Cmd
}

// startFolder is where choosing a file starts: the folder the last file came
// from, or the home folder.
func (a *app) startFolder() string {
	if about, err := os.Stat(a.lastFolder); err == nil && about.IsDir() {
		return a.lastFolder
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return string(filepath.Separator)
}

// browse lists a folder to choose a file from, the way the desktop client's
// file dialog does: its folders first, then its files of the kinds asked
// for. A folder opens, backspace goes up to the one it is in, and a file is
// the answer. A path can still be typed instead. The cursor starts on the
// folder named from, which is the one just left when going up.
func (a *app) browse(how browsing, dir, from string) tea.Cmd {
	entries, err := os.ReadDir(dir)
	if err != nil {
		a.failure = safeLine(err.Error())
		return nil
	}
	a.lastFolder = dir
	slices.SortFunc(entries, func(x, y fs.DirEntry) int {
		return strings.Compare(strings.ToLower(x.Name()), strings.ToLower(y.Name()))
	})

	var up func() tea.Cmd
	if parent := filepath.Dir(dir); parent != dir {
		up = func() tea.Cmd { return a.browse(how, parent, filepath.Base(dir)) }
	}
	items := []choice{{label: "Type a path…", run: func() tea.Cmd { return a.typePath(how, dir) }}}
	if up != nil {
		items = append(items, choice{label: "📁 ..", run: up})
	}
	cursor := len(items)
	var files []choice
	for _, entry := range entries {
		name := entry.Name()
		if !how.hidden && strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		folder := entry.IsDir()
		if entry.Type()&fs.ModeSymlink != 0 {
			if about, err := os.Stat(path); err == nil {
				folder = about.IsDir()
			}
		}
		// A name is whatever somebody called the file, so it is cleaned
		// before it is written to the terminal; the path itself is not.
		shown := safeLine(name)
		if folder {
			if name == from {
				cursor = len(items)
			}
			items = append(items, choice{label: "📁 " + shown + "/", run: func() tea.Cmd {
				return a.openFolder(how, dir, path)
			}})
			continue
		}
		if len(how.kinds) > 0 && !how.all && !slices.Contains(how.kinds, strings.ToLower(filepath.Ext(name))) {
			continue
		}
		label := "📄 " + shown
		if about, err := entry.Info(); err == nil && about.Size() > 0 {
			label += "  " + humanSize(about.Size())
		}
		files = append(files, choice{label: label, run: func() tea.Cmd { return how.chosen(path) }})
	}
	items = append(items, files...)
	if cursor >= len(items) {
		cursor = len(items) - 1
	}

	// The dialog's two switches, at the foot of the list for the mouse and
	// on keys of their own.
	everyFile := func() tea.Cmd {
		other := how
		other.all = !how.all
		return a.browse(other, dir, "")
	}
	hiddenFiles := func() tea.Cmd {
		other := how
		other.hidden = !how.hidden
		return a.browse(other, dir, "")
	}
	if len(how.kinds) > 0 {
		label := "Show every file"
		if how.all {
			label = "Show only " + how.kind
		}
		items = append(items, choice{label: label, run: everyFile})
	}
	label := "Show hidden files"
	if how.hidden {
		label = "Hide hidden files"
	}
	items = append(items, choice{label: label, run: hiddenFiles})

	hint := safeLine(dir)
	if len(files) == 0 {
		what := "files"
		if len(how.kinds) > 0 && !how.all {
			what = how.kind
		}
		hint += " · no " + what + " here"
	}
	a.ask(how.title, hint, cursor, items...)
	a.chooser.leftToRight = true
	a.chooser.keys = map[string]func() tea.Cmd{".": hiddenFiles}
	a.chooser.footer = "enter open · . hidden files · j/k move · esc back"
	if up != nil {
		for _, key := range []string{"backspace", "left", "h"} {
			a.chooser.keys[key] = up
		}
		a.chooser.footer = "enter open · backspace up · . hidden files · j/k move · esc back"
	}
	return nil
}

// openFolder goes into a folder, or says why it cannot and stays where it was.
func (a *app) openFolder(how browsing, dir, folder string) tea.Cmd {
	if _, err := os.ReadDir(folder); err != nil {
		a.failure = safeLine(err.Error())
		return a.browse(how, dir, filepath.Base(folder))
	}
	return a.browse(how, folder, "")
}

// typePath asks for the path instead, starting from the folder being looked
// at. A path that does not start at the root is taken from that folder, one
// that starts with ~ from the home folder, and a folder typed is opened.
func (a *app) typePath(how browsing, dir string) tea.Cmd {
	// The folder's path is put in the box to be added to, unless a name in it
	// would not be shown as it is; a path typed is taken from that folder
	// all the same.
	start := strings.TrimSuffix(dir, string(filepath.Separator)) + string(filepath.Separator)
	if cleanLine(start) != start {
		start = ""
	}
	return a.type_(how.title, "The whole path to it.", start,
		func(path string) tea.Cmd {
			if path == "" {
				return nil
			}
			if rest, ok := strings.CutPrefix(path, "~/"); ok {
				if home, err := os.UserHomeDir(); err == nil {
					path = filepath.Join(home, rest)
				}
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			if about, err := os.Stat(path); err == nil && about.IsDir() {
				return a.openFolder(how, dir, filepath.Clean(path))
			}
			return how.chosen(path)
		})
}
