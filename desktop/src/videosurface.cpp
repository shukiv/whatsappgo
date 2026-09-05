#include "videosurface.h"

#include <QPainter>
#include <QVideoFrame>

VideoSurface::VideoSurface(QQuickItem *parent)
    : QQuickPaintedItem(parent)
{
    // The Image render target is the point of this class: it paints into a
    // QImage on the CPU. FramebufferObject would need the GL context that is
    // missing exactly where VideoOutput already fails.
    setRenderTarget(QQuickPaintedItem::Image);
    setSmooth(true);
    connect(&m_sink, &QVideoSink::videoFrameChanged, this, [this](const QVideoFrame &frame) {
        setFrameImage(frame.isValid() ? frame.toImage() : QImage());
    });
}

void VideoSurface::setFrameImage(QImage image)
{
    const bool had = hasFrame();
    m_image = std::move(image);
    if (had != hasFrame())
        emit hasFrameChanged();
    update();
}

void VideoSurface::paint(QPainter *painter)
{
    if (m_image.isNull())
        return;
    // Letterboxed, the way VideoOutput's PreserveAspectFit draws it. A video
    // stretched to the item's shape is worse than bars beside it.
    const QSizeF bounds = size();
    const QSizeF scaled = QSizeF(m_image.size()).scaled(bounds, Qt::KeepAspectRatio);
    const QRectF target((bounds.width() - scaled.width()) / 2,
                        (bounds.height() - scaled.height()) / 2,
                        scaled.width(), scaled.height());
    painter->setRenderHint(QPainter::SmoothPixmapTransform, true);
    painter->drawImage(target, m_image);
}
