#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QEventLoop>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QTemporaryDir>
#include <QTimer>

namespace {
QByteArray eventLine(const QString &name, const QJsonObject &data)
{
    return QJsonDocument(QJsonObject{{QStringLiteral("version"), 1},
                                     {QStringLiteral("event"), name},
                                     {QStringLiteral("data"), data}})
        .toJson(QJsonDocument::Compact) + '\n';
}
}

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir runtime(shortTempTemplate(QStringLiteral("wag-presence")));
    if (!runtime.isValid())
        return testFatal("could not create a temporary runtime directory");
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return testFatal("could not create the socket directory", socketDir);

    QLocalServer server;
    const auto socketPath = RpcClient::socketPathForProfile(QStringLiteral("presence"));
    if (!server.listen(socketPath))
        return testFatal("could not listen on the socket", socketPath + QStringLiteral(": ") + server.errorString());

    bool subscribed = false;
    bool onlineSeen = false;
    bool typingSeen = false;
    bool offlineSeen = false;
    QLocalSocket *eventSocket = nullptr;
    QByteArray input;
    QObject::connect(&server, &QLocalServer::newConnection, &app, [&] {
        auto *socket = server.nextPendingConnection();
        eventSocket = socket;
        QObject::connect(socket, &QLocalSocket::readyRead, socket, [&, socket] {
            input += socket->readAll();
            while (true) {
                const auto newline = input.indexOf('\n');
                if (newline < 0)
                    break;
                const auto request = QJsonDocument::fromJson(input.left(newline)).object();
                input.remove(0, newline + 1);
                const auto method = request.value(QStringLiteral("method")).toString();
                QJsonValue result = QJsonObject{};
                if (method == QStringLiteral("status.get")) {
                    result = QJsonObject{{QStringLiteral("state"), QStringLiteral("connected")},
                                         {QStringLiteral("connected"), true},
                                         {QStringLiteral("logged_in"), true}};
                } else if (method == QStringLiteral("chats.list")) {
                    result = QJsonArray{};
                } else if (method == QStringLiteral("messages.list")) {
                    result = QJsonObject{{QStringLiteral("messages"), QJsonArray{}},
                                         {QStringLiteral("has_more"), false}};
                } else if (method == QStringLiteral("contact.presence.subscribe")) {
                    subscribed = request.value(QStringLiteral("params")).toObject()
                                     .value(QStringLiteral("chat_jid")).toString() == QStringLiteral("alice@lid");
                }
                socket->write(QJsonDocument(QJsonObject{{QStringLiteral("version"), 1},
                                                        {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                                                        {QStringLiteral("result"), result}})
                                  .toJson(QJsonDocument::Compact) + '\n');
                if (method == QStringLiteral("contact.presence.subscribe")) {
                    socket->write(eventLine(QStringLiteral("contact.presence"),
                                            {{QStringLiteral("jid"), QStringLiteral("alice@lid")},
                                             {QStringLiteral("unavailable"), false},
                                             {QStringLiteral("last_seen"), 0}}));
                    socket->write(eventLine(QStringLiteral("chat.presence"),
                                            {{QStringLiteral("chat_jid"), QStringLiteral("alice@lid")},
                                             {QStringLiteral("sender_jid"), QStringLiteral("alice@lid")},
                                             {QStringLiteral("state"), QStringLiteral("composing")},
                                             {QStringLiteral("media"), QStringLiteral("audio")}}));
                    socket->write(eventLine(QStringLiteral("chat.presence"),
                                            {{QStringLiteral("chat_jid"), QStringLiteral("alice@lid")},
                                             {QStringLiteral("state"), QStringLiteral("paused")}}));
                    socket->write(eventLine(QStringLiteral("contact.presence"),
                                            {{QStringLiteral("jid"), QStringLiteral("alice@lid")},
                                             {QStringLiteral("unavailable"), true},
                                             {QStringLiteral("last_seen"), 1700000000000.0}}));
                }
            }
        });
    });

    RpcClient client(QStringLiteral("presence"), QStringLiteral("alice@lid"));
    const auto observation = QObject::connect(&client, &RpcClient::selectedPresenceChanged, &app, [&] {
        const auto presence = client.selectedPresence();
        onlineSeen = onlineSeen || presence.value(QStringLiteral("state")).toString() == QStringLiteral("online");
        typingSeen = typingSeen || (presence.value(QStringLiteral("chat_state")).toString() == QStringLiteral("composing")
                                    && presence.value(QStringLiteral("media")).toString() == QStringLiteral("audio"));
        offlineSeen = offlineSeen || (presence.value(QStringLiteral("state")).toString() == QStringLiteral("offline")
                                      && presence.value(QStringLiteral("last_seen")).toLongLong() == 1700000000000LL
                                      && presence.value(QStringLiteral("chat_state")).toString().isEmpty());
        if (subscribed && onlineSeen && typingSeen && offlineSeen)
            app.quit();
    });
    QTimer watchdog;
    watchdog.setSingleShot(true);
    QObject::connect(&watchdog, &QTimer::timeout, &app, &QCoreApplication::quit);
    watchdog.start(2000);
    app.exec();
    watchdog.stop();
    QObject::disconnect(observation);
    if (!(subscribed && onlineSeen && typingSeen && offlineSeen) || !eventSocket)
        return testFatal("initial presence sequence did not arrive");
    auto *expiry = client.findChild<QTimer *>(QStringLiteral("chatPresenceExpiryTimer"));
    if (!expiry || expiry->interval() != 10000 || !expiry->isSingleShot())
        return testFatal("transient activity must have a ten-second single-shot expiry");
    // Exercise the real socket/event/timer/notification path at a shorter
    // interval so lost-packet regressions do not require a 12-second sleep.
    expiry->setInterval(150);
    const auto settle = [](int milliseconds) {
        QEventLoop loop;
        QTimer::singleShot(milliseconds, &loop, &QEventLoop::quit);
        loop.exec();
    };
    const auto composing = [&](const QString &media = QString()) {
        eventSocket->write(eventLine(QStringLiteral("chat.presence"),
                                    {{QStringLiteral("chat_jid"), QStringLiteral("alice@lid")},
                                     {QStringLiteral("sender_jid"), QStringLiteral("alice@lid")},
                                     {QStringLiteral("state"), QStringLiteral("composing")},
                                     {QStringLiteral("media"), media}}));
        settle(30);
    };
    composing();
    eventSocket->write(eventLine(QStringLiteral("contact.presence"),
                                {{QStringLiteral("jid"), QStringLiteral("alice@lid")},
                                 {QStringLiteral("unavailable"), true},
                                 {QStringLiteral("last_seen"), 1700000000000.0}}));
    settle(30);
    bool passed = true;
    if (client.selectedPresence().value(QStringLiteral("chat_state")).toString() == QStringLiteral("composing")
            || expiry->isActive()) {
        testFatal("Typing remains visible after the contact goes offline without a paused event");
        passed = false;
    }
    eventSocket->write(eventLine(QStringLiteral("contact.presence"),
                                {{QStringLiteral("jid"), QStringLiteral("alice@lid")},
                                 {QStringLiteral("unavailable"), false},
                                 {QStringLiteral("last_seen"), 1700000000000.0}}));
    composing(QStringLiteral("audio"));
    int expiryNotifications = 0;
    const auto expiryObservation = QObject::connect(&client, &RpcClient::selectedPresenceChanged, &app, [&] {
        if (!client.selectedPresence().contains(QStringLiteral("chat_state")))
            ++expiryNotifications;
    });
    // A lost paused packet must not leave transient activity on screen forever.
    settle(250);
    QObject::disconnect(expiryObservation);
    const auto expired = client.selectedPresence();
    if (expired.contains(QStringLiteral("chat_state")) || expired.contains(QStringLiteral("media"))
            || expired.contains(QStringLiteral("sender_jid")) || expiryNotifications != 1
            || expired.value(QStringLiteral("state")).toString() != QStringLiteral("online")
            || expired.value(QStringLiteral("last_seen")).toLongLong() != 1700000000000LL) {
        testFatal("activity expiry did not notify the UI and restore underlying presence");
        passed = false;
    }

    // Continued typing renews the deadline. Online heartbeats and another
    // chat's activity must not keep the selected contact's typing alive.
    expiry->setInterval(300);
    composing();
    settle(160);
    const auto beforeRefresh = expiry->remainingTime();
    composing();
    if (!expiry->isActive() || expiry->remainingTime() <= beforeRefresh)
        return testFatal("fresh activity did not renew its expiry");
    const auto beforeOnline = expiry->remainingTime();
    eventSocket->write(eventLine(QStringLiteral("contact.presence"),
                                {{QStringLiteral("jid"), QStringLiteral("alice@lid")},
                                 {QStringLiteral("unavailable"), false}}));
    eventSocket->write(eventLine(QStringLiteral("chat.presence"),
                                {{QStringLiteral("chat_jid"), QStringLiteral("bob@lid")},
                                 {QStringLiteral("state"), QStringLiteral("composing")}}));
    settle(30);
    if (expiry->remainingTime() > beforeOnline
            || client.selectedPresence().value(QStringLiteral("sender_jid")).toString() != QStringLiteral("alice@lid"))
        return testFatal("unrelated presence renewed activity or discarded its sender");
    settle(350);
    if (client.selectedPresence().contains(QStringLiteral("chat_state")))
        return testFatal("typing did not expire after the final activity update");

    composing();
    eventSocket->write(eventLine(QStringLiteral("chat.presence"),
                                {{QStringLiteral("chat_jid"), QStringLiteral("alice@lid")},
                                 {QStringLiteral("state"), QStringLiteral("paused")}}));
    settle(30);
    if (expiry->isActive() || client.selectedPresence().contains(QStringLiteral("chat_state")))
        return testFatal("paused activity did not clear immediately");
    composing();
    client.openChat(QStringLiteral("bob@lid"), QStringLiteral("Bob"));
    if (expiry->isActive() || !client.selectedPresence().isEmpty())
        return testFatal("typing survived changing the selected chat");
    client.openChat(QStringLiteral("alice@lid"), QStringLiteral("Alice"));
    settle(30); // Consume the fixture's subscription events first.
    composing();
    client.closeChat();
    if (expiry->isActive() || !client.selectedPresence().isEmpty())
        return testFatal("typing survived closing the selected chat");
    client.openChat(QStringLiteral("alice@lid"), QStringLiteral("Alice"));
    settle(30);
    composing();
    eventSocket->write(eventLine(QStringLiteral("connection.changed"),
                                {{QStringLiteral("state"), QStringLiteral("disconnected")},
                                 {QStringLiteral("connected"), false}}));
    settle(30);
    if (expiry->isActive() || !client.selectedPresence().isEmpty())
        return testFatal("typing survived losing the WhatsApp connection");
    composing();
    eventSocket->disconnectFromServer();
    settle(30);
    if (expiry->isActive() || !client.selectedPresence().isEmpty())
        return testFatal("typing survived losing the daemon connection");
    return passed ? EXIT_SUCCESS : EXIT_FAILURE;
}
