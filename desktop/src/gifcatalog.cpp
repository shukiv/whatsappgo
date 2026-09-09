#include "gifcatalog.h"

#include <QBuffer>
#include <QDateTime>
#include <QDir>
#include <QImageReader>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkReply>
#include <QUrlQuery>
#include <QTimer>

GifCatalog::GifCatalog(QObject *parent) : QObject(parent), m_network(this) {}

void GifCatalog::setSettings(AppSettings *settings)
{
    if (m_settings == settings) return;
    if (m_settings) disconnect(m_settings, nullptr, this, nullptr);
    m_settings = settings;
    if (settings) connect(settings, &AppSettings::changed, this, &GifCatalog::clear);
    clear();
}

QString GifCatalog::providerName() const
{
    if (!m_settings) return {};
    const auto provider = m_settings->gifProvider();
    return provider == QStringLiteral("giphy") ? QStringLiteral("GIPHY") : provider == QStringLiteral("klipy") ? QStringLiteral("KLIPY") : QString();
}

bool GifCatalog::configured() const
{
    return !providerName().isEmpty() && !m_settings->apiKey(m_settings->gifProvider()).isEmpty();
}

bool GifCatalog::mediaURL(const QUrl &url) const
{
    if (url.scheme() != QStringLiteral("https") || !url.userInfo().isEmpty() || url.port(443) != 443 || url.toString().size() > 8192) return false;
    const auto host = url.host().toLower();
    if (providerName() == QStringLiteral("GIPHY"))
        return host == QStringLiteral("media.giphy.com") || host == QStringLiteral("i.giphy.com") || host == QStringLiteral("media0.giphy.com")
            || host == QStringLiteral("media1.giphy.com") || host == QStringLiteral("media2.giphy.com") || host == QStringLiteral("media3.giphy.com") || host == QStringLiteral("media4.giphy.com");
    return providerName() == QStringLiteral("KLIPY") && (host == QStringLiteral("static.klipy.com") || host == QStringLiteral("static1.klipy.com") || host == QStringLiteral("static2.klipy.com"));
}

QNetworkReply *GifCatalog::fetch(const QUrl &url, qint64 limit, bool api, Response done)
{
    QNetworkRequest request(url);
    request.setTransferTimeout(api ? 15000 : 30000);
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::UserVerifiedRedirectPolicy);
    request.setAttribute(QNetworkRequest::CacheLoadControlAttribute, QNetworkRequest::AlwaysNetwork);
    request.setAttribute(QNetworkRequest::CookieLoadControlAttribute, QNetworkRequest::Manual);
    request.setAttribute(QNetworkRequest::CookieSaveControlAttribute, QNetworkRequest::Manual);
    auto *reply = m_network.get(request);
    QTimer::singleShot(api ? 20000 : 45000, reply, [reply] { if (!reply->isFinished()) reply->abort(); });
    reply->setReadBufferSize(64 * 1024);
    m_requests.append(reply);
    auto bytes = std::make_shared<QByteArray>();
    auto failed = std::make_shared<bool>(false);
    auto redirects = std::make_shared<int>(0);
    const auto consume = [reply, bytes, failed, limit] {
        if (*failed) return;
        const auto remaining = limit - bytes->size();
        bytes->append(reply->read(remaining + 1));
        if (bytes->size() > limit) { *failed = true; reply->abort(); }
    };
    connect(reply, &QNetworkReply::redirected, this, [this, reply, api, failed, redirects](const QUrl &target) {
        // API redirects could leak a key. CDN redirects must stay on a known
        // provider host; a provider response cannot fetch localhost or files.
        if (api || ++*redirects > 3 || !mediaURL(target)) { *failed = true; reply->abort(); }
        else reply->redirectAllowed();
    });
    connect(reply, &QIODevice::readyRead, this, consume);
    connect(reply, &QNetworkReply::finished, this, [this, reply, bytes, failed, consume, api, done] {
        consume();
        const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        if (api && status == 429) {
            bool numeric = false;
            qint64 delay = reply->rawHeader("Retry-After").toLongLong(&numeric);
            if (!numeric) delay = QDateTime::currentDateTimeUtc().secsTo(QDateTime::fromString(QString::fromLatin1(reply->rawHeader("Retry-After")), Qt::RFC2822Date));
            m_retryAt = QDateTime::currentSecsSinceEpoch() + qBound(qint64(1), delay > 0 ? delay : qint64(60), qint64(86400));
        }
        const bool ok = !*failed && reply->error() == QNetworkReply::NoError && status == 200;
        m_requests.removeAll(reply);
        reply->deleteLater();
        // Never log errorString(), the request URL or provider error bodies:
        // those can contain the user's API key.
        done(ok ? *bytes : QByteArray(), status);
    });
    return reply;
}

