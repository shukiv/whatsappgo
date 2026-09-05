#include "videosurface.h"

#include "TestSupport.h"
#include <QGuiApplication>
#include <QImage>
#include <QPainter>
#include <QVideoFrame>
#include <QVideoFrameFormat>

namespace {
int failures = 0;

void check(bool condition, const char *what)
{
    if (!condition) {
        qWarning("FAILED: %s", what);
        ++failures;
    }
}
}

// Qt Quick's software scene graph cannot draw a VideoOutput, and this
// application selects that scene graph by default, so every video played its
// audio against a black rectangle. VideoSurface paints the frames with
// QPainter instead. This checks that a frame pushed into its sink reaches the
// paint call, which is the whole of the difference.
int main(int argc, char **argv)
{
    QGuiApplication app(argc, argv);

    VideoSurface surface;
    surface.setSize(QSizeF(64, 64));
    check(!surface.hasFrame(), "a new surface reports no frame");

    QImage source(16, 8, QImage::Format_RGBX8888);
    source.fill(QColor(0, 200, 100));
    QVideoFrame frame(QVideoFrameFormat(source.size(), QVideoFrameFormat::Format_RGBX8888));
    if (!frame.map(QVideoFrame::WriteOnly))
        return testFatal("could not map a video frame for writing");
    for (int y = 0; y < source.height(); ++y)
        memcpy(frame.bits(0) + y * frame.bytesPerLine(0), source.constScanLine(y), source.bytesPerLine());
    frame.unmap();

    surface.videoSink()->setVideoFrame(frame);
    check(surface.hasFrame(), "a frame pushed into the sink reaches the surface");

    QImage canvas(64, 64, QImage::Format_RGB32);
    canvas.fill(Qt::black);
    {
        QPainter painter(&canvas);
        surface.paint(&painter);
    }

    // The frame is 2:1 and the item is square, so it is letterboxed: the middle
    // row carries the video and the top row stays as the caller painted it.
    const QColor middle = canvas.pixelColor(32, 32);
    const QColor top = canvas.pixelColor(32, 2);
    check(middle != QColor(Qt::black), "the frame is painted");
    check(qAbs(middle.green() - 200) < 24 && qAbs(middle.blue() - 100) < 24,
          "the painted frame keeps its colour");
    check(top == QColor(Qt::black), "the frame is letterboxed rather than stretched");

    return failures == 0 ? EXIT_SUCCESS : EXIT_FAILURE;
}
