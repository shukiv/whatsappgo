#include "profilemonitor.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QTemporaryDir>
#include <QTimer>

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir directory;
    if (!directory.isValid())
        return testFatal("could not create a temporary directory");
    const auto path = directory.filePath(QStringLiteral("profile.sock"));
    QLocalServer server;
    if (!server.listen(path))
        return testFatal("could not listen on the socket", path + QStringLiteral(": ") + server.errorString());

    int requests = 0;
    QObject::connect(&server, &QLocalServer::newConnection, &app, [&] {
        auto *socket = server.nextPendingConnection();
        QObject::connect(socket, &QLocalSocket::readyRead, socket, [&, socket] {
            while (socket->canReadLine()) {
                const auto request = QJsonDocument::fromJson(socket->readLine()).object();
                if (request.value(QStringLiteral("method")).toString() != QStringLiteral("chats.unread_count"))
                    QCoreApplication::exit(EXIT_FAILURE);
                ++requests;
                const auto count = requests == 1 ? 7 : (requests == 2 ? 8 : 0);
                if (requests <= 2) {
                    // A message arriving, then this account reading its
                    // conversations on another device: the second reaches the
                    // daemon as a receipt, and the badge has to follow it.
                    QJsonObject event{{QStringLiteral("version"), 1},
                                      {QStringLiteral("event"), requests == 1
                                          ? QStringLiteral("message.upsert")
                                          : QStringLiteral("message.receipt")},
                                      {QStringLiteral("data"), QJsonObject{}}};
                    socket->write(QJsonDocument(event).toJson(QJsonDocument::Compact) + '\n');
                }
                QJsonObject response{{QStringLiteral("version"), 1},
                                     {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                                     {QStringLiteral("result"), QJsonObject{{QStringLiteral("count"), count}}}};
                socket->write(QJsonDocument(response).toJson(QJsonDocument::Compact) + '\n');
            }
        });
    });

    ProfileMonitor monitor(QStringLiteral("work"), path);
    QObject::connect(&monitor, &ProfileMonitor::countChanged, &app,
                     [&](const QString &profile, int count) {
                         if (profile != QStringLiteral("work"))
                             QCoreApplication::exit(EXIT_FAILURE);
                         if (count == 0 && requests >= 3)
                             QCoreApplication::exit(EXIT_SUCCESS);
                     });
    QTimer::singleShot(3000, &app, [] { QCoreApplication::exit(EXIT_FAILURE); });
    return app.exec();
}