void GifCatalog::clear()
{
    ++m_generation;
    ++m_selection;
    const auto requests = m_requests;
    for (const auto &request : requests) if (request) request->abort();
    m_requests.clear();
    m_results.clear(); m_thumbnailQueue.clear(); m_thumbnails = 0;
    m_prepared.reset(); m_busy = false; m_preparing = false; m_hasMore = false;
    m_next.clear(); m_query.clear(); m_error.clear();
    emit changed();
}

void GifCatalog::search(const QString &query, bool more)
{
    if (more && (m_busy || !m_hasMore || m_results.size() >= 120)) return;
    if (!more) clear();
    if (!configured()) { m_error = tr("Choose a GIF provider and save its API key in WhatsAppGo settings."); emit changed(); return; }
    const auto remaining = m_retryAt - QDateTime::currentSecsSinceEpoch();
    if (remaining > 0) { m_error = tr("The provider is rate limited. Try again in %1 seconds.").arg(remaining); emit changed(); return; }
    if (!more) m_query = query.trimmed().left(100);
    const bool giphy = providerName() == QStringLiteral("GIPHY");
    // Both providers offer a real discovery feed. KLIPY's Tenor-compatible
    // featured endpoint shares the search response and pagination contract.
    QUrl url(giphy ? QStringLiteral("https://api.giphy.com/v1/gifs/") + (m_query.isEmpty() ? QStringLiteral("trending") : QStringLiteral("search"))
                   : QStringLiteral("https://api.klipy.com/v2/") + (m_query.isEmpty() ? QStringLiteral("featured") : QStringLiteral("search")));
    QUrlQuery params;
    params.addQueryItem(giphy ? QStringLiteral("api_key") : QStringLiteral("key"), m_settings->apiKey(m_settings->gifProvider()));
    params.addQueryItem(QStringLiteral("limit"), QStringLiteral("24"));
    if (!m_query.isEmpty()) params.addQueryItem(QStringLiteral("q"), m_query);
    if (giphy) {
        params.addQueryItem(QStringLiteral("rating"), QStringLiteral("pg"));
        params.addQueryItem(QStringLiteral("offset"), QString::number(m_results.size()));
    } else {
        params.addQueryItem(QStringLiteral("contentfilter"), QStringLiteral("high"));
        params.addQueryItem(QStringLiteral("media_filter"), QStringLiteral("preview,tinygif,mp4,tinymp4"));
        if (!m_next.isEmpty()) params.addQueryItem(QStringLiteral("pos"), m_next);
    }
    // Form-style API parsers interpret '+' as a space, unlike QUrlQuery.
    url.setQuery(params.query(QUrl::FullyEncoded).replace(QLatin1Char('+'), QStringLiteral("%2B")));
    m_error.clear(); m_busy = true; emit changed();
    const auto generation = m_generation;
    fetch(url, 2 << 20, true, [this, generation, giphy](QByteArray bytes, int status) {
        if (generation != m_generation) return;
        m_busy = false;
        QJsonParseError parse;
        const auto doc = QJsonDocument::fromJson(bytes, &parse);
        const auto obj = doc.object();
        const auto key = giphy ? QStringLiteral("data") : QStringLiteral("results");
        if (bytes.isEmpty() || parse.error != QJsonParseError::NoError || !obj.value(key).isArray()) {
            m_error = status == 401 || status == 403 ? tr("The provider rejected this API key. Check it in WhatsAppGo settings.")
                    : status == 429 ? tr("The provider is rate limited. Wait before trying again.")
                    : tr("GIF search failed. Check your connection or key, then retry.");
            emit changed(); return;
        }
        const auto items = obj.value(key).toArray();
        const auto previousNext = m_next;
        m_next = obj.value(QStringLiteral("next")).toString().left(2048);
        const int initial = m_results.size();
        for (const auto &value : items) {
            if (m_results.size() >= 120 || m_results.size() - initial >= 24) break;
            const auto item = value.toObject();
            const auto media = item.value(giphy ? QStringLiteral("images") : QStringLiteral("media_formats")).toObject();
            QString thumb, mp4;
            if (giphy) {
                thumb = media.value(QStringLiteral("fixed_width_still")).toObject().value(QStringLiteral("url")).toString();
                mp4 = media.value(QStringLiteral("original")).toObject().value(QStringLiteral("mp4")).toString();
                if (mp4.isEmpty()) mp4 = media.value(QStringLiteral("fixed_height")).toObject().value(QStringLiteral("mp4")).toString();
            } else {
                thumb = media.value(QStringLiteral("preview")).toObject().value(QStringLiteral("url")).toString();
                if (thumb.isEmpty()) thumb = media.value(QStringLiteral("tinygif")).toObject().value(QStringLiteral("url")).toString();
                mp4 = media.value(QStringLiteral("mp4")).toObject().value(QStringLiteral("url")).toString();
                if (mp4.isEmpty()) mp4 = media.value(QStringLiteral("tinymp4")).toObject().value(QStringLiteral("url")).toString();
            }
            // Preserve the provider's ordering, including an unavailable tile.
            const auto index = m_results.size();
            m_results.append(QVariantMap{{QStringLiteral("title"), item.value(QStringLiteral("title")).toString().left(300)},
                {QStringLiteral("image"), QString()}, {QStringLiteral("thumbnail"), mediaURL(QUrl(thumb)) ? thumb : QString()},
                {QStringLiteral("mp4"), mediaURL(QUrl(mp4)) ? mp4 : QString()}, {QStringLiteral("loading"), !thumb.isEmpty()}});
            m_thumbnailQueue.append(index);
        }
        const auto pagination = obj.value(QStringLiteral("pagination")).toObject();
        m_hasMore = m_results.size() < 120 && !items.isEmpty() && (giphy
            ? m_results.size() < pagination.value(QStringLiteral("total_count")).toInt(m_results.size())
            : !m_next.isEmpty() && m_next != previousNext);
        emit changed();
        loadThumbnails();
    });
}

