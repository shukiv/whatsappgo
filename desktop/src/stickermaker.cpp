#include "stickermaker.h"

#include <QBuffer>
#include <QColorSpace>
#include <QDir>
#include <QFileInfo>
#include <QFutureWatcher>
#include <QImageReader>
#include <QPainter>
#include <QPromise>
#include <QThreadPool>
#include <QUrl>
#ifdef WHATSAPPGO_WEBP
#include <webp/encode.h>
#include <webp/decode.h>
#endif

namespace {
struct PreparedSticker {
    std::shared_ptr<QTemporaryFile> file;
    std::shared_ptr<QTemporaryFile> preview;
    QString error;
};

PreparedSticker prepareSticker(const QString &path)
{
    const auto fail = [](const char *message) { return PreparedSticker{{}, {}, StickerMaker::tr(message)}; };
    constexpr qint64 maxInput = 20 * 1024 * 1024;
    QFile source(path);
    if (!QFileInfo(path).isFile() || !source.open(QIODevice::ReadOnly))
        return fail("Could not read the image. Choose a JPEG or PNG from your computer.");
    auto bytes = source.read(maxInput + 1);
    if (bytes.size() > maxInput || source.error() != QFileDevice::NoError)
        return fail("Choose a JPEG or PNG under 20 MiB.");
    QBuffer input(&bytes);
    input.open(QIODevice::ReadOnly);
    QImageReader reader(&input);
    const auto format = reader.format().toLower();
    if ((format != "jpeg" && format != "jpg" && format != "png") || reader.supportsAnimation())
        return fail("Choose a still JPEG or PNG. Animated sticker creation is not supported.");
    if (format == "png") {
        // Qt may read APNG as a still image. Detect its control chunk before
        // IDAT rather than silently discarding every frame but the first.
        for (qsizetype pos = 8; pos + 8 <= bytes.size();) {
            const auto type = bytes.mid(pos + 4, 4);
            if (type == "acTL") return fail("Choose a still PNG, not an animated PNG.");
            if (type == "IDAT" || type == "IEND") break;
            quint32 length = 0;
            for (int i = 0; i < 4; ++i) length = (length << 8) | static_cast<unsigned char>(bytes[pos + i]);
            if (qint64(length) + 12 > bytes.size() - pos) break;
            pos += qsizetype(length) + 12;
        }
    }
    const auto size = reader.size();
    if (size.isEmpty() || qint64(size.width()) * size.height() > 32 * 1024 * 1024)
        return fail("This image is too large to prepare safely (maximum 32 megapixels).");
    reader.setAutoTransform(true);
    auto image = reader.read();
    if (image.isNull()) return fail("Could not decode this image. Your original was not changed.");
    if (image.colorSpace().isValid()) image.convertToColorSpace(QColorSpace::SRgb);
    image = image.scaled(480, 480, Qt::KeepAspectRatio, Qt::SmoothTransformation);
    QImage canvas(512, 512, QImage::Format_ARGB32);
    if (image.isNull() || canvas.isNull()) return fail("Not enough memory to prepare this sticker.");
    canvas.fill(Qt::transparent);
    { QPainter painter(&canvas); painter.drawImage((512 - image.width()) / 2, (512 - image.height()) / 2, image); }
    // A fresh canvas omits source EXIF/location data. Bound both dimensions
    // and output bytes to the documented static-sticker limits.
    QByteArray encoded;
#ifdef WHATSAPPGO_WEBP
    const auto rgba = canvas.convertToFormat(QImage::Format_RGBA8888);
    if (rgba.isNull()) return fail("Not enough memory to prepare this sticker.");
    for (int quality : {90, 75, 60, 45, 30, 15}) {
        uint8_t *output = nullptr;
        const auto length = WebPEncodeRGBA(rgba.constBits(), 512, 512, int(rgba.bytesPerLine()), float(quality), &output);
        if (!length || !output) {
            WebPFree(output);
            return fail("WebP export failed. Sticker creation may be unavailable in this build.");
        }
        encoded = QByteArray(reinterpret_cast<const char *>(output), qsizetype(length));
        WebPFree(output);
        if (!encoded.isEmpty() && encoded.size() <= 100000) break;
    }
#else
    return fail("WebP export is unavailable in this build.");
#endif
    if (encoded.isEmpty() || encoded.size() > 100000)
        return fail("This image is too detailed for a static sticker. Choose a simpler image.");
    auto file = std::make_shared<QTemporaryFile>(QDir::tempPath() + QStringLiteral("/whatsappgo-sticker-XXXXXX.webp"));
    if (!file->open() || file->write(encoded) != encoded.size() || !file->flush())
        return fail("Could not create the private sticker copy. Check free disk space.");
    file->close();
    // Qt installations need not include a WebP image plugin. Decode the actual
    // encoded sticker with our existing libwebp dependency for an exact PNG
    // preview, including any compression artifacts, without another install.
    QImage previewImage(512, 512, QImage::Format_RGBA8888);
#ifdef WHATSAPPGO_WEBP
    if (previewImage.isNull() || !WebPDecodeRGBAInto(reinterpret_cast<const uint8_t *>(encoded.constData()),
            size_t(encoded.size()), previewImage.bits(), size_t(previewImage.sizeInBytes()), int(previewImage.bytesPerLine())))
        return fail("Could not prepare the sticker preview.");
#endif
    auto preview = std::make_shared<QTemporaryFile>(QDir::tempPath() + QStringLiteral("/whatsappgo-sticker-preview-XXXXXX.png"));
    if (!preview->open() || !previewImage.save(preview.get(), "PNG") || !preview->flush())
        return fail("Could not create the private sticker preview. Check free disk space.");
    preview->close();
    return {file, preview, {}};
}
}

