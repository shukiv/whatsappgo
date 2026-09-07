#include "lucideprovider.h"

#include <QColor>
#include <QFile>
#include <QImage>
#include <QPainter>
#include <QRegularExpression>
#include <QSvgRenderer>

LucideProvider::LucideProvider() : QQuickImageProvider(QQuickImageProvider::Image)
{
}

QImage LucideProvider::requestImage(const QString &id, QSize *size, const QSize &requestedSize)
{
    // The URL contains only a semantic resource name and a Qt RGB/ARGB hex tint.
    // Never allow a caller to turn this into a filesystem or network SVG loader.
    static const QRegularExpression validId(QStringLiteral("^([a-z0-9-]+)/([a-fA-F0-9]{6}|[a-fA-F0-9]{8})$"));
    const auto match = validId.match(id);
    if (!match.hasMatch())
        return {};

    const QString name = match.captured(1);
    QFile file(QStringLiteral(":/qt/qml/org/whatsappgo/qml/icons/%1.svg").arg(name));
    if (!file.open(QIODevice::ReadOnly))
        return {};
    QByteArray svg = file.readAll();
    if (name == QLatin1String("star-filled"))
        svg.replace("fill=\"none\"", "fill=\"currentColor\"");
    QSvgRenderer renderer(svg);
    if (!renderer.isValid())
        return {};

    if (size)
        *size = QSize(24, 24);
    const int edge = qBound(1, requestedSize.isValid()
                                 ? qMin(requestedSize.width(), requestedSize.height()) : 24, 512);
    QImage image(edge, edge, QImage::Format_ARGB32_Premultiplied);
    image.fill(Qt::transparent);
    QPainter painter(&image);
    painter.setRenderHint(QPainter::Antialiasing);
    renderer.render(&painter, QRectF(0, 0, edge, edge));
    painter.setCompositionMode(QPainter::CompositionMode_SourceIn);
    painter.fillRect(image.rect(), QColor(QLatin1Char('#') + match.captured(2)));
    painter.end();
    return image;
}