void GifCatalog::loadThumbnails()
{
    while (m_thumbnails < 3 && !m_thumbnailQueue.isEmpty()) {
        const int index = m_thumbnailQueue.takeFirst();
        auto item = m_results.at(index).toMap();
        const auto url = QUrl(item.value(QStringLiteral("thumbnail")).toString());
        if (!mediaURL(url)) { item[QStringLiteral("loading")] = false; m_results[index] = item; emit changed(); continue; }
        ++m_thumbnails;
        const auto generation = m_generation;
        fetch(url, 2 << 20, false, [this, generation, index](QByteArray bytes, int) {
            if (generation != m_generation) return;
            --m_thumbnails;
            auto item = m_results.at(index).toMap();
            QBuffer input(&bytes); input.open(QIODevice::ReadOnly);
            QImageReader reader(&input);
            const auto size = reader.size();
            if (size.isValid() && size.width() <= 4096 && size.height() <= 4096 && qint64(size.width()) * size.height() <= 8 * 1024 * 1024) {
                reader.setScaledSize(size.scaled(240, 180, Qt::KeepAspectRatio));
                const auto image = reader.read();
                QByteArray png;
                QBuffer output(&png); output.open(QIODevice::WriteOnly);
                if (!image.isNull() && image.save(&output, "PNG"))
                    item[QStringLiteral("image")] = QStringLiteral("data:image/png;base64,") + QString::fromLatin1(png.toBase64());
            }
            item[QStringLiteral("loading")] = false; item.remove(QStringLiteral("thumbnail")); m_results[index] = item;
            emit changed(); loadThumbnails();
        });
    }
}

QString GifCatalog::preparedUrl() const
{
    return m_prepared ? QUrl::fromLocalFile(m_prepared->fileName()).toString() : QString();
}

void GifCatalog::cancelPreview()
{
    ++m_selection;
    if (m_download) m_download->abort();
    m_download.clear();
    m_prepared.reset(); m_preparing = false; m_error.clear(); emit changed();
}

void GifCatalog::prepare(int index)
{
    cancelPreview();
    if (index < 0 || index >= m_results.size()) return;
    const auto url = QUrl(m_results.at(index).toMap().value(QStringLiteral("mp4")).toString());
    if (!mediaURL(url)) { m_error = tr("This GIF has no supported MP4 rendition."); emit changed(); return; }
    const auto generation = m_generation, selection = m_selection;
    m_preparing = true; emit changed();
    m_download = fetch(url, 16 << 20, false, [this, generation, selection](QByteArray bytes, int) {
        if (generation != m_generation || selection != m_selection) return;
        m_preparing = false;
        // Reject HTML/error payloads even if the CDN incorrectly says 200.
        if (bytes.size() < 12 || bytes.mid(4, 4) != "ftyp") {
            m_error = tr("The GIF could not be downloaded as an MP4 (maximum 16 MiB). Try another result."); emit changed(); return;
        }
        auto file = std::make_shared<QTemporaryFile>(QDir::tempPath() + QStringLiteral("/whatsappgo-gif-XXXXXX.mp4"));
        if (!file->open() || file->write(bytes) != bytes.size() || !file->flush()) {
            m_error = tr("Could not prepare the GIF. Check free disk space."); emit changed(); return;
        }
        file->close(); m_prepared = file; emit changed();
    });
}

QString GifCatalog::holdPrepared()
{
    const auto url = preparedUrl();
    if (!url.isEmpty()) m_uploads.insert(url, m_prepared);
    return url;
}

void GifCatalog::release(const QString &url) { m_uploads.remove(url); }
