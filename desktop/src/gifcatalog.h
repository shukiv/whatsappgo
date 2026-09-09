#pragma once

#include "appsettings.h"
#include <QObject>
#include <QNetworkAccessManager>
#include <QPointer>
#include <QTemporaryFile>
#include <QVariantList>
#include <QtQml/qqmlregistration.h>
#include <functional>
#include <memory>

// Provider requests originate on the end user's desktop. Search words and
// credentials never enter the WhatsApp daemon or its diagnostic logs.
class GifCatalog : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(AppSettings *settings READ settings WRITE setSettings NOTIFY changed)
    Q_PROPERTY(QVariantList results READ results NOTIFY changed)
    Q_PROPERTY(bool busy READ busy NOTIFY changed)
    Q_PROPERTY(bool preparing READ preparing NOTIFY changed)
    Q_PROPERTY(bool hasMore READ hasMore NOTIFY changed)
    Q_PROPERTY(QString error READ error NOTIFY changed)
    Q_PROPERTY(QString preparedUrl READ preparedUrl NOTIFY changed)
    Q_PROPERTY(QString providerName READ providerName NOTIFY changed)
    Q_PROPERTY(bool configured READ configured NOTIFY changed)
public:
    explicit GifCatalog(QObject *parent = nullptr);
    AppSettings *settings() const { return m_settings; }
    void setSettings(AppSettings *settings);
    QVariantList results() const { return m_results; }
    bool busy() const { return m_busy; }
    bool preparing() const { return m_preparing; }
    bool hasMore() const { return m_hasMore; }
    QString error() const { return m_error; }
    QString preparedUrl() const;
    QString providerName() const;
    bool configured() const;
    Q_INVOKABLE void search(const QString &query, bool more = false);
    Q_INVOKABLE void prepare(int index);
    Q_INVOKABLE void cancelPreview();
    Q_INVOKABLE void clear();
    // Hold the file until the daemon acknowledges the upload, even when the
    // picker closes or the user changes accounts during the request.
    Q_INVOKABLE QString holdPrepared();
    Q_INVOKABLE void release(const QString &url);
signals:
    void changed();
private:
    using Response = std::function<void(QByteArray, int)>;
    QNetworkReply *fetch(const QUrl &url, qint64 limit, bool api, Response done);
    void loadThumbnails();
    bool mediaURL(const QUrl &url) const;
    QPointer<AppSettings> m_settings;
    QNetworkAccessManager m_network;
    QList<QPointer<QNetworkReply>> m_requests;
    QPointer<QNetworkReply> m_download;
    QVariantList m_results;
    QString m_query, m_next, m_error;
    bool m_busy = false, m_preparing = false, m_hasMore = false;
    int m_generation = 0, m_selection = 0, m_thumbnails = 0;
    QList<int> m_thumbnailQueue;
    qint64 m_retryAt = 0;
    std::shared_ptr<QTemporaryFile> m_prepared;
    QHash<QString, std::shared_ptr<QTemporaryFile>> m_uploads;
};
