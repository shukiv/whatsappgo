#pragma once

#include <QByteArray>
#include <QImage>
#include <QJsonObject>
#include <QJsonValue>
#include <QSize>
#include <QString>

#include <functional>

class QObject;
class QVideoFrame;

// What a video message carries so a phone can show the video before it is
// downloaded: a small still, the length and the picture size. WhatsApp's own
// apps always send them. A video sent without them is a grey box with a play
// button on the phone, and no length.
namespace VideoPreview {

struct Result {
    // JPEG; empty when no frame could be read.
    QByteArray thumbnail;
    int seconds = 0;
    // The picture as it is watched, upright; empty when unknown.
    QSize size;
};

// The still a video message embeds: the frame scaled to fit 160 pixels on its
// longer side, as JPEG. The previews phones attached to videos this account
// received were 160 by 90 at quality 92, two to three kilobytes.
QByteArray thumbnailJpeg(const QImage &frame);

// A decoded frame as it is watched. toImage() turns the picture by the
// rotation a phone recorded in the file, but not by a frame's own rotation.
QImage uprightImage(const QVideoFrame &frame);

// The message.send_media fields for a result; empty when there is nothing.
// Only values the service accepts are included, since it refuses the whole
// send over one it does not.
QJsonObject requestFields(const Result &result);

// Whether an rpc.discover answer says message.send_media takes those fields. A
// service refuses a field it does not know, so sending them to an older one
// would fail the send itself.
bool sendMediaTakesPreview(const QJsonValue &discovery);

// Opens the video without sound, takes its first frame and its length, and
// calls done once on context's thread: with whatever it has after timeoutMs,
// or an empty result if the video cannot be read. done is never called if
// context is destroyed first.
void inspect(const QString &path, QObject *context, std::function<void(const Result &)> done, int timeoutMs = 5000);

} // namespace VideoPreview
