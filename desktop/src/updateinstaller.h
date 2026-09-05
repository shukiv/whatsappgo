#pragma once

#include <QString>

// Applies a downloaded release artifact to this installation.
//
// The daemon finds the release and fetches the file, but it has no idea what
// the window is running from and cannot restart it, so applying one is this
// process's job. What "applying" means depends on how the application was
// installed: an AppImage is one file and is replaced in place, Windows has an
// installer that has to take over, and a macOS disk image is opened for the
// reader to drag across.
namespace updateinstaller {

struct Outcome {
    bool ok = false;
    // The new version is in place and the window should relaunch itself.
    bool restart = false;
    // Another program is taking over and this one has to get out of its way.
    bool quit = false;
    // Why it failed, or what the reader has to do next.
    QString message;
};

// installable reports whether install() can do anything on this machine. A
// build running from a source tree, or an AppImage the reader cannot write to,
// can only be pointed at the release page.
bool installable();

// install applies a file that the daemon downloaded and checked.
Outcome install(const QString &downloadedPath);

}
