#include "rpcclient.h"

#include <QCoreApplication>
#include <QDir>
#include <QSettings>
#include <QTemporaryDir>

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QCoreApplication::setOrganizationName(QStringLiteral("WhatsAppGoTest"));
    QCoreApplication::setApplicationName(QStringLiteral("ProfileDisplayNames"));
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");

    QTemporaryDir temporary;
    if (!temporary.isValid())
        return EXIT_FAILURE;
    qputenv("XDG_RUNTIME_DIR", temporary.path().toUtf8());
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, temporary.path());

    {
        QSettings settings;
        settings.setValue(QStringLiteral("accounts/profiles"),
                          QStringList{QStringLiteral("default"), QStringLiteral("israeli")});
    }

    {
        RpcClient client(QStringLiteral("default"));
        client.renameProfile(QStringLiteral("israeli"), QStringLiteral("  Israeli support  "));
        if (client.profileDisplayNames().value(QStringLiteral("israeli")).toString()
            != QStringLiteral("Israeli support")) {
            return EXIT_FAILURE;
        }
        client.renameProfile(QStringLiteral("missing"), QStringLiteral("Must not be stored"));
        if (client.profileDisplayNames().contains(QStringLiteral("missing")))
            return EXIT_FAILURE;
    }

    RpcClient reloaded(QStringLiteral("default"));
    return reloaded.profileDisplayNames().value(QStringLiteral("israeli")).toString()
            == QStringLiteral("Israeli support")
        ? EXIT_SUCCESS
        : EXIT_FAILURE;
}
