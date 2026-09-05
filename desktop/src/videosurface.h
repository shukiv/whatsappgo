#pragma once

#include <QImage>
#include <QQuickPaintedItem>
#include <QVideoSink>

// VideoSurface draws decoded video frames with QPainter instead of a scene
// graph texture.
//
// Qt Quick's software renderer cannot draw a VideoOutput at all: it has no
// texture path, so the item stays black while the audio plays. This
// application runs on the software renderer by default - see main.cpp, where
// QT_QUICK_BACKEND is set - so every video was black in a default
// installation. The same happens on any machine that cannot create a GL
// drawable, which is the usual case inside a container or over a remote
// display.
//
// A QQuickPaintedItem has none of that. Decoding is unaffected: the frames
// arrive through a QVideoSink exactly as they do for VideoOutput, and only the
// last one is kept.
class VideoSurface : public QQuickPaintedItem
{
    Q_OBJECT
    QML_ELEMENT
    // MediaPlayer::videoOutput accepts any object exposing a videoSink, which
    // is how this stands in for VideoOutput from QML.
    Q_PROPERTY(QVideoSink *videoSink READ videoSink CONSTANT)
    // True once a frame has arrived. QML can hold a thumbnail until then
    // rather than showing an empty rectangle while the first frame decodes.
    Q_PROPERTY(bool hasFrame READ hasFrame NOTIFY hasFrameChanged)

public:
    explicit VideoSurface(QQuickItem *parent = nullptr);

    // Q_INVOKABLE, not merely a property: QMediaPlayer resolves its video
    // output by invoking a method of this name through the meta-object, and
    // logs "No such method VideoSurface::videoSink()" without it.
    Q_INVOKABLE QVideoSink *videoSink() { return &m_sink; }
    bool hasFrame() const { return !m_image.isNull(); }

    void paint(QPainter *painter) override;

signals:
    void hasFrameChanged();

private:
    void setFrameImage(QImage image);

    QVideoSink m_sink;
    QImage m_image;
};
