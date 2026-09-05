#include "updateinstaller.h"

#include <QCoreApplication>
#include <QDesktopServices>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QProcess>
#include <QUrl>

#include <cstdio>

namespace updateinstaller {
namespace {

// artifactSuffix is what this platform's release artifact is called. The path
// arrives over the socket, so it is checked rather than run on trust: a file
// that is not the shape of a release artifact is not something to execute.
QString artifactSuffix()
{
#if defined(Q_OS_LINUX)
    return QStringLiteral(".AppImage");
#elif defined(Q_OS_WIN)
    return QStringLiteral(".exe");
#elif defined(Q_OS_MACOS)
    return QStringLiteral(".dmg");
#else
    return {};
#endif
}

bool looksLikeAnArtifact(const QFileInfo &file)
{
    const auto suffix = artifactSuffix();
    return !suffix.isEmpty()
        && file.isFile()
        && file.fileName().startsWith(QStringLiteral("WhatsAppGo-"))
        && file.fileName().endsWith(suffix);
}

#if defined(Q_OS_LINUX)
// runningAppImage is the file the reader launched. QCoreApplication reports the
// path inside the mounted image instead, which disappears the moment the
// application exits and is no use for replacing anything.
QString runningAppImage()
{
    const auto value = qgetenv("APPIMAGE");
    if (value.isEmpty())
        return {};
    QFileInfo file(QString::fromLocal8Bit(value));
    return file.exists() ? file.absoluteFilePath() : QString();
}
#endif

}

bool installable()
{
#if defined(Q_OS_LINUX)
    const auto target = runningAppImage();
    return !target.isEmpty() && QFileInfo(target).isWritable();
#elif defined(Q_OS_WIN) || defined(Q_OS_MACOS)
    return true;
#else
    return false;
#endif
}

Outcome install(const QString &downloadedPath)
{
    const QFileInfo downloaded(downloadedPath);
    if (!looksLikeAnArtifact(downloaded)) {
        return {false, false, false,
                QCoreApplication::translate("updateinstaller",
                    "The downloaded file is not a WhatsAppGo release and was not installed.")};
    }

#if defined(Q_OS_LINUX)
    const auto target = runningAppImage();
    if (target.isEmpty()) {
        return {false, false, false,
                QCoreApplication::translate("updateinstaller",
                    "This copy was not installed as an AppImage, so it cannot replace itself. "
                    "The download is ready on the release page.")};
    }
    const QFileInfo current(target);
    if (!current.isWritable()) {
        return {false, false, false,
                QCoreApplication::translate("updateinstaller",
                    "%1 belongs to another user and cannot be replaced from here.").arg(target)};
    }
    // Written beside the file it replaces so that the last step is a rename on
    // one filesystem: either the old version or the new one is there, never
    // half of either.
    const auto staged = target + QStringLiteral(".new");
    QFile::remove(staged);
    if (!QFile::copy(downloaded.absoluteFilePath(), staged)) {
        return {false, false, false,
                QCoreApplication::translate("updateinstaller",
                    "The new version could not be written next to %1.").arg(target)};
    }
    QFile::setPermissions(staged, current.permissions()
                          | QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner);
    // QFile::rename refuses to replace an existing file; this is the rename
    // that does, and it is atomic. Replacing a running executable is safe on
    // Linux: this process keeps the file it already opened.
    if (std::rename(QFile::encodeName(staged).constData(), QFile::encodeName(target).constData()) != 0) {
        QFile::remove(staged);
        return {false, false, false,
                QCoreApplication::translate("updateinstaller",
                    "The new version could not be moved into place at %1.").arg(target)};
    }
    QFile::remove(downloaded.absoluteFilePath());
    return {true, true, false, {}};

#elif defined(Q_OS_WIN)
    // The installer replaces files this process is holding open, so it runs
    // after the window has closed. /SILENT shows its progress without asking
    // the questions that were already answered at install time.
    if (!QProcess::startDetached(downloaded.absoluteFilePath(), {QStringLiteral("/SILENT")})) {
        return {false, false, false,
                QCoreApplication::translate("updateinstaller", "The installer could not be started.")};
    }
    return {true, false, true, {}};

#elif defined(Q_OS_MACOS)
    // An unsigned bundle cannot replace itself in Applications without asking
    // for rights this application has no business holding, so the disk image is
    // opened and the reader drags it across, the way they installed it.
    if (!QDesktopServices::openUrl(QUrl::fromLocalFile(downloaded.absoluteFilePath()))) {
        return {false, false, false,
                QCoreApplication::translate("updateinstaller", "The disk image could not be opened.")};
    }
    return {true, false, false,
            QCoreApplication::translate("updateinstaller",
                "The disk image is open. Drag WhatsAppGo into Applications, replacing the old copy, "
                "then open it again.")};
#else
    return {false, false, false,
            QCoreApplication::translate("updateinstaller",
                "This platform has no automatic install; the release page has the download.")};
#endif
}

}
