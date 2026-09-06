#include "rpcclient.h"
#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QJsonDocument>
#include <QLocalServer>
#include <QPointer>
#include <QSettings>
#include <QTemporaryDir>
#include <functional>

int testPrivacySettings()
{
    QTemporaryDir runtime(shortTempTemplate(QStringLiteral("wag-privacy")));
    if (!runtime.isValid())
        return testFatal("could not create privacy test runtime");
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    // XDG_CONFIG_HOME does not redirect Windows' native registry settings,
    // which also reject an empty organization. Seed both accounts in a
    // temporary INI store on every platform, as the profile-name test does.
    QCoreApplication::setOrganizationName(QStringLiteral("WhatsAppGoTest"));
    QCoreApplication::setApplicationName(QStringLiteral("PrivacySettings"));
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, runtime.path());
    QSettings::setPath(QSettings::IniFormat, QSettings::SystemScope, runtime.path());
    {
        QSettings settings;
        settings.setValue(QStringLiteral("accounts/profiles"), QStringList{QStringLiteral("default"), QStringLiteral("work")});
        settings.sync();
        if (settings.status() != QSettings::NoError)
            return testFatal("could not seed privacy test profiles", settings.fileName());
    }
    QDir().mkpath(QDir(runtime.path()).filePath(QStringLiteral("whatsappgo")));
    QLocalServer first, second;
    if (!first.listen(RpcClient::socketPathForProfile(QStringLiteral("default")))
            || !second.listen(RpcClient::socketPathForProfile(QStringLiteral("work"))))
        return testFatal("could not listen on privacy test sockets");
    struct Request { QPointer<QLocalSocket> socket; QJsonValue id; };
    QList<Request> pending;
    QPointer<QLocalSocket> active;
    const auto answer = [](const Request &request, const QJsonObject &result) {
        if (request.socket)
            request.socket->write(QJsonDocument(QJsonObject{
                {QStringLiteral("version"), 1}, {QStringLiteral("id"), request.id},
                {QStringLiteral("result"), result}}).toJson(QJsonDocument::Compact) + '\n');
    };
    const auto attach = [&](QLocalServer *server) {
        QObject::connect(server, &QLocalServer::newConnection, server, [&, server] {
            auto *socket = server->nextPendingConnection();
            active = socket;
            QObject::connect(socket, &QLocalSocket::readyRead, socket, [&, socket] {
                while (socket->canReadLine()) {
                    const auto object = QJsonDocument::fromJson(socket->readLine()).object();
                    const auto method = object.value(QStringLiteral("method")).toString();
                    Request request{socket, object.value(QStringLiteral("id"))};
                    if (method == QStringLiteral("privacy.get")) {
                        pending.append(request);
                        continue;
                    }
                    answer(request, method == QStringLiteral("status.get")
                        ? QJsonObject{{QStringLiteral("connected"), true}, {QStringLiteral("logged_in"), true}}
                        : QJsonObject{});
                }
            });
        });
    };
    attach(&first);
    attach(&second);
    const auto waitFor = [](const std::function<bool()> &done) {
        QElapsedTimer clock;
        clock.start();
        while (!done() && clock.elapsed() < 4000)
            QCoreApplication::processEvents(QEventLoop::AllEvents, 20);
        return done();
    };
    const auto everyone = QJsonObject{{QStringLiteral("last_seen"), QStringLiteral("all")},
                                      {QStringLiteral("online"), QStringLiteral("all")}};
    const auto nobody = QJsonObject{{QStringLiteral("last_seen"), QStringLiteral("none")},
                                    {QStringLiteral("online"), QStringLiteral("match_last_seen")}};
    RpcClient client(QStringLiteral("default"));
    if (!client.profiles().contains(QStringLiteral("work")))
        return testFatal("the privacy test's second account was not loaded");
    if (!waitFor([&] { return !pending.isEmpty(); }))
        return testFatal("privacy was not fetched after connecting");
    answer(pending.takeFirst(), everyone);
    if (!waitFor([&] { return client.privacySettings() == everyone.toVariantMap(); }))
        return testFatal("the first account's privacy settings never loaded");
    client.switchProfile(QStringLiteral("work"));
    if (client.profile() != QStringLiteral("work"))
        return testFatal("the privacy test did not switch to its second account");
    if (!client.privacySettings().isEmpty())
        return testFatal("the second account inherited the first account's privacy settings");
    if (!waitFor([&] { return !pending.isEmpty(); }))
        return testFatal("privacy was not fetched after switching accounts");
    answer(pending.takeFirst(), nobody);
    if (!waitFor([&] { return client.privacySettings() == nobody.toVariantMap(); }))
        return testFatal("the second account's privacy settings never loaded");
    // A newer event must win over a read already in flight.
    client.refreshPrivacySettings();
    if (!waitFor([&] { return !pending.isEmpty(); }))
        return testFatal("privacy refresh never reached the stub");
    active->write(QJsonDocument(QJsonObject{
        {QStringLiteral("version"), 1}, {QStringLiteral("event"), QStringLiteral("privacy.changed")},
        {QStringLiteral("data"), QJsonObject{{QStringLiteral("settings"), everyone}}}
    }).toJson(QJsonDocument::Compact) + '\n');
    if (!waitFor([&] { return client.privacySettings() == everyone.toVariantMap(); }))
        return testFatal("a privacy change event did not update settings");
    answer(pending.takeFirst(), nobody);
    // A later status.get response proves the preceding stale read was handled.
    bool statusReturned = false;
    QObject::connect(&client, &RpcClient::statusChanged, &client, [&] { statusReturned = true; });
    client.refreshStatus();
    if (!waitFor([&] { return statusReturned; }) || client.privacySettings() != everyone.toVariantMap())
        return testFatal("a stale read overwrote a newer privacy event");
    pending.clear();
    client.reconnect();
    if (!waitFor([&] { return !pending.isEmpty(); }))
        return testFatal("privacy was not fetched on reconnect");
    answer(pending.takeFirst(), nobody);
    if (!waitFor([&] { return client.privacySettings() == nobody.toVariantMap(); }))
        return testFatal("privacy remained stale after reconnecting");
    return EXIT_SUCCESS;
}
