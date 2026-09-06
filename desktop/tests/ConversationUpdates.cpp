#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QPointer>
#include <QTemporaryDir>
#include <QTimer>

#include <functional>

namespace {

bool waitFor(const std::function<bool()> &done, int timeoutMs = 3000)
{
    QElapsedTimer clock;
    clock.start();
    while (!done() && clock.elapsed() < timeoutMs)
        QCoreApplication::processEvents(QEventLoop::WaitForMoreEvents, 20);
    return done();
}

void settle(int ms = 120)
{
    QElapsedTimer clock;
    clock.start();
    while (clock.elapsed() < ms)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 20);
}

QJsonObject message(const QString &id, const QString &chat, qint64 timestamp, bool fromMe = false)
{
    return QJsonObject{
        {QStringLiteral("id"), id},
        {QStringLiteral("chat_jid"), chat},
        {QStringLiteral("sender_jid"), fromMe ? QStringLiteral("me@lid") : chat},
        {QStringLiteral("timestamp"), timestamp},
        {QStringLiteral("kind"), QStringLiteral("text")},
        {QStringLiteral("body"), QStringLiteral("body ") + id},
        {QStringLiteral("from_me"), fromMe},
        {QStringLiteral("status"), QStringLiteral("received")},
    };
}

}

// What a conversation does with answers that arrive late, with a daemon that
// goes away mid-request, and with the small changes - a reaction, a star, a new
// message - that must not cost the reader their place.
int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir runtime(shortTempTemplate(QStringLiteral("wag-convo")));
    if (!runtime.isValid())
        return testFatal("could not create a temporary runtime directory");
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return testFatal("could not create the socket directory", socketDir);

    QLocalServer server;
    const auto socketPath = RpcClient::socketPathForProfile(QStringLiteral("convo"));
    if (!server.listen(socketPath))
        return testFatal("could not listen on the socket", socketPath + QStringLiteral(": ") + server.errorString());

    const QString alice = QStringLiteral("alice@lid");
    const QString bob = QStringLiteral("bob@lid");

    QPointer<QLocalSocket> connection;
    QByteArray input;
    // Requests the test answers by hand, so a transfer can still be running
    // while the reader moves on.
    QHash<QString, QJsonObject> held;
    int downloadRequests = 0;
    int readRequests = 0;
    QJsonArray readIds;
    int messageGetRequests = 0;
    int listRequests = 0;
    int sendRequests = 0;
    QString heldSendId;

    const auto reply = [&](const QJsonValue &id, const QJsonValue &result) {
        if (!connection)
            return;
        connection->write(QJsonDocument(QJsonObject{
            {QStringLiteral("version"), 1},
            {QStringLiteral("id"), id},
            {QStringLiteral("result"), result},
        }).toJson(QJsonDocument::Compact) + '\n');
    };

    QObject::connect(&server, &QLocalServer::newConnection, &app, [&] {
        auto *socket = server.nextPendingConnection();
        connection = socket;
        QObject::connect(socket, &QLocalSocket::readyRead, socket, [&, socket] {
            input += socket->readAll();
            while (true) {
                const auto newline = input.indexOf('\n');
                if (newline < 0)
                    break;
                const auto request = QJsonDocument::fromJson(input.left(newline)).object();
                input.remove(0, newline + 1);
                const auto method = request.value(QStringLiteral("method")).toString();
                const auto params = request.value(QStringLiteral("params")).toObject();
                const auto id = request.value(QStringLiteral("id"));

                if (method == QStringLiteral("message.download")) {
                    ++downloadRequests;
                    // Held: the answer is delivered by the test, after the
                    // reader has opened another conversation.
                    held.insert(id.toString(), params);
                    continue;
                }
                if (method == QStringLiteral("message.send")) {
                    ++sendRequests;
                    heldSendId = id.toString();
                    continue;
                }
                QJsonValue result = QJsonObject{};
                if (method == QStringLiteral("status.get")) {
                    result = QJsonObject{{QStringLiteral("state"), QStringLiteral("connected")},
                                         {QStringLiteral("connected"), true},
                                         {QStringLiteral("logged_in"), true}};
                } else if (method == QStringLiteral("messages.list")) {
                    ++listRequests;
                    QJsonArray page;
                    const auto chat = params.value(QStringLiteral("chat_jid")).toString();
                    for (int index = 0; index < 100; ++index)
                        page.append(message(QStringLiteral("m%1").arg(index, 3, 10, QChar('0')), chat, 1000 + index));
                    result = QJsonObject{{QStringLiteral("messages"), page},
                                         {QStringLiteral("has_more"), true},
                                         {QStringLiteral("next_before"), 1000},
                                         {QStringLiteral("next_before_id"), QStringLiteral("m000")}};
                } else if (method == QStringLiteral("message.get")) {
                    ++messageGetRequests;
                    auto updated = message(params.value(QStringLiteral("message_id")).toString(),
                                           params.value(QStringLiteral("chat_jid")).toString(), 1005);
                    updated.insert(QStringLiteral("reactions"), QJsonArray{QJsonObject{
                        {QStringLiteral("emoji"), QStringLiteral("👍")},
                        {QStringLiteral("sender_jid"), QStringLiteral("friend@lid")}}});
                    result = updated;
                } else if (method == QStringLiteral("chat.read")) {
                    ++readRequests;
                    readIds = params.value(QStringLiteral("message_ids")).toArray();
                }
                reply(id, result);
            }
        });
    });

    RpcClient client(QStringLiteral("convo"));
    if (!waitFor([&] { return client.daemonConnected(); }))
        return testFatal("the client never connected to the stub daemon");

    // 1. A download that finishes after the reader has moved on belongs to the
    // conversation it was started in, not the one now on screen.
    client.openChat(alice, QStringLiteral("Alice"));
    if (!waitFor([&] { return client.messageList()->count() == 100; }))
        return testFatal("the first conversation never loaded");
    client.downloadMedia(QStringLiteral("m050"));
    if (!waitFor([&] { return downloadRequests == 1 && !held.isEmpty(); }))
        return testFatal("no download was requested");
    const auto heldId = held.keys().constFirst();
    client.openChat(bob, QStringLiteral("Bob"));
    if (!waitFor([&] { return client.messageList()->count() == 100 && listRequests == 2; }))
        return testFatal("the second conversation never loaded");
    auto downloaded = message(QStringLiteral("m050"), alice, 1050);
    downloaded.insert(QStringLiteral("kind"), QStringLiteral("image"));
    downloaded.insert(QStringLiteral("media_path"), QStringLiteral("/tmp/whatsappgo-test.jpg"));
    reply(heldId, downloaded);
    settle();
    for (int row = 0; row < client.messageList()->count(); ++row) {
        if (client.messageList()->at(row).value(QStringLiteral("chat_jid")).toString() != bob)
            return testFatal("a late download was inserted into the wrong conversation",
                             client.messageList()->at(row).value(QStringLiteral("chat_jid")).toString());
    }
    held.clear();

    // 2. A reaction updates one message and leaves every loaded page alone.
    const int before = client.messageList()->count();
    connection->write(QJsonDocument(QJsonObject{
        {QStringLiteral("version"), 1},
        {QStringLiteral("event"), QStringLiteral("message.reaction")},
        {QStringLiteral("data"), QJsonObject{{QStringLiteral("chat_jid"), bob},
                                             {QStringLiteral("message_id"), QStringLiteral("m010")}}},
    }).toJson(QJsonDocument::Compact) + '\n');
    if (!waitFor([&] { return messageGetRequests == 1; }))
        return testFatal("a reaction did not ask for the message it changed");
    settle();
    if (client.messageList()->count() != before)
        return testFatal("a reaction threw away loaded history",
                         QStringLiteral("%1 -> %2").arg(before).arg(client.messageList()->count()));
    if (client.messageList()->byId(QStringLiteral("m010")).value(QStringLiteral("reactions")).toList().isEmpty())
        return testFatal("the reaction never reached the message");

    // 3. A star lands on the message it was asked for.
    connection->write(QJsonDocument(QJsonObject{
        {QStringLiteral("version"), 1},
        {QStringLiteral("event"), QStringLiteral("message.starred")},
        {QStringLiteral("data"), QJsonObject{{QStringLiteral("chat_jid"), bob},
                                             {QStringLiteral("message_id"), QStringLiteral("m000")},
                                             {QStringLiteral("starred"), true}}},
    }).toJson(QJsonDocument::Compact) + '\n');
    settle();
    if (!client.messageList()->byId(QStringLiteral("m000")).value(QStringLiteral("starred")).toBool())
        return testFatal("the star did not reach the message it was meant for");
    if (client.messageList()->byId(QStringLiteral("m099")).value(QStringLiteral("starred")).toBool())
        return testFatal("the star landed on another message as well");

    // 4. A message that arrives while its conversation is open is acknowledged.
    readRequests = 0;
    readIds = {};
    connection->write(QJsonDocument(QJsonObject{
        {QStringLiteral("version"), 1},
        {QStringLiteral("event"), QStringLiteral("message.upsert")},
        {QStringLiteral("data"), message(QStringLiteral("live-1"), bob, 2000)},
    }).toJson(QJsonDocument::Compact) + '\n');
    if (!waitFor([&] { return readRequests > 0; }))
        return testFatal("a message the reader can see was never marked read");
    if (readIds.isEmpty() || readIds.constBegin()->toString() != QStringLiteral("live-1"))
        return testFatal("the wrong message was acknowledged");

    // A message of the reader's own needs no receipt.
    readRequests = 0;
    connection->write(QJsonDocument(QJsonObject{
        {QStringLiteral("version"), 1},
        {QStringLiteral("event"), QStringLiteral("message.upsert")},
        {QStringLiteral("data"), message(QStringLiteral("mine-1"), bob, 2001, true)},
    }).toJson(QJsonDocument::Compact) + '\n');
    settle();
    if (readRequests != 0)
        return testFatal("the reader's own message was acknowledged back to them");

    // 5. A send whose daemon disappears mid-request leaves the composer usable.
    client.sendMessage(QStringLiteral("hello"), QString());
    if (!waitFor([&] { return sendRequests == 1 && client.busy(); }))
        return testFatal("the send never reached the daemon");
    server.close();
    if (connection)
        connection->disconnectFromServer();
    if (!waitFor([&] { return !client.daemonConnected(); }))
        return testFatal("the client did not notice the daemon leaving");
    settle();
    if (client.busy())
        return testFatal("sending stayed disabled after the daemon disconnected");

    // A request made with no daemon answers its caller too.
    client.sendMessage(QStringLiteral("hello again"), QString());
    settle();
    if (client.busy())
        return testFatal("a send refused before it left the window left the composer disabled");
    return EXIT_SUCCESS;
}
