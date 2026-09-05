#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
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
    QByteArray input;
    QObject::connect(&server, &QLocalServer::newConnection, &app, [&] {
        auto *socket = server.nextPendingConnection();
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
    QObject::connect(&client, &RpcClient::selectedPresenceChanged, &app, [&] {
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
    QTimer::singleShot(2000, &app, &QCoreApplication::quit);
    app.exec();
    return subscribed && onlineSeen && typingSeen && offlineSeen ? EXIT_SUCCESS : EXIT_FAILURE;
}
