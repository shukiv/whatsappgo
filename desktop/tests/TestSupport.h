#pragma once

#include <QDir>
#include <QFileInfo>
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

// macOS caps a Unix socket path at 104 characters and hands QDir::tempPath a
// 49-character one - /var/folders/d8/hvxvltxn0fl4rmnd52sncbth0000gn/T - so a
// socket under a QTemporaryDir there overran the limit and every test that
// served one failed with "QLocalServer::listen: Name error". /tmp keeps the
// same paths around fifty characters.
inline QString shortTempTemplate(const QString &prefix)
{
#ifdef Q_OS_UNIX
    const QFileInfo shortBase(QStringLiteral("/tmp"));
    if (shortBase.isDir() && shortBase.isWritable())
        return shortBase.filePath() + QLatin1Char('/') + prefix + QStringLiteral("-XXXXXX");
#endif
    return QDir::tempPath() + QLatin1Char('/') + prefix + QStringLiteral("-XXXXXX");
}
