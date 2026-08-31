#include "rpcclient.h"

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
    QTemporaryDir runtime;
    if (!runtime.isValid())
        return EXIT_FAILURE;
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return EXIT_FAILURE;

    QLocalServer server;
    const auto socketPath = QDir(socketDir).filePath(QStringLiteral("whatsappd-autopair.sock"));
    if (!server.listen(socketPath))
        return EXIT_FAILURE;

    bool pairingStarted = false;
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
                                         {QStringLiteral("logged_in"), false}};
                } else if (method == QStringLiteral("chats.list")) {
                    result = QJsonArray{};
                } else if (method == QStringLiteral("pairing.start")) {
                    pairingStarted = true;
                }
                QJsonObject response{{QStringLiteral("version"), 1},
                                     {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                                     {QStringLiteral("result"), result}};
                socket->write(QJsonDocument(response).toJson(QJsonDocument::Compact) + '\n');
                if (pairingStarted)
                    QTimer::singleShot(0, &app, &QCoreApplication::quit);
            }
        });
    });

    RpcClient client(QStringLiteral("autopair"), QString{});
    QTimer::singleShot(2000, &app, &QCoreApplication::quit);
    app.exec();
    return pairingStarted ? EXIT_SUCCESS : EXIT_FAILURE;
}
