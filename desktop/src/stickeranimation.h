#pragma once

#include <QImage>
#include <QQuickPaintedItem>
#include <QTimer>
#include <QUrl>
#include <memory>
#include <QtQml/qqmlregistration.h>

// Incremental, bounded WebP decoding; no optional Qt image plugin required.
class StickerAnimation : public QQuickPaintedItem
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(QUrl source READ source WRITE setSource NOTIFY sourceChanged)
    Q_PROPERTY(bool playing READ playing WRITE setPlaying NOTIFY playingChanged)
    Q_PROPERTY(bool hasFrame READ hasFrame NOTIFY frameChanged)
    Q_PROPERTY(bool supported READ supported CONSTANT)
    Q_PROPERTY(QString error READ error NOTIFY errorChanged)
public:
    explicit StickerAnimation(QQuickItem *parent = nullptr);
    ~StickerAnimation() override;
    QUrl source() const { return m_source; }
    void setSource(const QUrl &source);
    bool playing() const { return m_playing; }
    void setPlaying(bool playing);
    bool hasFrame() const { return !m_image.isNull(); }
    bool supported() const;
    QString error() const { return m_error; }
    void paint(QPainter *painter) override;
signals:
    void sourceChanged();
    void playingChanged();
    void frameChanged();
    void errorChanged();
private:
    struct Decoder;
    void nextFrame();
    QUrl m_source;
    QImage m_image;
    QString m_error;
    bool m_playing = false, m_pending = false;
    int m_generation = 0, m_delay = 20;
    QTimer m_timer;
    std::shared_ptr<Decoder> m_decoder;
};
