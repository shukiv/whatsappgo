#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QLocalServer>
#include <QSettings>
#include <QTemporaryDir>

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QCoreApplication::setOrganizationName(QStringLiteral("WhatsAppGoTest"));
    QCoreApplication::setApplicationName(QStringLiteral("ProfileDisplayNames"));
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");

    QTemporaryDir temporary(shortTempTemplate(QStringLiteral("wag-names")));
    if (!temporary.isValid())
        return testFatal("could not create a temporary directory");
    qputenv("XDG_RUNTIME_DIR", temporary.path().toUtf8());
    qputenv("XDG_DATA_HOME", QDir(temporary.path()).filePath(QStringLiteral("data")).toUtf8());
    qputenv("XDG_CACHE_HOME", QDir(temporary.path()).filePath(QStringLiteral("cache")).toUtf8());
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, temporary.path());

    // This checks local settings, not backend lifecycle. Keep a listener up
    // so constructing two clients cannot spawn/stop real helpers and exhaust
    // the Windows timeout. A distinct profile also isolates its named pipe.
    const auto profile = QStringLiteral("profile-display-names-test");
    const auto socketPath = RpcClient::socketPathForProfile(profile);
#ifndef Q_OS_WIN
    QDir().mkpath(QFileInfo(socketPath).absolutePath());
#endif
    QLocalServer backend;
    if (!backend.listen(socketPath))
        return testFatal("could not listen on profile-name test socket", backend.errorString());

    {
        QSettings settings;
        settings.setValue(QStringLiteral("accounts/profiles"),
                          QStringList{QStringLiteral("default"), QStringLiteral("israeli")});
    }

    {
        RpcClient client(profile);
        // Deliver the first connection before constructing the next client;
        // Windows creates the next listening pipe instance during this turn.
        QCoreApplication::processEvents();
        client.renameProfile(QStringLiteral("israeli"), QStringLiteral("  Israeli support  "));
        if (client.profileDisplayNames().value(QStringLiteral("israeli")).toString()
            != QStringLiteral("Israeli support")) {
            return testFatal("a renamed account did not keep its trimmed display name");
        }
        client.renameProfile(QStringLiteral("missing"), QStringLiteral("Must not be stored"));
        if (client.profileDisplayNames().contains(QStringLiteral("missing")))
            return testFatal("renaming an account that does not exist stored a name");
    }

    RpcClient reloaded(profile);
    return reloaded.profileDisplayNames().value(QStringLiteral("israeli")).toString()
            == QStringLiteral("Israeli support")
        ? EXIT_SUCCESS
        : EXIT_FAILURE;
}
