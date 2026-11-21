#include "rpcclient.h"

#include "TestSupport.h"
#include <QBuffer>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QGuiApplication>
#include <QImageReader>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLocalServer>
#include <QLocalSocket>
#include <QTemporaryDir>
#include <QUrl>

#include <optional>

// A video sent without a thumbnail and a length is a grey box on the phone.
// This checks what the window puts in message.send_media for a video, against
// a stand-in service: nothing for a service that does not list the fields,
// which would refuse the send, and a send that still goes when the video
// cannot be read.
//
// The repository carries no video, so the case with a readable one runs only
// when one is given: video-send-test --video clip.mp4
namespace {
int failures = 0;

void check(bool condition, const char *what)
{
    if (!condition) {
        qWarning("FAILED: %s", what);
        ++failures;
    }
}

const QStringList previewFields{QStringLiteral("thumbnail"), QStringLiteral("seconds"), QStringLiteral("width"),
                                QStringLiteral("height")};

struct Sent {
    bool discoveryAsked = false;
    std::optional<QJsonObject> params;
};

// Serves one profile; answers rpc.discover with or without the preview fields
// and records the message.send_media it receives.
Sent sendThrough(const QString &profile, bool serviceTakesPreview, const QString &file, bool document)
{
    Sent sent;
    QLocalServer server;
    const auto socketPath = RpcClient::socketPathForProfile(profile);
    if (!server.listen(socketPath)) {
        testFatal("could not listen on the socket", socketPath + QStringLiteral(": ") + server.errorString());
        ++failures;
        return sent;
    }
    bool discoveryAnswered = false;
    QByteArray input;
    QObject::connect(&server, &QLocalServer::newConnection, &server, [&] {
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
                    result = QJsonObject{{QStringLiteral("messages"), QJsonArray{}}, {QStringLiteral("has_more"), false}};
                } else if (method == QStringLiteral("rpc.discover")) {
                    sent.discoveryAsked = true;
                    QJsonObject params{{QStringLiteral("chat_jid"), QString()}, {QStringLiteral("path"), QString()}};
                    if (serviceTakesPreview) {
                        for (const auto &field : previewFields)
                            params.insert(field, field == QStringLiteral("thumbnail") ? QJsonValue(QString()) : QJsonValue(0));
                    }
                    result = QJsonObject{{QStringLiteral("methods"),
                                          QJsonArray{QJsonObject{{QStringLiteral("name"), QStringLiteral("message.send_media")},
                                                                 {QStringLiteral("params_example"), params}}}}};
                    discoveryAnswered = true;
                } else if (method == QStringLiteral("message.send_media")) {
                    sent.params = request.value(QStringLiteral("params")).toObject();
                }
                socket->write(QJsonDocument(QJsonObject{{QStringLiteral("version"), 1},
                                                        {QStringLiteral("id"), request.value(QStringLiteral("id"))},
                                                        {QStringLiteral("result"), result}})
                                  .toJson(QJsonDocument::Compact) + '\n');
            }
        });
    });

    RpcClient client(profile, QStringLiteral("alice@lid"));
    QElapsedTimer clock;
    clock.start();
    while (!(discoveryAnswered && client.selectedChat().value(QStringLiteral("jid")).toString() == QStringLiteral("alice@lid"))
           && clock.elapsed() < 5000)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 20);
    // The discovery answer has been written; let the client read it.
    clock.restart();
    while (clock.elapsed() < 200)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 20);
    check(sent.discoveryAsked, "the window asks the service what it takes");

    client.sendFile(QUrl::fromLocalFile(file).toString(), {}, {}, document, QStringLiteral("standard"));
    clock.restart();
    while (!sent.params && clock.elapsed() < 10000)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 20);
    check(sent.params.has_value(), "the video is sent");
    return sent;
}

bool carriesNoPreview(const Sent &sent)
{
    if (!sent.params)
        return false;
    for (const auto &field : previewFields) {
        if (sent.params->contains(field))
            return false;
    }
    return true;
}
} // namespace

int main(int argc, char **argv)
{
    QTemporaryDir runtime(shortTempTemplate(QStringLiteral("wag-video")));
    if (!runtime.isValid())
        return testFatal("could not create a temporary runtime directory");
    qputenv("HOME", runtime.path().toUtf8());
    qputenv("XDG_RUNTIME_DIR", runtime.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", QDir(runtime.path()).filePath(QStringLiteral("config")).toUtf8());
    qputenv("WHATSAPPGO_DISABLE_PROFILE_MONITORS", "1");
    QGuiApplication app(argc, argv);
    const auto socketDir = QDir(runtime.path()).filePath(QStringLiteral("whatsappgo"));
    if (!QDir().mkpath(socketDir))
        return testFatal("could not create the socket directory", socketDir);

    QString readable;
    const auto arguments = app.arguments();
    const auto videoArgument = arguments.indexOf(QStringLiteral("--video"));
    if (videoArgument > 0 && videoArgument + 1 < arguments.size())
        readable = QFileInfo(arguments.at(videoArgument + 1)).absoluteFilePath();

    // Named as a video, but nothing a player can open.
    const auto unreadable = QDir(runtime.path()).filePath(QStringLiteral("clip.mp4"));
    QFile bogus(unreadable);
    if (!bogus.open(QIODevice::WriteOnly) || bogus.write("not a video") < 0)
        return testFatal("could not write the unreadable video", unreadable);
    bogus.close();

    const auto unread = sendThrough(QStringLiteral("videounread"), true, unreadable, false);
    check(carriesNoPreview(unread), "a video that cannot be read goes without a preview");

    const auto sample = readable.isEmpty() ? unreadable : readable;
    check(carriesNoPreview(sendThrough(QStringLiteral("videoolder"), false, sample, false)),
          "a service that does not list the fields is not sent them");
    check(carriesNoPreview(sendThrough(QStringLiteral("videodoc"), true, sample, true)),
          "a video sent as a document carries no preview");

    if (!readable.isEmpty()) {
        const auto sent = sendThrough(QStringLiteral("videonew"), true, readable, false);
        const auto params = sent.params.value_or(QJsonObject{});
        QBuffer jpeg;
        jpeg.setData(QByteArray::fromBase64(params.value(QStringLiteral("thumbnail")).toString().toLatin1()));
        jpeg.open(QIODevice::ReadOnly);
        QImageReader reader(&jpeg);
        const auto format = reader.format();
        const auto still = reader.read();
        check(format == "jpeg" && !still.isNull() && qMax(still.width(), still.height()) == 160,
              "a readable video goes with a JPEG still 160 pixels on its longer side");
        check(params.value(QStringLiteral("seconds")).toInt() > 0, "and its length");
        check(params.value(QStringLiteral("width")).toInt() > 0 && params.value(QStringLiteral("height")).toInt() > 0,
              "and its size");
        qInfo("sent with %s: %d s, %dx%d, still %dx%d", qPrintable(readable), params.value(QStringLiteral("seconds")).toInt(),
              params.value(QStringLiteral("width")).toInt(), params.value(QStringLiteral("height")).toInt(), still.width(),
              still.height());
    }

    return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
