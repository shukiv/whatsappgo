#include "desktopentry.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QStandardPaths>

namespace {

const char *kEntryName = "org.whatsappgo.Desktop.desktop";
const char *kIconName = "org.whatsappgo.Desktop.svg";

// A path a shell command can be handed back. The specification quotes with
// double quotes and escapes the quote characters inside, which is what a
// directory named after somebody with an apostrophe needs.
QString quotedForExec(const QString &path)
{
    QString escaped = path;
    escaped.replace(QLatin1Char('\\'), QStringLiteral("\\\\"));
    escaped.replace(QLatin1Char('"'), QStringLiteral("\\\""));
    return QLatin1Char('"') + escaped + QLatin1Char('"');
}

QString entryContents(const QString &executable)
{
    // StartupWMClass is how a shell ties a window it can see to the entry it
    // has: Qt names the window's class after the application, while the name
    // beside it is whatever file the program was started from - which for a
    // downloaded file is the download's own name, and matches nothing.
    return QStringLiteral(
               "[Desktop Entry]\n"
               "Type=Application\n"
               "Name=WhatsAppGo\n"
               "Comment=Low-memory native WhatsApp client\n"
               "Exec=%1\n"
               "Icon=org.whatsappgo.Desktop\n"
               "Terminal=false\n"
               "Categories=Network;InstantMessaging;Chat;\n"
               "StartupNotify=true\n"
               "StartupWMClass=WhatsAppGo\n"
               "X-GNOME-UsesNotifications=true\n"
               "Keywords=WhatsApp;Chat;Messages;\n")
        .arg(quotedForExec(executable));
}

// A file is only written when it is not already exactly what should be there.
// Startup runs this every time, and rewriting an unchanged entry would make
// the desktop reread its menus for nothing.
bool writeIfDifferent(const QString &path, const QByteArray &wanted)
{
    if (QFile::exists(path)) {
        QFile existing(path);
        if (existing.open(QIODevice::ReadOnly) && existing.readAll() == wanted)
            return false;
    }
    QDir().mkpath(QFileInfo(path).absolutePath());
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return false;
    return file.write(wanted) == wanted.size();
}

// An entry installed by a package manager belongs to that package. Writing a
// second one into the user's own directory would shadow it, and would go on
// naming a file that an upgrade has since replaced.
bool systemEntryExists()
{
    const auto roots = QStandardPaths::standardLocations(QStandardPaths::GenericDataLocation);
    const auto mine = QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation);
    for (const auto &root : roots) {
        if (root == mine)
            continue;
        if (QFileInfo::exists(QDir(root).filePath(QLatin1String("applications/") + QLatin1String(kEntryName))))
            return true;
    }
    return false;
}

} // namespace

QStringList installDesktopEntry(const QString &executable)
{
#ifndef Q_OS_LINUX
    Q_UNUSED(executable)
    return {};
#else
    if (executable.isEmpty() || systemEntryExists())
        return {};

    const auto dataRoot = QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation);
    if (dataRoot.isEmpty())
        return {};

    QStringList written;
    // The icon first: an entry naming an icon that is not there yet is an
    // entry the shell draws blank, and it may not look again for a while.
    const auto iconPath = QDir(dataRoot).filePath(QLatin1String("icons/hicolor/scalable/apps/")
                                                  + QLatin1String(kIconName));
    QFile carried(QStringLiteral(":/org.whatsappgo.Desktop.svg"));
    if (carried.open(QIODevice::ReadOnly) && writeIfDifferent(iconPath, carried.readAll()))
        written.append(iconPath);

    const auto entryPath = QDir(dataRoot).filePath(QLatin1String("applications/") + QLatin1String(kEntryName));
    if (writeIfDifferent(entryPath, entryContents(executable).toUtf8()))
        written.append(entryPath);
    return written;
#endif
}
