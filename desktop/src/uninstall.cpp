#include "uninstall.h"

#include <QCoreApplication>
#include <QDir>
#include <QDirIterator>
#include <QFile>
#include <QFileInfo>
#include <QLocalSocket>
#include <QSettings>
#include <QStandardPaths>
#include <QTextStream>

#include <algorithm>
#include <cstdio>

namespace {

// The same rules the rest of the app uses to decide where its files live; see
// dataBaseDir/cacheBaseDir in rpcclient.cpp and internal/config/paths.go. An
// uninstall that looked in different places would leave an account behind.
QString dataRoot()
{
    const auto configured = qEnvironmentVariable("XDG_DATA_HOME");
    if (!configured.isEmpty())
        return QDir(configured).filePath(QStringLiteral("whatsappgo"));
#if defined(Q_OS_WIN)
    return QDir(qEnvironmentVariable("APPDATA")).filePath(QStringLiteral("whatsappgo"));
#elif defined(Q_OS_MACOS)
    return QDir::home().filePath(QStringLiteral("Library/Application Support/whatsappgo"));
#else
    return QDir::home().filePath(QStringLiteral(".local/share/whatsappgo"));
#endif
}

QString cacheRoot()
{
    const auto configured = qEnvironmentVariable("XDG_CACHE_HOME");
    if (!configured.isEmpty())
        return QDir(configured).filePath(QStringLiteral("whatsappgo"));
#if defined(Q_OS_WIN)
    return QDir(qEnvironmentVariable("LOCALAPPDATA")).filePath(QStringLiteral("whatsappgo"));
#elif defined(Q_OS_MACOS)
    return QDir::home().filePath(QStringLiteral("Library/Caches/whatsappgo"));
#else
    return QDir::home().filePath(QStringLiteral(".cache/whatsappgo"));
#endif
}

QString runtimeRoot()
{
    auto runtime = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (runtime.isEmpty())
        runtime = QStandardPaths::writableLocation(QStandardPaths::RuntimeLocation);
    if (runtime.isEmpty())
        return {};
    return QDir(runtime).filePath(QStringLiteral("whatsappgo"));
}

qint64 sizeOf(const QString &path)
{
    const QFileInfo info(path);
    if (!info.exists())
        return 0;
    if (info.isFile())
        return info.size();
    qint64 total = 0;
    QDirIterator walk(path, QDir::Files | QDir::Hidden | QDir::System, QDirIterator::Subdirectories);
    while (walk.hasNext()) {
        walk.next();
        total += walk.fileInfo().size();
    }
    return total;
}

QString readable(qint64 bytes)
{
    if (bytes >= 1024LL * 1024 * 1024)
        return QStringLiteral("%1 GB").arg(bytes / double(1024LL * 1024 * 1024), 0, 'f', 1);
    if (bytes >= 1024 * 1024)
        return QStringLiteral("%1 MB").arg(bytes / double(1024 * 1024), 0, 'f', 1);
    if (bytes >= 1024)
        return QStringLiteral("%1 KB").arg(bytes / 1024);
    return QStringLiteral("%1 bytes").arg(bytes);
}

void add(UninstallPlan &plan, const QString &path, const QString &description, bool holdsAccounts = false)
{
    if (path.isEmpty() || !QFileInfo::exists(path))
        return;
    plan.targets.append({path, description, sizeOf(path), holdsAccounts});
}

// A window still open would put its settings back as it exits, and its daemons
// hold the databases being removed. The single-instance socket is how the app
// already recognises itself, so it answers this too.
bool anInstanceIsRunning(const QStringList &accounts)
{
    const auto runtime = runtimeRoot();
    if (runtime.isEmpty())
        return false;
    auto names = accounts;
    names.append(QStringLiteral("default"));
    names.removeDuplicates();
    for (const auto &account : std::as_const(names)) {
        QLocalSocket probe;
        probe.connectToServer(QDir(runtime).filePath(QStringLiteral("ui-%1.sock").arg(account)),
                              QIODevice::ReadOnly);
        if (probe.waitForConnected(300)) {
            probe.abort();
            return true;
        }
    }
    return false;
}

// An AppImage that has been integrated leaves a launcher behind, and only the
// entries that point at this very file are ours to remove.
QStringList desktopEntriesFor(const QString &appImage)
{
    QStringList found;
    if (appImage.isEmpty())
        return found;
    QDir directory(QDir::home().filePath(QStringLiteral(".local/share/applications")));
    if (!directory.exists())
        return found;
    const auto entries = directory.entryInfoList({QStringLiteral("*.desktop")}, QDir::Files);
    for (const auto &entry : entries) {
        QFile file(entry.absoluteFilePath());
        if (!file.open(QIODevice::ReadOnly | QIODevice::Text))
            continue;
        if (QString::fromUtf8(file.readAll()).contains(appImage))
            found.append(entry.absoluteFilePath());
    }
    return found;
}

bool removePath(const QString &path, QTextStream &out)
{
    const QFileInfo info(path);
    const bool gone = info.isDir() ? QDir(path).removeRecursively() : QFile::remove(path);
    if (!gone)
        out << QStringLiteral("  could not remove %1\n").arg(path);
    return gone;
}

} // namespace

