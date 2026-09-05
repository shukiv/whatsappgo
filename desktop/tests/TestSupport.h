#pragma once

#include <QString>
#include <QtGlobal>

#include <cstdlib>

// A test that returns EXIT_FAILURE without saying anything is worth very
// little on a machine you cannot open a terminal on. Every early exit here
// names what it was doing and, where there is one, the reason the platform
// gave: the macOS jobs failed six of these in hundredths of a second with no
// output at all.
inline int testFatal(const char *what, const QString &detail = QString())
{
    if (detail.isEmpty())
        qWarning("FAILED: %s", what);
    else
        qWarning("FAILED: %s: %s", what, qPrintable(detail));
    return EXIT_FAILURE;
}
