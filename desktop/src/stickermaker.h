#pragma once

#include <QObject>
#include <QHash>
#include <QTemporaryFile>
#include <QtQml/qqmlregistration.h>
#include <memory>

// Local-only preparation. The original is never modified; upload is a separate
// explicit action, with a private file lease until its RPC acknowledgement.
class StickerMaker : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(bool supported READ supported CONSTANT)
    Q_PROPERTY(bool preparing READ preparing NOTIFY changed)
    Q_PROPERTY(QString preparedUrl READ preparedUrl NOTIFY changed)
    Q_PROPERTY(QString previewUrl READ previewUrl NOTIFY changed)
    Q_PROPERTY(QString error READ error NOTIFY changed)
public:
    explicit StickerMaker(QObject *parent = nullptr) : QObject(parent) {}
    bool supported() const;
    bool preparing() const { return m_preparing; }
    QString preparedUrl() const;
    QString previewUrl() const;
    QString error() const { return m_error; }
    Q_INVOKABLE void prepare(const QString &localUrl);
    Q_INVOKABLE void clear();
    Q_INVOKABLE QString holdPrepared();
    Q_INVOKABLE void release(const QString &url);
signals:
    void changed();
private:
    quint64 m_generation = 0;
    bool m_preparing = false;
    QString m_error;
    std::shared_ptr<QTemporaryFile> m_prepared;
    std::shared_ptr<QTemporaryFile> m_preview;
    QHash<QString, std::shared_ptr<QTemporaryFile>> m_uploads;
};