bool StickerMaker::supported() const
{
#ifdef WHATSAPPGO_WEBP
    return true;
#else
    return false;
#endif
}

QString StickerMaker::preparedUrl() const
{
    return m_prepared ? QUrl::fromLocalFile(m_prepared->fileName()).toString() : QString();
}

QString StickerMaker::previewUrl() const
{
    return m_preview ? QUrl::fromLocalFile(m_preview->fileName()).toString() : QString();
}

void StickerMaker::clear()
{
    ++m_generation;
    m_preparing = false;
    m_prepared.reset();
    m_preview.reset();
    m_error.clear();
    emit changed();
}

void StickerMaker::prepare(const QString &localUrl)
{
    if (m_preparing) return;
    clear();
    const QUrl url(localUrl);
    if (!url.isLocalFile() || !supported()) {
        m_error = !supported() ? tr("WebP export is unavailable in this build.") : tr("Choose an image from your computer.");
        emit changed();
        return;
    }
    const auto generation = m_generation;
    const auto path = url.toLocalFile();
    m_preparing = true;
    emit changed();
    auto promise = std::make_shared<QPromise<PreparedSticker>>();
    auto *watcher = new QFutureWatcher<PreparedSticker>(this);
    connect(watcher, &QFutureWatcher<PreparedSticker>::finished, this, [this, watcher, generation] {
        const auto result = watcher->result();
        watcher->deleteLater();
        if (generation != m_generation) return;
        m_preparing = false;
        m_prepared = result.file;
        m_preview = result.preview;
        m_error = result.error;
        emit changed();
    });
    promise->start();
    watcher->setFuture(promise->future());
    static QThreadPool pool;
    pool.setMaxThreadCount(1);
    pool.start([promise, path] { promise->addResult(prepareSticker(path)); promise->finish(); });
}

QString StickerMaker::holdPrepared()
{
    const auto url = preparedUrl();
    if (!url.isEmpty()) m_uploads.insert(url, m_prepared);
    return url;
}

void StickerMaker::release(const QString &url) { m_uploads.remove(url); }
