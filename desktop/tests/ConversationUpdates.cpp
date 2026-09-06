#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QColor>
#include <QElapsedTimer>
#include <QImage>
#include <QEventLoop>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QPointer>
#include <QTemporaryDir>
#include <QTimer>
#include <QUrl>

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
int testPrivacySettings();

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    if (app.arguments().contains(QStringLiteral("--privacy-settings")))
        return testPrivacySettings();
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
    int biggestLimit = 0;
    QList<int> chatLimits;
    int presenceRequests = 0;
    int sendRequests = 0;
    QString heldSendId;
    QList<QJsonObject> mediaSends;
    QList<QJsonObject> starredRequests;

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
                if (method == QStringLiteral("message.send_media"))
                    mediaSends.append(params);
                if (method == QStringLiteral("messages.starred")) {
                    starredRequests.append(request);
                    continue; // Deliver out of order to exercise scope races.
                }
                QJsonValue result = QJsonObject{};
                if (method == QStringLiteral("status.get")) {
                    result = QJsonObject{{QStringLiteral("state"), QStringLiteral("connected")},
                                         {QStringLiteral("connected"), true},
                                         {QStringLiteral("logged_in"), true}};
                } else if (method == QStringLiteral("chats.list")) {
                    const int offset = params.value(QStringLiteral("offset")).toInt();
                    // The daemon serves whatever is asked for up to 500, which
                    // is what lets a refresh return the pages already loaded.
                    int limit = params.value(QStringLiteral("limit")).toInt();
                    if (limit <= 0 || limit > 500)
                        limit = 100;
                    chatLimits.append(limit);
                    QJsonArray page;
                    // 250 conversations: the first page is full, the second is
                    // the remainder, and there is nothing after that.
                    for (int index = offset; index < qMin(offset + limit, 250); ++index) {
                        page.append(QJsonObject{{QStringLiteral("jid"), QStringLiteral("chat%1@lid").arg(index)},
                                                {QStringLiteral("title"), QStringLiteral("Chat %1").arg(index)},
                                                {QStringLiteral("last_message_at"), 5000 - index}});
                    }
                    result = page;
                } else if (method == QStringLiteral("messages.list")) {
                    ++listRequests;
                    const auto requested = params.value(QStringLiteral("limit")).toInt();
                    biggestLimit = qMax(biggestLimit, requested);
                    // The daemon never serves more than 200 in one page, and a
                    // request for more used to be answered with 50.
                    const int size = requested > 200 ? 50 : requested;
                    QJsonArray page;
                    const auto chat = params.value(QStringLiteral("chat_jid")).toString();
                    const auto before = params.value(QStringLiteral("before")).toVariant().toLongLong();
                    const qint64 newest = before > 0 ? before - 1 : 1099;
                    for (qint64 stamp = newest - size + 1; stamp <= newest; ++stamp) {
                        page.append(message(QStringLiteral("m%1").arg(stamp, 4, 10, QChar('0')), chat, stamp));
                    }
                    const qint64 oldest = newest - size + 1;
                    result = QJsonObject{{QStringLiteral("messages"), page},
                                         {QStringLiteral("has_more"), oldest > 100},
                                         {QStringLiteral("next_before"), oldest},
                                         {QStringLiteral("next_before_id"), QStringLiteral("m%1").arg(oldest, 4, 10, QChar('0'))}};
                } else if (method == QStringLiteral("message.get")) {
                    ++messageGetRequests;
                    auto updated = message(params.value(QStringLiteral("message_id")).toString(),
                                           params.value(QStringLiteral("chat_jid")).toString(), 1005);
                    updated.insert(QStringLiteral("reactions"), QJsonArray{QJsonObject{
                        {QStringLiteral("emoji"), QStringLiteral("👍")},
                        {QStringLiteral("sender_jid"), QStringLiteral("friend@lid")}}});
                    result = updated;
                } else if (method == QStringLiteral("contact.presence.subscribe")) {
                    ++presenceRequests;
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
    if (!waitFor([&] { return client.messageList()->count() == 50; }))
        return testFatal("the first conversation never loaded");
    client.downloadMedia(QStringLiteral("m1050"));
    if (!waitFor([&] { return downloadRequests == 1 && !held.isEmpty(); }))
        return testFatal("no download was requested");
    const auto heldId = held.keys().constFirst();
    client.openChat(bob, QStringLiteral("Bob"));
    if (!waitFor([&] { return client.messageList()->count() == 50 && listRequests == 2; }))
        return testFatal("the second conversation never loaded");
    auto downloaded = message(QStringLiteral("m1050"), alice, 1050);
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
                                             {QStringLiteral("message_id"), QStringLiteral("m1050")}}},
    }).toJson(QJsonDocument::Compact) + '\n');
    if (!waitFor([&] { return messageGetRequests == 1; }))
        return testFatal("a reaction did not ask for the message it changed");
    settle();
    if (client.messageList()->count() != before)
        return testFatal("a reaction threw away loaded history",
                         QStringLiteral("%1 -> %2").arg(before).arg(client.messageList()->count()));
    if (client.messageList()->byId(QStringLiteral("m1050")).value(QStringLiteral("reactions")).toList().isEmpty())
        return testFatal("the reaction never reached the message");

    // 3. A star lands on the message it was asked for.
    connection->write(QJsonDocument(QJsonObject{
        {QStringLiteral("version"), 1},
        {QStringLiteral("event"), QStringLiteral("message.starred")},
        {QStringLiteral("data"), QJsonObject{{QStringLiteral("chat_jid"), bob},
                                             {QStringLiteral("message_id"), QStringLiteral("m1050")},
                                             {QStringLiteral("starred"), true}}},
    }).toJson(QJsonDocument::Compact) + '\n');
    settle();
    if (!client.messageList()->byId(QStringLiteral("m1050")).value(QStringLiteral("starred")).toBool())
        return testFatal("the star did not reach the message it was meant for");
    if (client.messageList()->byId(QStringLiteral("m1090")).value(QStringLiteral("starred")).toBool())
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

    // 5. A refresh keeps the pages the reader had loaded rather than asking for
    // more than the daemon serves and being handed the smallest page instead.
    listRequests = 0;
    const int firstPage = client.messageList()->count();
    // Scroll back past one page's worth: a conversation with more than 200
    // messages loaded is what the refresh used to throw away.
    for (int page = 0; page < 5; ++page) {
        const int before = client.messageList()->count();
        client.loadOlderMessages();
        if (!waitFor([&] { return client.messageList()->count() >= before + 50; }))
            return testFatal("an older page was never added",
                             QString::number(client.messageList()->count()));
    }
    if (client.messageList()->count() <= 200 || client.messageList()->count() <= firstPage)
        return testFatal("the conversation did not grow past one page",
                         QString::number(client.messageList()->count()));
    const int scrolledBack = client.messageList()->count();
    client.refreshOpenMessages();
    settle(300);
    if (!waitFor([&] { return client.messageList()->count() >= scrolledBack; }, 5000))
        return testFatal("refreshing the conversation lost the pages that were loaded",
                         QStringLiteral("%1 -> %2").arg(scrolledBack).arg(client.messageList()->count()));
    if (biggestLimit > 200)
        return testFatal("the daemon was asked for a page larger than it serves",
                         QString::number(biggestLimit));

    // 6. The sidebar asks for the conversations after its first page, and a
    // refresh keeps them: a message arriving used to scroll the reader back to
    // the first two hundred conversations.
    const int listedBefore = client.chats().size();
    client.loadMoreChats();
    if (!waitFor([&] { return client.chats().size() > listedBefore; }))
        return testFatal("the sidebar never asked for the conversations after its first page");
    const int listedAfterPaging = client.chats().size();
    chatLimits.clear();
    client.refreshChats();
    settle(300);
    if (client.chats().size() < listedAfterPaging)
        return testFatal("a refresh threw away the conversations the sidebar had loaded",
                         QStringLiteral("%1 -> %2").arg(listedAfterPaging).arg(client.chats().size()));
    if (chatLimits.isEmpty() || chatLimits.constFirst() < listedAfterPaging)
        return testFatal("the refresh asked for fewer conversations than were loaded",
                         chatLimits.isEmpty() ? QStringLiteral("nothing was asked")
                                              : QString::number(chatLimits.constFirst()));

    // 7. Coming back finds the conversation again by itself: anything that
    // arrived while the connection was down is missing until it does, and the
    // presence subscription went with the socket.
    listRequests = 0;
    presenceRequests = 0;
    connection->abort();
    if (!waitFor([&] { return !client.daemonConnected(); }))
        return testFatal("the client did not notice the connection dropping");
    if (!waitFor([&] { return client.daemonConnected(); }, 10000))
        return testFatal("the client never reconnected");
    if (!waitFor([&] { return listRequests > 0; }, 5000))
        return testFatal("reconnecting left the open conversation as it was");
    if (presenceRequests == 0)
        return testFatal("the presence subscription was not renewed after reconnecting");

    // Attachment intent survives the desktop RPC boundary. These are fixture
    // URLs; the stub never opens files or sends anything to WhatsApp.
    const auto photoUrl = QUrl::fromLocalFile(QDir(runtime.path()).filePath(QStringLiteral("photo.jpg"))).toString();
    client.sendFile(photoUrl, QStringLiteral("original"), QStringLiteral("quoted"), true);
    if (!waitFor([&] { return mediaSends.size() == 1 && !client.busy(); }))
        return testFatal("the document send never reached the stub");
    if (!mediaSends.last().value(QStringLiteral("document")).toBool()
            || mediaSends.last().value(QStringLiteral("reply_to")).toString() != QStringLiteral("quoted"))
        return testFatal("the desktop discarded document or reply intent");
    client.sendFile(photoUrl);
    if (!waitFor([&] { return mediaSends.size() == 2 && !client.busy(); }))
        return testFatal("the normal photo send never reached the stub");
    if (mediaSends.last().value(QStringLiteral("document")).toBool())
        return testFatal("a normal photo was forced to a document");

    // A recorder callback supplies its captured recipient even when the
    // selected chat has changed; a callback from another profile is refused.
    client.sendVoice(photoUrl, alice, client.profile(), QStringLiteral("voice-reply"));
    if (!waitFor([&] { return mediaSends.size() == 3 && !client.busy(); }))
        return testFatal("the voice callback never reached the stub");
    if (mediaSends.last().value(QStringLiteral("chat_jid")).toString() != alice
            || !mediaSends.last().value(QStringLiteral("voice")).toBool()
            || mediaSends.last().value(QStringLiteral("reply_to")).toString() != QStringLiteral("voice-reply"))
        return testFatal("voice completion used the current chat instead of its recorded recipient");
    client.sendVoice(photoUrl, alice, QStringLiteral("another-account"));
    settle();
    if (mediaSends.size() != 3 || client.busy())
        return testFatal("a voice callback crossed accounts");

    client.loadStarredMessages();
    client.loadStarredMessages(alice);
    if (!waitFor([&] { return starredRequests.size() == 2; }))
        return testFatal("the starred-message requests never arrived");
    if (!starredRequests[0].value(QStringLiteral("params")).toObject().value(QStringLiteral("chat_jid")).toString().isEmpty()
            || starredRequests[1].value(QStringLiteral("params")).toObject().value(QStringLiteral("chat_jid")).toString() != alice)
        return testFatal("global and contact stars used the same scope");
    reply(starredRequests[1].value(QStringLiteral("id")), QJsonObject{
        {QStringLiteral("items"), QJsonArray{message(QStringLiteral("alice-star"), alice, 1)}}});
    if (!waitFor([&] { return !client.starredMessagesLoading() && client.starredMessages().size() == 1; }))
        return testFatal("contact stars never loaded");
    reply(starredRequests[0].value(QStringLiteral("id")), QJsonObject{
        {QStringLiteral("items"), QJsonArray{message(QStringLiteral("bob-star"), bob, 2)}}});
    settle();
    if (client.starredMessages().first().toMap().value(QStringLiteral("chat_jid")).toString() != alice)
        return testFatal("a late global answer replaced the contact's stars");

    // Completion carries the original identity, not whichever chat is selected
    // when the daemon eventually acknowledges the send.
    QVariantMap completedSend;
    QObject::connect(&client, &RpcClient::textSendFinished, &client,
        [&](const QString &profile, const QString &chat, const QString &text, const QString &quote, bool success) {
            completedSend = {{"profile", profile}, {"chat", chat}, {"text", text}, {"quote", quote}, {"success", success}};
        });
    const auto sentChat = client.selectedChat().value(QStringLiteral("jid")).toString();
    client.sendMessage(QStringLiteral("late acknowledgement"), QStringLiteral("original-reply"));
    if (!waitFor([&] { return sendRequests == 1 && client.busy(); }))
        return testFatal("the held send never reached the daemon");
    client.openChat(sentChat == alice ? bob : alice, QStringLiteral("Other chat"));
    reply(heldSendId, QJsonObject{});
    if (!waitFor([&] { return completedSend.value("success").toBool(); })
            || completedSend.value("chat").toString() != sentChat
            || completedSend.value("profile").toString() != client.profile()
            || completedSend.value("text").toString() != QStringLiteral("late acknowledgement")
            || completedSend.value("quote").toString() != QStringLiteral("original-reply"))
        return testFatal("the send acknowledgement lost its original draft identity");

    // 8. A send whose daemon disappears mid-request leaves the composer usable.
    client.sendMessage(QStringLiteral("hello"), QString());
    if (!waitFor([&] { return sendRequests == 2 && client.busy(); }))
        return testFatal("the send never reached the daemon");
    server.close();
    if (connection)
        connection->disconnectFromServer();
    if (!waitFor([&] { return !client.daemonConnected(); }))
        return testFatal("the client did not notice the daemon leaving");
    settle();
    if (client.busy())
        return testFatal("sending stayed disabled after the daemon disconnected");

    // 9. A photo turned in the preview is sent turned. The preview only
    // rotated what was on screen, so the picture arrived the way the camera
    // wrote it and the turning was lost.
    QImage upright(8, 4, QImage::Format_ARGB32);
    upright.fill(Qt::white);
    upright.setPixelColor(0, 0, QColor(Qt::red));
    const auto uprightPath = QDir(runtime.path()).filePath(QStringLiteral("upright.png"));
    if (!upright.save(uprightPath, "PNG"))
        return testFatal("could not write the picture to turn", uprightPath);
    const auto uprightUrl = QUrl::fromLocalFile(uprightPath).toString();
    if (client.rotatedImage(uprightUrl, 0) != uprightUrl)
        return testFatal("a picture nobody turned was copied anyway");
    const auto turnedUrl = client.rotatedImage(uprightUrl, 90);
    if (turnedUrl == uprightUrl)
        return testFatal("a picture turned in the preview was sent as it was");
    const QImage turned(QUrl(turnedUrl).toLocalFile());
    if (turned.width() != upright.height() || turned.height() != upright.width())
        return testFatal("the turned picture kept its old shape",
                         QStringLiteral("%1x%2").arg(turned.width()).arg(turned.height()));
    if (turned.pixelColor(turned.width() - 1, 0) != QColor(Qt::red))
        return testFatal("the picture was copied without being turned");

    // A request made with no daemon answers its caller too.
    client.sendClipboardImage(turnedUrl, QStringLiteral("keep caption"), QStringLiteral("keep-reply"));
    settle();
    if (!QFile::exists(QUrl(turnedUrl).toLocalFile()))
        return testFatal("failed clipboard send destroyed the image needed for retry");
    client.sendMessage(QStringLiteral("hello again"), QString());
    settle();
    if (client.busy())
        return testFatal("a send refused before it left the window left the composer disabled");

    return EXIT_SUCCESS;
}
