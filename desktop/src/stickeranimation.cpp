#include "stickeranimation.h"

#include <QFile>
#include <QFileInfo>
#include <QFutureWatcher>
#include <QPainter>
#include <QPromise>
#include <QThreadPool>
#include <atomic>
#ifdef WHATSAPPGO_WEBP
#include <webp/decode.h>
#include <webp/demux.h>
#endif

struct StickerAnimation::Decoder {
    std::atomic_bool cancelled = false;
    QByteArray bytes;
    int timestamp = 0;
#ifdef WHATSAPPGO_WEBP
    WebPAnimDecoder *decoder = nullptr;
    WebPAnimInfo info{};
    ~Decoder() { if (decoder) WebPAnimDecoderDelete(decoder); }
#endif
};

namespace {
struct Frame {
    QImage image;
    QString error;
    int delay = 20;
};
QThreadPool &stickerPool()
{
    static QThreadPool pool;
    static const bool configured = [] { pool.setMaxThreadCount(1); return true; }();
    Q_UNUSED(configured);
    return pool;
}
}

StickerAnimation::StickerAnimation(QQuickItem *parent) : QQuickPaintedItem(parent)
{
    setRenderTarget(QQuickPaintedItem::Image);
    m_timer.setSingleShot(true);
    connect(&m_timer, &QTimer::timeout, this, &StickerAnimation::nextFrame);
    connect(this, &QQuickItem::visibleChanged, this, [this] {
        if (!isVisible()) m_timer.stop();
        else if (m_playing) nextFrame();
    });
}

StickerAnimation::~StickerAnimation()
{
    if (m_decoder) m_decoder->cancelled = true;
}

bool StickerAnimation::supported() const
{
#ifdef WHATSAPPGO_WEBP
    return true;
#else
    return false;
#endif
}

void StickerAnimation::setSource(const QUrl &source)
{
    if (source == m_source) return;
    ++m_generation;
    m_timer.stop();
    if (m_decoder) m_decoder->cancelled = true;
    m_decoder = std::make_shared<Decoder>();
    m_source = source; m_image = {}; m_error.clear(); m_pending = false;
    emit sourceChanged(); emit frameChanged(); emit errorChanged(); update();
    if (m_playing && isVisible()) nextFrame();
}

void StickerAnimation::setPlaying(bool playing)
{
    if (m_playing == playing) return;
    m_playing = playing;
    emit playingChanged();
    if (playing && isVisible()) nextFrame();
    else m_timer.stop();
}

void StickerAnimation::nextFrame()
{
    if (m_pending || !m_playing || !isVisible() || m_source.isEmpty() || !m_error.isEmpty()) return;
    if (!supported() || !m_source.isLocalFile()) {
        m_error = tr("Animated stickers are unavailable in this build."); emit errorChanged(); return;
    }
    if (!m_decoder) m_decoder = std::make_shared<Decoder>();
    m_pending = true;
    const auto state = m_decoder;
    const auto path = m_source.toLocalFile();
    const auto generation = m_generation;
    auto promise = std::make_shared<QPromise<Frame>>();
    auto *watcher = new QFutureWatcher<Frame>(this);
    connect(watcher, &QFutureWatcher<Frame>::finished, this, [this, watcher, generation] {
        const auto frame = watcher->result();
        watcher->deleteLater();
        if (generation != m_generation) return;
        m_pending = false;
        if (!frame.error.isEmpty()) {
            m_error = frame.error; emit errorChanged(); return;
        }
        if (!frame.image.isNull()) {
            m_image = frame.image; m_delay = frame.delay;
            emit frameChanged(); update();
        }
        if (m_playing && isVisible()) m_timer.start(m_delay);
    });
    promise->start(); watcher->setFuture(promise->future());
    stickerPool().start([state, path, promise] {
        Frame frame;
        const auto finish = [&] { promise->addResult(frame); promise->finish(); };
        if (state->cancelled) { finish(); return; }
#ifdef WHATSAPPGO_WEBP
        if (!state->decoder) {
            QFile file(path);
            if (!QFileInfo(file).isFile() || !file.open(QIODevice::ReadOnly) || file.size() > (1 << 20)) {
                frame.error = tr("Sticker is unavailable or exceeds 1 MiB."); finish(); return;
            }
            state->bytes = file.read((1 << 20) + 1);
            const auto *data = reinterpret_cast<const uint8_t *>(state->bytes.constData());
            int width = 0, height = 0;
            if (state->bytes.size() > (1 << 20) || !WebPGetInfo(data, state->bytes.size(), &width, &height)
                || width < 1 || height < 1 || width > 512 || height > 512) {
                frame.error = tr("Sticker has unsupported dimensions or invalid data."); finish(); return;
            }
            WebPData input{data, size_t(state->bytes.size())};
            WebPAnimDecoderOptions options;
            if (!WebPAnimDecoderOptionsInit(&options)) {
                frame.error = tr("The WebP decoder is incompatible."); finish(); return;
            }
            options.color_mode = MODE_RGBA;
            options.use_threads = 0;
            state->decoder = WebPAnimDecoderNew(&input, &options);
            if (!state->decoder || !WebPAnimDecoderGetInfo(state->decoder, &state->info)
                || state->info.frame_count > 600 || state->info.canvas_width > 512 || state->info.canvas_height > 512) {
                frame.error = tr("Sticker animation is invalid or too complex."); finish(); return;
            }
        }
        if (!WebPAnimDecoderHasMoreFrames(state->decoder)) {
            WebPAnimDecoderReset(state->decoder);
            state->timestamp = 0;
        }
        uint8_t *pixels = nullptr;
        int timestamp = 0;
        if (!WebPAnimDecoderGetNext(state->decoder, &pixels, &timestamp) || timestamp > 120000) {
            frame.error = tr("Sticker animation could not be decoded."); finish(); return;
        }
        if (!state->cancelled) {
            frame.image = QImage(pixels, int(state->info.canvas_width), int(state->info.canvas_height), QImage::Format_RGBA8888).copy();
            frame.delay = qBound(20, timestamp - state->timestamp, 10000);
            state->timestamp = timestamp;
        }
#endif
        finish();
    });
}

void StickerAnimation::paint(QPainter *painter)
{
    if (m_image.isNull()) return;
    const auto scaled = QSizeF(m_image.size()).scaled(size(), Qt::KeepAspectRatio);
    const QRectF target((width() - scaled.width()) / 2, (height() - scaled.height()) / 2, scaled.width(), scaled.height());
    painter->setRenderHint(QPainter::SmoothPixmapTransform);
    painter->drawImage(target, m_image);
}
