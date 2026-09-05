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

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir runtime(shortTempTemplate(QStringLiteral("wag-chats")));
    if (!runtime.isValid())
        return testFatal("could not create a temporary runtime directory");
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return testFatal("could not create the socket directory", socketDir);

    QLocalServer server;
    const auto socketPath = RpcClient::socketPathForProfile(QStringLiteral("chatlist"));
    if (!server.listen(socketPath))
        return testFatal("could not listen on the socket", socketPath + QStringLiteral(": ") + server.errorString());

    int chatListRequests = 0;
    int avatarRequests = 0;
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
                const auto params = request.value(QStringLiteral("params")).toObject();
                QJsonValue result = QJsonObject{};
                if (method == QStringLiteral("status.get")) {
                    result = QJsonObject{{QStringLiteral("state"), QStringLiteral("connected")},
                                         {QStringLiteral("connected"), true},
                                         {QStringLiteral("logged_in"), true}};
                } else if (method == QStringLiteral("chats.list")) {
                    if (!params.value(QStringLiteral("archived")).toBool()) {
                        ++chatListRequests;
                        result = QJsonArray{
                            QJsonObject{{QStringLiteral("jid"), QStringLiteral("alice@lid")},
                                        {QStringLiteral("title"), QStringLiteral("Alice")}},
                            QJsonObject{{QStringLiteral("jid"), QStringLiteral("bob@lid")},
                                        {QStringLiteral("title"), QStringLiteral("Bob")}},
                        };
                    } else {
                        result = QJsonArray{};
                    }
                } else if (method == QStringLiteral("chat.avatar")) {
                    ++avatarRequests;
                    const auto path = QStringLiteral("/tmp/alice-refreshed.png");
                    socket->write(QJsonDocument(QJsonObject{
                        {QStringLiteral("version"), 1},
                        {QStringLiteral("event"), QStringLiteral("chat.updated")},
                        {QStringLiteral("data"), QJsonObject{
                            {QStringLiteral("jid"), QStringLiteral("alice@lid")},
                            {QStringLiteral("avatar_path"), path},
                        }},
                    }).toJson(QJsonDocument::Compact) + '\n');
                    result = QJsonObject{{QStringLiteral("path"), path}};
                }
                socket->write(QJsonDocument(QJsonObject{
                    {QStringLiteral("version"), 1},
                    {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                    {QStringLiteral("result"), result},
                }).toJson(QJsonDocument::Compact) + '\n');
            }
        });
    });

    RpcClient client(QStringLiteral("chatlist"), QString{});
    auto *model = qobject_cast<ChatListModel *>(client.chatListModel());
    if (model == nullptr)
        return testFatal("the client did not expose a chat list model");
    int resets = 0;
    QObject::connect(model, &QAbstractItemModel::modelReset, &app, [&resets] { ++resets; });

    bool avatarRequested = false;
    bool avatarApplied = false;
    QTimer poll;
    poll.setInterval(10);
    QObject::connect(&poll, &QTimer::timeout, &app, [&] {
        if (!avatarRequested && model->count() == 2) {
            avatarRequested = true;
            client.refreshChatAvatar(QStringLiteral("alice@lid"));
            return;
        }
        if (avatarRequested && model->count() == 2
            && model->at(0).value(QStringLiteral("avatar_path")).toString()
                == QStringLiteral("/tmp/alice-refreshed.png")) {
            avatarApplied = true;
            QTimer::singleShot(100, &app, &QCoreApplication::quit);
            poll.stop();
        }
    });
    poll.start();
    QTimer::singleShot(3000, &app, &QCoreApplication::quit);
    app.exec();

    return avatarApplied && avatarRequests == 1 && chatListRequests == 1 && resets == 0
        ? EXIT_SUCCESS : EXIT_FAILURE;
}
