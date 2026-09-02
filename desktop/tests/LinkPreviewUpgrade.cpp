#include "rpcclient.h"

#include <QCoreApplication>
#include <QDir>
#include <QImage>
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
    QTemporaryDir runtime;
    if (!runtime.isValid())
        return EXIT_FAILURE;
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return EXIT_FAILURE;

    const auto thumbnail = QDir(runtime.path()).filePath(QStringLiteral("tiny.jpg"));
    if (!QImage(80, 80, QImage::Format_RGB32).save(thumbnail, "JPEG"))
        return EXIT_FAILURE;

    QLocalServer server;
    if (!server.listen(QDir(socketDir).filePath(QStringLiteral("whatsappd-linkupgrade.sock"))))
        return EXIT_FAILURE;

    bool refreshRequested = false;
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
                    result = QJsonObject{
                        {QStringLiteral("messages"),
                         QJsonArray{QJsonObject{{QStringLiteral("id"), QStringLiteral("link-1")},
                                                {QStringLiteral("chat_jid"), QStringLiteral("alice@lid")},
                                                {QStringLiteral("kind"), QStringLiteral("text")},
                                                {QStringLiteral("body"), QStringLiteral("https://example.com")},
                                                {QStringLiteral("link_url"), QStringLiteral("https://example.com")},
                                                {QStringLiteral("link_thumbnail"), thumbnail}}}},
                        {QStringLiteral("has_more"), false}};
                } else if (method == QStringLiteral("link.preview.refresh")) {
                    const auto params = request.value(QStringLiteral("params")).toObject();
                    refreshRequested = params.value(QStringLiteral("chat_jid")).toString() == QStringLiteral("alice@lid")
                        && params.value(QStringLiteral("message_id")).toString() == QStringLiteral("link-1");
                    if (refreshRequested)
                        QTimer::singleShot(0, &app, &QCoreApplication::quit);
                }
                socket->write(QJsonDocument(QJsonObject{{QStringLiteral("version"), 1},
                                                        {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                                                        {QStringLiteral("result"), result}})
                                  .toJson(QJsonDocument::Compact) + '\n');
            }
        });
    });

    RpcClient client(QStringLiteral("linkupgrade"), QStringLiteral("alice@lid"));
    QTimer::singleShot(2000, &app, &QCoreApplication::quit);
    app.exec();
    return refreshRequested ? EXIT_SUCCESS : EXIT_FAILURE;
}