UninstallPlan planUninstall()
{
    UninstallPlan plan;

    // Asked of QSettings rather than worked out: this is the same file the app
    // reads and writes, wherever the platform decides to put it.
    const auto settingsPath = QSettings().fileName();
    if (QFileInfo::exists(settingsPath)) {
        QSettings settings(settingsPath, QSettings::IniFormat);
        plan.accounts = settings.value(QStringLiteral("accounts/profiles")).toStringList();
        for (auto &account : plan.accounts)
            account = account.trimmed();
        plan.accounts.removeAll(QString());
    }
    if (plan.accounts.isEmpty())
        plan.accounts.append(QStringLiteral("default"));

    plan.appImage = qEnvironmentVariable("APPIMAGE");
    plan.somethingIsRunning = anInstanceIsRunning(plan.accounts);

    add(plan, dataRoot(), QStringLiteral("accounts, message history and attachments"), true);
    add(plan, cacheRoot(), QStringLiteral("cached pictures, video and avatars"));
    add(plan, settingsPath, QStringLiteral("settings and the account list"));
    // The settings directory belongs to this app alone, so it goes with them.
    const auto settingsDirectory = QFileInfo(settingsPath).absolutePath();
    if (settingsDirectory.endsWith(QStringLiteral("WhatsAppGo")))
        add(plan, settingsDirectory, QStringLiteral("the settings folder"));
    add(plan, runtimeRoot(), QStringLiteral("sockets left by the background service"));
    const auto entries = desktopEntriesFor(plan.appImage);
    for (const auto &entry : entries)
        add(plan, entry, QStringLiteral("the launcher entry for this AppImage"));

    return plan;
}

int runUninstall(const UninstallPlan &plan, bool assumeYes, bool keepAccounts)
{
    QTextStream out(stdout);
    QTextStream in(stdin);

    if (plan.somethingIsRunning) {
        out << QStringLiteral("WhatsAppGo is still open. Close it first: its background service holds the "
                              "files this would remove, and quitting writes the settings back.\n");
        out.flush();
        return 1;
    }

    QList<UninstallTarget> doomed;
    qint64 total = 0;
    for (const auto &target : plan.targets) {
        if (keepAccounts && target.holdsAccounts)
            continue;
        doomed.append(target);
        total += target.bytes;
    }

    if (doomed.isEmpty()) {
        out << QStringLiteral("Nothing to remove: no WhatsAppGo data, settings or launcher entry on this "
                              "computer.\n");
        out.flush();
        return 0;
    }

    out << QStringLiteral("This removes WhatsAppGo from this computer:\n\n");
    for (const auto &target : std::as_const(doomed))
        out << QStringLiteral("  %1\n      %2, %3\n").arg(target.path, target.description, readable(target.bytes));
    out << QStringLiteral("\n  %1 in total.\n").arg(readable(total));

    const bool losingAccounts = std::any_of(doomed.cbegin(), doomed.cend(),
                                            [](const UninstallTarget &target) { return target.holdsAccounts; });
    if (losingAccounts) {
        // Deleting the keys does not tell WhatsApp anything. The phone goes on
        // listing this computer until it is told otherwise, and saying so here
        // is the difference between an uninstall and a mystery.
        out << QStringLiteral("\nYour messages live only on this computer and this cannot be undone.\n"
                              "WhatsApp is not told: open WhatsApp on your phone, go to Settings, "
                              "Linked devices, and remove this computer.\n");
    }
    if (!plan.appImage.isEmpty())
        out << QStringLiteral("\nThe AppImage itself stays where it is. Delete %1 when you are done with it.\n")
                   .arg(plan.appImage);

    if (!assumeYes) {
        out << QStringLiteral("\nType yes to remove all of this: ");
        out.flush();
        const auto answer = in.readLine().trimmed().toLower();
        if (answer != QStringLiteral("yes")) {
            out << QStringLiteral("Nothing was removed.\n");
            out.flush();
            return 1;
        }
    }

    int failures = 0;
    for (const auto &target : std::as_const(doomed)) {
        if (!removePath(target.path, out))
            ++failures;
    }
    // Settings are not always a file: on Windows they are registry keys, which
    // no amount of deleting folders reaches. Asking QSettings to empty itself
    // works wherever they are kept.
    QSettings settings;
    if (!settings.allKeys().isEmpty()) {
        settings.clear();
        settings.sync();
    }
    if (failures > 0)
        out << QStringLiteral("\n%1 of %2 could not be removed; the rest is gone.\n")
                   .arg(failures)
                   .arg(doomed.size());
    else if (keepAccounts)
        // Saying "removed" here would be a lie the reader only finds out about
        // when the accounts are still there.
        out << QStringLiteral("\nSettings and cached files are gone. Your accounts and their message "
                              "history are still on this computer.\n");
    else
        out << QStringLiteral("\nWhatsAppGo has been removed.\n");
    out.flush();
    return failures == 0 ? 0 : 1;
}
