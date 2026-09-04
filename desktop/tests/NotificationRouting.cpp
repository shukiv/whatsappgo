#include "rpcclient.h"

#include <QCoreApplication>
#include <QDir>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QTemporaryDir>
#include <QTimer>

namespace {
int failures = 0;

void check(bool condition, const char *what)
{
    if (!condition) {
        qWarning("FAILED: %s", what);
        ++failures;
    }
}

QByteArray notification(const QString &handled)
{
    QJsonObject data{
        {QStringLiteral("chat_jid"), QStringLiteral("alice@s.whatsapp.net")},
        {QStringLiteral("title"), QStringLiteral("Alice")},
        {QStringLiteral("body"), QStringLiteral("Hello")},
    };
    if (!handled.isEmpty())
        data.insert(QStringLiteral("handled"), handled);
    return QJsonDocument(QJsonObject{
               {QStringLiteral("version"), 1},
               {QStringLiteral("event"), QStringLiteral("notification.received")},
               {QStringLiteral("data"), data},
           }).toJson(QJsonDocument::Compact)
        + '\n';
}
}

// A daemon that already showed a notification through the desktop's own
// service marks it handled. Presenting it again in the client would give the
// user two notifications for one message on Linux; ignoring the unhandled ones
// would give them none on Windows and macOS.
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

    QLocalServer server;
    if (!server.listen(RpcClient::socketPathForProfile(QStringLiteral("notify"))))
        return EXIT_FAILURE;

    QLocalSocket *daemon = nullptr;
    QObject::connect(&server, &QLocalServer::newConnection, &app, [&] {
        daemon = server.nextPendingConnection();
        // Answer every request with an empty result so the client stays quiet.
        QObject::connect(daemon, &QLocalSocket::readyRead, daemon, [daemon] {
            static QByteArray input;
            input += daemon->readAll();
            while (true) {
                const auto newline = input.indexOf('\n');
                if (newline < 0)
                    break;
                const auto request = QJsonDocument::fromJson(input.left(newline)).object();
                input.remove(0, newline + 1);
                daemon->write(QJsonDocument(QJsonObject{
                                  {QStringLiteral("version"), 1},
                                  {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                                  {QStringLiteral("result"), QJsonObject{}},
                              }).toJson(QJsonDocument::Compact)
                              + '\n');
            }
        });
    });

    RpcClient client(QStringLiteral("notify"));
    QStringList presented;
    QObject::connect(&client, &RpcClient::notificationRequested, &app,
                     [&presented](const QString &chatJid, const QString &title, const QString &body) {
                         presented << (chatJid + '|' + title + '|' + body);
                     });

    QTimer::singleShot(600, &app, [&] {
        check(daemon != nullptr, "the client connected to the fake daemon");
        if (daemon == nullptr) {
            app.quit();
            return;
        }
        daemon->write(notification(QStringLiteral("1")));
        daemon->flush();
        QTimer::singleShot(200, &app, [&] {
            check(presented.isEmpty(), "a notification the daemon already showed stays out of the client");
            daemon->write(notification(QStringLiteral("0")));
            daemon->flush();
            QTimer::singleShot(200, &app, [&] {
                check(presented.size() == 1, "the client presents a notification the daemon could not");
                if (presented.size() == 1) {
                    check(presented.first()
                              == QStringLiteral("alice@s.whatsapp.net|Alice|Hello"),
                          "the notification carries its chat, title and body");
                }
                app.quit();
            });
        });
    });

    app.exec();
    return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
