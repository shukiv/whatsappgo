#include "rpcclient.h"

#include "TestSupport.h"
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QTemporaryDir>

#include <functional>

namespace {

// waitFor turns the event loop until the condition holds or the time is up.
bool waitFor(const std::function<bool()> &done, int timeoutMs = 3000)
{
    QElapsedTimer clock;
    clock.start();
    while (!done() && clock.elapsed() < timeoutMs)
        QCoreApplication::processEvents(QEventLoop::WaitForMoreEvents, 50);
    return done();
}

}

// What the client does with an update the daemon found: it asks about one as
// soon as it connects, and a download reports its progress and the file it
// ended with.
int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir runtime(shortTempTemplate(QStringLiteral("wag-update")));
    if (!runtime.isValid())
        return testFatal("could not create a temporary runtime directory");
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return testFatal("could not create the socket directory", socketDir);

    QLocalServer server;
    const auto socketPath = RpcClient::socketPathForProfile(QStringLiteral("update"));
    if (!server.listen(socketPath))
        return testFatal("could not listen on the socket", socketPath + QStringLiteral(": ") + server.errorString());

    int statusRequests = 0;
    int checkRequests = 0;
    int downloadRequests = 0;
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
                } else if (method == QStringLiteral("update.status")) {
                    ++statusRequests;
                    result = QJsonObject{{QStringLiteral("current"), QStringLiteral("v1.0.0")},
                                         {QStringLiteral("available"), true},
                                         {QStringLiteral("latest"), QStringLiteral("v1.1.0")},
                                         {QStringLiteral("url"), QStringLiteral("https://example.test/v1.1.0")},
                                         {QStringLiteral("installable"), true}};
                } else if (method == QStringLiteral("update.check")) {
                    ++checkRequests;
                    result = QJsonObject{{QStringLiteral("current"), QStringLiteral("v1.0.0")},
                                         {QStringLiteral("available"), true},
                                         {QStringLiteral("latest"), QStringLiteral("v1.1.0")},
                                         {QStringLiteral("url"), QStringLiteral("https://example.test/v1.1.0")},
                                         {QStringLiteral("installable"), true}};
                } else if (method == QStringLiteral("update.download")) {
                    ++downloadRequests;
                    // The daemon answers at once and reports the transfer as it
                    // goes, which is what the reader watches.
                    const QJsonObject events[] = {
                        QJsonObject{{QStringLiteral("event"), QStringLiteral("update.progress")},
                                    {QStringLiteral("data"), QJsonObject{
                                        {QStringLiteral("received"), 512},
                                        {QStringLiteral("total"), 1024}}}},
                        QJsonObject{{QStringLiteral("event"), QStringLiteral("update.ready")},
                                    {QStringLiteral("data"), QJsonObject{
                                        {QStringLiteral("path"), QStringLiteral("/tmp/WhatsAppGo-x86_64.AppImage")},
                                        {QStringLiteral("version"), QStringLiteral("v1.1.0")}}}},
                    };
                    for (const auto &event : events) {
                        auto payload = event;
                        payload.insert(QStringLiteral("version"), 1);
                        socket->write(QJsonDocument(payload).toJson(QJsonDocument::Compact) + '\n');
                    }
                    result = QJsonObject{{QStringLiteral("started"), true}};
                }
                socket->write(QJsonDocument(QJsonObject{
                    {QStringLiteral("version"), 1},
                    {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                    {QStringLiteral("result"), result},
                }).toJson(QJsonDocument::Compact) + '\n');
            }
        });
    });

    // A second daemon that has not looked for a release in days: opening the
    // window is when somebody would want to know, so the window asks by itself.
    QLocalServer staleServer;
    const auto stalePath = RpcClient::socketPathForProfile(QStringLiteral("update-stale"));
    if (!staleServer.listen(stalePath))
        return testFatal("could not listen on the stale socket", stalePath);
    int staleChecks = 0;
    QByteArray staleInput;
    QObject::connect(&staleServer, &QLocalServer::newConnection, &app, [&] {
        auto *socket = staleServer.nextPendingConnection();
        QObject::connect(socket, &QLocalSocket::readyRead, socket, [&, socket] {
            staleInput += socket->readAll();
            while (true) {
                const auto newline = staleInput.indexOf('\n');
                if (newline < 0)
                    break;
                const auto request = QJsonDocument::fromJson(staleInput.left(newline)).object();
                staleInput.remove(0, newline + 1);
                const auto method = request.value(QStringLiteral("method")).toString();
                QJsonValue result = QJsonObject{};
                if (method == QStringLiteral("update.status")) {
                    result = QJsonObject{{QStringLiteral("current"), QStringLiteral("v1.0.0")},
                                         {QStringLiteral("available"), false},
                                         {QStringLiteral("checked_at"), 0},
                                         {QStringLiteral("installable"), true}};
                } else if (method == QStringLiteral("update.check")) {
                    ++staleChecks;
                    result = QJsonObject{{QStringLiteral("current"), QStringLiteral("v1.0.0")},
                                         {QStringLiteral("available"), false},
                                         {QStringLiteral("checked_at"), 1},
                                         {QStringLiteral("installable"), true}};
                }
                socket->write(QJsonDocument(QJsonObject{
                    {QStringLiteral("version"), 1},
                    {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                    {QStringLiteral("result"), result},
                }).toJson(QJsonDocument::Compact) + '\n');
            }
        });
    });
    RpcClient stale(QStringLiteral("update-stale"));
    QString staleOutcome;
    bool staleAnnounced = false;
    QObject::connect(&stale, &RpcClient::updateCheckFinished, &app,
                     [&](bool, const QString &, const QString &) { staleAnnounced = true; });

    RpcClient client(QStringLiteral("update"));
    qint64 received = 0;
    qint64 total = 0;
    QString readyPath;
    QString readyVersion;
    Q_UNUSED(staleOutcome);
    QString failure;
    QString offered;
    int offers = 0;
    QObject::connect(&client, &RpcClient::updateAvailable, &app, [&](const QString &version) {
        offered = version;
        ++offers;
    });
    QObject::connect(&client, &RpcClient::updateProgress, &app, [&](qint64 got, qint64 expected) {
        received = got;
        total = expected;
    });
    QObject::connect(&client, &RpcClient::updateReady, &app, [&](const QString &path, const QString &version) {
        readyPath = path;
        readyVersion = version;
    });
    QObject::connect(&client, &RpcClient::updateFailed, &app, [&](const QString &message) {
        failure = message;
    });

    // Connecting is enough to learn about an update; nothing has to ask.
    if (!waitFor([&] { return statusRequests > 0 && !client.updateStatus().isEmpty(); }))
        return testFatal("the client never asked the daemon about updates");
    if (client.updateStatus().value(QStringLiteral("latest")).toString() != QStringLiteral("v1.1.0"))
        return testFatal("the newer version did not reach the client",
                         client.updateStatus().value(QStringLiteral("latest")).toString());
    // The daemon announces a release once, when it first sees it, and it
    // starts before this window exists. The status answer has to offer it as
    // well or nobody is ever asked about an update they can install.
    if (!waitFor([&] { return offers > 0; }))
        return testFatal("connecting to a daemon that already found an update offered nothing");
    if (offered != QStringLiteral("v1.1.0"))
        return testFatal("the wrong version was offered", offered);

    // Pressing the control has to say what came of it: a check that answers
    // nothing is a button that looks broken.
    bool checkAnnounced = false;
    bool checkAvailable = false;
    QString checkVersion;
    QString checkError;
    bool sawChecking = false;
    QObject::connect(&client, &RpcClient::checkingForUpdatesChanged, &app, [&] {
        if (client.checkingForUpdates())
            sawChecking = true;
    });
    QObject::connect(&client, &RpcClient::updateCheckFinished, &app,
                     [&](bool available, const QString &version, const QString &error) {
        checkAnnounced = true;
        checkAvailable = available;
        checkVersion = version;
        checkError = error;
    });
    client.checkForUpdates();
    if (!waitFor([&] { return checkAnnounced; }))
        return testFatal("a check the reader asked for answered nothing");
    if (!sawChecking)
        return testFatal("the control never showed that a check was running");
    if (client.checkingForUpdates())
        return testFatal("the check never finished");
    if (!checkAvailable || checkVersion != QStringLiteral("v1.1.0") || !checkError.isEmpty())
        return testFatal("the check reported the wrong outcome", checkVersion + QStringLiteral(" ") + checkError);
    if (checkRequests != 1)
        return testFatal("update.check was asked a different number of times",
                         QString::number(checkRequests));

    // The daemon that last looked days ago is asked again, quietly.
    if (!waitFor([&] { return staleChecks > 0; }))
        return testFatal("a daemon that had not looked in a day was never asked");
    if (staleAnnounced)
        return testFatal("a check nobody asked for announced itself anyway");

    client.downloadUpdate();
    if (!waitFor([&] { return !readyPath.isEmpty(); }))
        return testFatal("the finished download was never announced");
    if (downloadRequests != 1)
        return testFatal("update.download was requested a different number of times",
                         QString::number(downloadRequests));
    if (received != 512 || total != 1024)
        return testFatal("the progress numbers did not arrive intact",
                         QStringLiteral("%1/%2").arg(received).arg(total));
    if (readyPath != QStringLiteral("/tmp/WhatsAppGo-x86_64.AppImage") || readyVersion != QStringLiteral("v1.1.0"))
        return testFatal("the downloaded file was not named", readyPath);
    if (client.updateStatus().value(QStringLiteral("downloaded")).toString().isEmpty())
        return testFatal("the status forgot the file that was downloaded");
    if (!failure.isEmpty())
        return testFatal("a successful download reported a failure", failure);

    // Installing something that was never downloaded says so rather than
    // acting on an empty path.
    RpcClient bare(QStringLiteral("update-none"));
    QString bareFailure;
    QObject::connect(&bare, &RpcClient::updateFailed, &app, [&](const QString &message) {
        bareFailure = message;
    });
    if (bare.installUpdate())
        return testFatal("installing with nothing downloaded claimed to work");
    if (bareFailure.isEmpty())
        return testFatal("installing with nothing downloaded said nothing");
    return EXIT_SUCCESS;
}
