#include "videopreview.h"

#include <QBuffer>
#include <QJsonArray>
#include <QMediaPlayer>
#include <QPointer>
#include <QTimer>
#include <QTransform>
#include <QUrl>
#include <QVideoFrame>
#include <QVideoSink>

#include <climits>
#include <utility>

namespace VideoPreview {
namespace {

constexpr int thumbnailSide = 160;
constexpr int thumbnailQuality = 90;
// What the service accepts (internal/service/videopreview.go). A value past
// these would fail the send, so it is left out instead.
constexpr int maxThumbnailBytes = 64 * 1024;
constexpr int maxSeconds = 24 * 60 * 60;
constexpr int maxSide = 16384;

// One video being looked at. It plays the video with no audio output, which
// is silent, takes the first frame it is shown, and deletes itself.
class Inspection : public QObject
{
public:
    Inspection(const QString &path, QObject *context, std::function<void(const Result &)> done, int timeoutMs)
        : m_context(context), m_done(std::move(done))
    {
        m_player.setVideoSink(&m_sink);
        // The sink may be fed from a decoding thread; this object is the
        // context, so the frame is taken on its own thread.
        connect(&m_sink, &QVideoSink::videoFrameChanged, this, [this](const QVideoFrame &frame) { takeFrame(frame); });
        connect(&m_player, &QMediaPlayer::durationChanged, this, [this](qint64 ms) { takeDuration(ms); });
        connect(&m_player, &QMediaPlayer::mediaStatusChanged, this, [this](QMediaPlayer::MediaStatus status) {
            switch (status) {
            case QMediaPlayer::LoadedMedia:
                takeDuration(m_player.duration());
                m_player.play();
                break;
            case QMediaPlayer::InvalidMedia:
            case QMediaPlayer::EndOfMedia:
                finish();
                break;
            default:
                break;
            }
        });
        connect(&m_player, &QMediaPlayer::errorOccurred, this, [this] { finish(); });
        m_timer.setSingleShot(true);
        connect(&m_timer, &QTimer::timeout, this, [this] { finish(); });
        m_timer.start(timeoutMs);
        m_player.setSource(QUrl::fromLocalFile(path));
    }

private:
    void takeDuration(qint64 ms)
    {
        if (ms > 0)
            m_result.seconds = int(qMin<qint64>(INT_MAX, qMax<qint64>(1, (ms + 500) / 1000)));
    }

    void takeFrame(const QVideoFrame &frame)
    {
        if (m_finished || !frame.isValid())
            return;
        const QImage image = uprightImage(frame);
        if (image.isNull())
            return;
        m_result.thumbnail = thumbnailJpeg(image);
        m_result.size = image.size();
        takeDuration(m_player.duration());
        finish();
    }

    void finish()
    {
        if (m_finished)
            return;
        m_finished = true;
        m_timer.stop();
        m_player.stop();
        const auto done = std::exchange(m_done, {});
        if (done && m_context)
            done(m_result);
        deleteLater();
    }

    QPointer<QObject> m_context;
    std::function<void(const Result &)> m_done;
    Result m_result;
    bool m_finished = false;
    QTimer m_timer;
    // The player is declared last so it goes first, while its sink still exists.
    QVideoSink m_sink;
    QMediaPlayer m_player;
};

} // namespace

QByteArray thumbnailJpeg(const QImage &frame)
{
    if (frame.isNull())
        return {};
    QImage still = frame;
    if (qMax(still.width(), still.height()) > thumbnailSide)
        still = still.scaled(thumbnailSide, thumbnailSide, Qt::KeepAspectRatio, Qt::SmoothTransformation);
    if (still.isNull())
        return {};
    still = still.convertToFormat(QImage::Format_RGB888);
    QByteArray bytes;
    QBuffer buffer(&bytes);
    if (!buffer.open(QIODevice::WriteOnly) || !still.save(&buffer, "JPEG", thumbnailQuality))
        return {};
    return bytes.size() <= maxThumbnailBytes ? bytes : QByteArray();
}

QImage uprightImage(const QVideoFrame &frame)
{
    QImage image = frame.toImage();
#if QT_VERSION >= QT_VERSION_CHECK(6, 7, 0)
    const int turn = int(frame.rotation());
    if (!image.isNull() && turn != 0)
        image = image.transformed(QTransform().rotate(turn));
#endif
    return image;
}

QJsonObject requestFields(const Result &result)
{
    QJsonObject fields;
    if (!result.thumbnail.isEmpty() && result.thumbnail.size() <= maxThumbnailBytes)
        fields.insert(QStringLiteral("thumbnail"), QString::fromLatin1(result.thumbnail.toBase64()));
    if (result.seconds > 0 && result.seconds <= maxSeconds)
        fields.insert(QStringLiteral("seconds"), result.seconds);
    const auto size = result.size;
    if (size.width() > 0 && size.height() > 0 && size.width() <= maxSide && size.height() <= maxSide) {
        fields.insert(QStringLiteral("width"), size.width());
        fields.insert(QStringLiteral("height"), size.height());
    }
    return fields;
}

bool sendMediaTakesPreview(const QJsonValue &discovery)
{
    const auto methods = discovery.toObject().value(QStringLiteral("methods")).toArray();
    for (const auto &entry : methods) {
        const auto method = entry.toObject();
        if (method.value(QStringLiteral("name")).toString() != QStringLiteral("message.send_media"))
            continue;
        const auto params = method.value(QStringLiteral("params_example")).toObject();
        for (const auto *field : {"thumbnail", "seconds", "width", "height"}) {
            if (!params.contains(QLatin1String(field)))
                return false;
        }
        return true;
    }
    return false;
}

void inspect(const QString &path, QObject *context, std::function<void(const Result &)> done, int timeoutMs)
{
    // Deletes itself when it finishes.
    new Inspection(path, context, std::move(done), timeoutMs);
}

} // namespace VideoPreview
