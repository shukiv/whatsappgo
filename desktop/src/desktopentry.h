#pragma once

#include <QString>
#include <QStringList>

// A running window is drawn by the desktop shell, and the shell draws it from a
// launcher entry rather than from the window. Qt hands the window the icon this
// program carries, which is enough for a taskbar that reads the window itself,
// and not enough for a shell that looks the application up by name: with no
// entry to find, it falls back to a blank mark for an unknown program.
//
// A packaged installation ships that entry. A single downloaded file cannot, so
// it writes one for itself, into the user's own directories and nowhere else.
//
// executable is what the entry should run, normally the file this program was
// started from. Answers with the paths written, which is nothing at all when
// the desktop already has a current entry or when the platform has no such
// concept.
QStringList installDesktopEntry(const QString &executable);
