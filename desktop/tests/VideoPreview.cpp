#include "videopreview.h"

#include "TestSupport.h"
#include <QBuffer>
#include <QElapsedTimer>
#include <QGuiApplication>
#include <QImageReader>
#include <QJsonArray>
#include <QJsonObject>
#include <QTemporaryDir>
#include <QVideoFrame>
#include <QVideoFrameFormat>

#include <cstring>
#include <optional>

namespace {
int failures = 0;

void check(bool condition, const char *what)
{
    if (!condition) {
        qWarning("FAILED: %s", what);
        ++failures;
    }
}

QImage decoded(const QByteArray &jpeg, QByteArray *format = nullptr)
{
    QBuffer buffer;
    buffer.setData(jpeg);
    buffer.open(QIODevice::ReadOnly);
    QImageReader reader(&buffer);
    if (format)
        *format = reader.format();
    return reader.read();
}

QJsonObject discovery(const QJsonObject &sendMediaParams)
{
    return {{QStringLiteral("methods"),
             QJsonArray{QJsonObject{{QStringLiteral("name"), QStringLiteral("message.send")},
                                    {QStringLiteral("params_example"), QJsonObject{{QStringLiteral("thumbnail"), QString()}}}},
                        QJsonObject{{QStringLiteral("name"), QStringLiteral("message.send_media")},
                                    {QStringLiteral("params_example"), sendMediaParams}}}}};
}

void checkThumbnail()
{
    // WhatsApp's apps send about 160 pixels on the longer side, as JPEG.
    QImage frame(1280, 720, QImage::Format_RGB32);
    frame.fill(QColor(37, 211, 102));
    QByteArray format;
    const QImage still = decoded(VideoPreview::thumbnailJpeg(frame), &format);
    check(format == "jpeg", "the thumbnail is a JPEG");
    check(still.size() == QSize(160, 90), "a landscape frame becomes 160 by 90");
    const QColor colour = still.pixelColor(80, 45);
    check(qAbs(colour.green() - 211) < 12 && qAbs(colour.red() - 37) < 12, "the thumbnail keeps the frame's colour");

    QImage portrait(720, 1280, QImage::Format_ARGB32);
    portrait.fill(Qt::white);
    check(decoded(VideoPreview::thumbnailJpeg(portrait)).size() == QSize(90, 160), "a portrait frame becomes 90 by 160");

    QImage modest(320, 180, QImage::Format_RGB32);
    modest.fill(Qt::black);
    check(decoded(VideoPreview::thumbnailJpeg(modest)).size() == QSize(160, 90), "a frame just past 160 is scaled too");

    QImage small(100, 50, QImage::Format_RGB32);
    small.fill(Qt::red);
    check(decoded(VideoPreview::thumbnailJpeg(small)).size() == QSize(100, 50), "a small frame is not enlarged");

    check(VideoPreview::thumbnailJpeg(QImage()).isEmpty(), "no frame, no thumbnail");
    QImage strip(4000, 2, QImage::Format_RGB32);
    strip.fill(Qt::blue);
    const QImage thin = decoded(VideoPreview::thumbnailJpeg(strip));
    check(thin.width() == 160 && thin.height() >= 1, "a strip keeps a row of pixels");
}

QVideoFrame frameOf(const QImage &source)
{
    const QImage image = source.convertToFormat(QImage::Format_RGBX8888);
    QVideoFrame frame(QVideoFrameFormat(image.size(), QVideoFrameFormat::Format_RGBX8888));
    if (!frame.map(QVideoFrame::WriteOnly))
        return {};
    for (int y = 0; y < image.height(); ++y)
        memcpy(frame.bits(0) + y * frame.bytesPerLine(0), image.constScanLine(y), image.bytesPerLine());
    frame.unmap();
    return frame;
}

void checkUpright()
{
    // A red mark in the top left corner of a landscape frame.
    QImage source(16, 8, QImage::Format_RGB32);
    source.fill(Qt::white);
    for (int y = 0; y < 3; ++y)
        for (int x = 0; x < 3; ++x)
            source.setPixelColor(x, y, Qt::red);
    QVideoFrame frame = frameOf(source);
    check(frame.isValid(), "a frame can be built for the test");
    check(VideoPreview::uprightImage(frame).size() == QSize(16, 8), "a frame with no rotation keeps its shape");
#if QT_VERSION >= QT_VERSION_CHECK(6, 7, 0)
    // A frame shown turned a quarter clockwise: the picture is watched
    // standing up, and the mark that was top left is top right.
    frame.setRotation(QtVideo::Rotation::Clockwise90);
    const QImage upright = VideoPreview::uprightImage(frame);
    check(upright.size() == QSize(8, 16), "a frame turned a quarter is watched upright");
    const QColor topRight = upright.pixelColor(upright.width() - 1, 0);
    const QColor topLeft = upright.pixelColor(0, 0);
    check(topRight.red() > 200 && topRight.green() < 60, "it is turned clockwise, as the frame says");
    check(topLeft.green() > 200, "and not the other way");
#endif
}

void checkRequestFields()
{
    VideoPreview::Result result;
    result.thumbnail = QByteArrayLiteral("\xff\xd8 pretend jpeg");
    result.seconds = 12;
    result.size = QSize(1280, 720);
    const QJsonObject fields = VideoPreview::requestFields(result);
    check(QByteArray::fromBase64(fields.value(QStringLiteral("thumbnail")).toString().toLatin1()) == result.thumbnail,
          "the thumbnail goes as base64");
    check(fields.value(QStringLiteral("seconds")).toInt() == 12, "the length goes along");
    check(fields.value(QStringLiteral("width")).toInt() == 1280 && fields.value(QStringLiteral("height")).toInt() == 720,
          "the size goes along");

    check(VideoPreview::requestFields({}).isEmpty(), "nothing known, nothing sent");

    // The service refuses the whole send over a value past its limits, so
    // such a value is left out and the video still goes.
    VideoPreview::Result tooMuch;
    tooMuch.thumbnail = QByteArray(64 * 1024 + 1, 'x');
    tooMuch.seconds = 24 * 60 * 60 + 1;
    tooMuch.size = QSize(16385, 720);
    check(VideoPreview::requestFields(tooMuch).isEmpty(), "values the service would refuse are left out");

    VideoPreview::Result half;
    half.size = QSize(1280, 0);
    check(VideoPreview::requestFields(half).isEmpty(), "half a size is not sent");
}

void checkDiscovery()
{
    const QJsonObject all{{QStringLiteral("chat_jid"), QString()}, {QStringLiteral("thumbnail"), QString()},
                          {QStringLiteral("seconds"), 0}, {QStringLiteral("width"), 0}, {QStringLiteral("height"), 0}};
    check(VideoPreview::sendMediaTakesPreview(discovery(all)), "a service that lists the fields takes them");
    QJsonObject older = all;
    older.remove(QStringLiteral("thumbnail"));
    check(!VideoPreview::sendMediaTakesPreview(discovery(older)), "a service that does not list them is not sent them");
    check(!VideoPreview::sendMediaTakesPreview(discovery({{QStringLiteral("chat_jid"), QString()}})),
          "the fields listed on another method do not count");
    check(!VideoPreview::sendMediaTakesPreview(QJsonObject{}), "an empty answer is an older service");
    check(!VideoPreview::sendMediaTakesPreview(QJsonValue(QStringLiteral("nonsense"))), "a malformed answer is an older service");
}

// A video that cannot be read still has to answer, or the send it holds up
// never happens.
void checkUnreadableVideo(const QString &missing)
{
    std::optional<VideoPreview::Result> answer;
    QObject context;
    VideoPreview::inspect(missing, &context, [&answer](const VideoPreview::Result &result) { answer = result; }, 3000);
    QElapsedTimer clock;
    clock.start();
    while (!answer && clock.elapsed() < 10000)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 50);
    check(answer.has_value(), "an unreadable video is answered");
    check(answer && answer->thumbnail.isEmpty() && answer->seconds == 0 && answer->size.isEmpty(),
          "with nothing in the answer");

    // The window may be gone before the answer; nobody is called then.
    bool called = false;
    auto *gone = new QObject;
    VideoPreview::inspect(missing, gone, [&called](const VideoPreview::Result &) { called = true; }, 500);
    delete gone;
    clock.restart();
    while (clock.elapsed() < 1500)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 50);
    check(!called, "no answer goes to a window that has closed");
}
} // namespace

int main(int argc, char **argv)
{
    QTemporaryDir home;
    if (!home.isValid())
        return testFatal("could not make a temporary home", home.errorString());
    qputenv("HOME", home.path().toLocal8Bit());
    QGuiApplication app(argc, argv);

    checkThumbnail();
    checkUpright();
    checkRequestFields();
    checkDiscovery();
    checkUnreadableVideo(home.filePath(QStringLiteral("missing.mp4")));

    return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
