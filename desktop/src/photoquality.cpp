#include "photoquality.h"

#include <QColorSpace>
#include <QDir>
#include <QFileInfo>
#include <QImageReader>
#include <QImageWriter>
#include <QPainter>
#include <QObject>

PhotoQuality::Prepared PhotoQuality::prepareProfilePhoto(const QString &path)
{
    Prepared result;
    const QFileInfo info(path);
    if (!info.isFile() || info.size() > 20 * 1024 * 1024) {
        result.error = QObject::tr("Choose a JPEG or PNG photo under 20 MiB.");
        return result;
    }
    QImageReader reader(path);
    const auto format = reader.format().toLower();
    if ((format != "jpeg" && format != "jpg" && format != "png") || reader.supportsAnimation()) {
        result.error = QObject::tr("Choose a still JPEG or PNG photo.");
        return result;
    }
    if (format == "png") {
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly)) {
            result.error = QObject::tr("Could not read the photo.");
            return result;
        }
        file.seek(8);
        // Qt can treat APNG as a still PNG. Reject its animation control chunk.
        while (!file.atEnd()) {
            const auto chunk = file.read(8);
            if (chunk.size() != 8) break;
            if (chunk.mid(4) == "acTL") {
                result.error = QObject::tr("Choose a still photo, not an animated PNG.");
                return result;
            }
            if (chunk.mid(4) == "IDAT" || chunk.mid(4) == "IEND") break;
            quint32 length = 0;
            for (int i = 0; i < 4; ++i) length = (length << 8) | static_cast<unsigned char>(chunk[i]);
            if (qint64(length) + 4 > file.size() - file.pos()) break;
            if (!file.seek(file.pos() + qint64(length) + 4)) break;
        }
    }
    const auto size = reader.size();
    if (size.isEmpty() || qint64(size.width()) * size.height() > 32 * 1024 * 1024) {
        result.error = QObject::tr("This photo is too large to prepare safely (maximum 32 megapixels).");
        return result;
    }
    reader.setAutoTransform(true);
    auto image = reader.read();
    if (image.isNull()) {
        result.error = QObject::tr("Could not decode the photo. Your original was not changed.");
        return result;
    }
    if (image.colorSpace().isValid()) image.convertToColorSpace(QColorSpace::SRgb);
    const int edge = qMin(image.width(), image.height());
    image = image.copy((image.width() - edge) / 2, (image.height() - edge) / 2, edge, edge)
                 .scaled(640, 640, Qt::IgnoreAspectRatio, Qt::SmoothTransformation);
    // A fresh image strips EXIF/location metadata and flattens transparency.
    QImage output(640, 640, QImage::Format_RGB32);
    if (image.isNull() || output.isNull()) {
        result.error = QObject::tr("Not enough memory to prepare the photo.");
        return result;
    }
    output.fill(Qt::white);
    { QPainter painter(&output); painter.drawImage(0, 0, image); }
    auto temporary = std::make_shared<QTemporaryFile>(QDir::tempPath() + QStringLiteral("/whatsappgo-profile-photo-XXXXXX.jpg"));
    if (!temporary->open()) {
        result.error = QObject::tr("Could not create a private photo copy.");
        return result;
    }
    QImageWriter writer(temporary.get(), "jpeg");
    writer.setQuality(90);
    if (!writer.write(output) || !temporary->flush()) {
        result.error = QObject::tr("Could not prepare the profile photo.");
        return result;
    }
    temporary->close();
    result.path = temporary->fileName();
    result.temporary = std::move(temporary);
    return result;
}

PhotoQuality::Prepared PhotoQuality::prepare(const QString &path, const QString &quality)
{
    Prepared result{path, {}, {}};
    if (quality != QStringLiteral("standard") && quality != QStringLiteral("hd")) return result;
    QImageReader reader(path);
    const auto format = reader.format().toLower();
    // GIF/WebP may be animations or stickers. Unsupported formats retain
    // their bytes rather than quietly becoming a still frame.
    if (format != "jpeg" && format != "jpg" && format != "png") return result;
    if (reader.supportsAnimation()) return result;
    if (format == "png") {
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly)) {
            result.error = QObject::tr("Could not read the photo.");
            return result;
        }
        // Qt's PNG handler does not necessarily recognize APNG. Walk chunk
        // headers before IDAT; acTL always precedes image data in an APNG.
        file.seek(8);
        while (!file.atEnd()) {
            const auto chunk = file.read(8);
            if (chunk.size() != 8) break;
            if (chunk.mid(4) == "acTL") return result;
            if (chunk.mid(4) == "IDAT" || chunk.mid(4) == "IEND") break;
            quint32 length = 0;
            for (int i = 0; i < 4; ++i) length = (length << 8) | static_cast<unsigned char>(chunk[i]);
            if (qint64(length) + 4 > file.size() - file.pos()) break;
            if (!file.seek(file.pos() + qint64(length) + 4)) break;
        }
    }
    const auto size = reader.size();
    if (size.isEmpty() || qint64(size.width()) * size.height() > 32 * 1024 * 1024
        || QFileInfo(path).size() > 50 * 1024 * 1024) {
        result.error = QObject::tr("This photo is too large to resize safely. Choose Original quality or send it as a document.");
        return result;
    }
    reader.setAutoTransform(true); // Honor the camera's EXIF orientation.
    auto image = reader.read();
    if (image.isNull()) {
        result.error = QObject::tr("Could not decode the photo. The original was not changed.");
        return result;
    }
    const int edge = quality == QStringLiteral("hd") ? 3840 : 1600;
    if (image.width() > edge || image.height() > edge)
        image = image.scaled(edge, edge, Qt::KeepAspectRatio, Qt::SmoothTransformation);
    if (image.colorSpace().isValid()) image.convertToColorSpace(QColorSpace::SRgb);
    // Flatten transparency to white, not black. A fresh image also omits EXIF
    // location/text metadata from converted photos. Original mode retains it.
    QImage output(image.size(), QImage::Format_RGB32);
    if (output.isNull()) {
        result.error = QObject::tr("Not enough memory to prepare the photo.");
        return result;
    }
    output.fill(Qt::white);
    {
        QPainter painter(&output);
        painter.drawImage(0, 0, image);
    }
    auto temporary = std::make_shared<QTemporaryFile>(QDir::tempPath() + QStringLiteral("/whatsappgo-photo-XXXXXX.jpg"));
    if (!temporary->open()) {
        result.error = QObject::tr("Could not create a private photo copy.");
        return result;
    }
    QImageWriter writer(temporary.get(), "jpeg");
    writer.setQuality(quality == QStringLiteral("hd") ? 90 : 80);
    if (!writer.write(output) || !temporary->flush()) {
        result.error = QObject::tr("Could not prepare the photo for upload.");
        return result;
    }
    temporary->close();
    result.path = temporary->fileName();
    result.temporary = std::move(temporary);
    return result;
}
