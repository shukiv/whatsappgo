#include "appsettings.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonObject>
#include <QSaveFile>
#include <QStandardPaths>

namespace {
const QStringList providers{QStringLiteral("giphy"), QStringLiteral("tenor"), QStringLiteral("klipy")};
bool activeProvider(const QString &provider)
{
    // Google retired Tenor's API on 2026-06-30. Retain a legacy key without
    // advertising an API that can no longer be used for requests.
    return provider == QStringLiteral("none") || provider == QStringLiteral("giphy")
        || provider == QStringLiteral("klipy");
}
bool validKey(const QString &key)
{
    if (key.size() > 2048)
        return false;
    for (const QChar c : key) {
        if (c.isSpace() || c.category() == QChar::Other_Control)
            return false;
    }
    return true;
}
}

AppSettings::AppSettings(QObject *parent) : QObject(parent)
{
    const QString config = QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation);
    if (config.isEmpty()) {
        fail(tr("The local settings folder is unavailable."));
        return;
    }
    m_path = config + QStringLiteral("/private/gif-providers.json");
    QFile file(m_path);
    if (!file.exists())
        return;
    if (!file.open(QIODevice::ReadOnly) || file.size() > 16384) {
        fail(tr("Could not read GIF provider settings. Saving will replace the stored configuration."));
        return;
    }
    QJsonParseError parseError;
    const auto document = QJsonDocument::fromJson(file.readAll(), &parseError);
    const auto object = document.object();
    const QString provider = object.value(QStringLiteral("provider")).toString();
    if (parseError.error != QJsonParseError::NoError || !document.isObject() || !activeProvider(provider)
        || !object.value(QStringLiteral("keys")).isObject()) {
        fail(tr("GIF provider settings are invalid. Saving will replace the stored configuration."));
        return;
    }
    const auto keys = object.value(QStringLiteral("keys")).toObject();
    for (const auto &name : providers) {
        const auto key = keys.value(name);
        if (!key.isString() || !validKey(key.toString())) {
            fail(tr("GIF provider settings are invalid. Saving will replace the stored configuration."));
            return;
        }
    }
    m_provider = provider;
    for (const auto &name : providers)
        m_keys.insert(name, keys.value(name).toString());
}

QString AppSettings::apiKey(const QString &provider) const
{
    return m_keys.value(provider).toString();
}

bool AppSettings::fail(const QString &message)
{
    m_error = message;
    emit errorChanged();
    return false;
}

bool AppSettings::saveGifProviders(const QString &provider, const QVariantMap &keys)
{
    if (!activeProvider(provider))
        return fail(tr("Choose GIPHY, KLIPY, or None. Tenor's API has been retired."));
    QVariantMap cleanKeys;
    for (const auto &name : providers) {
        const QString key = keys.value(name).toString().trimmed();
        if (!validKey(key))
            return fail(tr("API keys must be at most 2048 characters, without spaces or line breaks."));
        cleanKeys.insert(name, key);
    }
    if (provider != QStringLiteral("none") && cleanKeys.value(provider).toString().isEmpty())
        return fail(tr("Enter an API key for the selected GIF provider, or choose None."));
    if (m_path.isEmpty())
        return fail(tr("The local settings folder is unavailable."));

    const QString directory = QFileInfo(m_path).absolutePath();
    if (!QDir().mkpath(directory))
        return fail(tr("Could not create the local settings folder."));
    const auto filePermissions = QFileDevice::ReadOwner | QFileDevice::WriteOwner;
#ifdef Q_OS_UNIX
    if (!QFile::setPermissions(directory, filePermissions | QFileDevice::ExeOwner))
        return fail(tr("Could not protect the local settings folder."));
#endif
    QSaveFile file(m_path);
    if (!file.open(QIODevice::WriteOnly))
        return fail(tr("Could not save GIF provider settings."));
    if (!file.setPermissions(filePermissions))
        return fail(tr("Could not protect the saved API keys."));
    const QByteArray data = QJsonDocument(QJsonObject{
        {QStringLiteral("provider"), provider},
        {QStringLiteral("keys"), QJsonObject::fromVariantMap(cleanKeys)}
    }).toJson();
    if (file.write(data) != data.size() || !file.commit())
        return fail(tr("Could not save GIF provider settings. Your previous settings have not changed."));
    m_provider = provider;
    m_keys = cleanKeys;
    m_error.clear();
    emit errorChanged();
    emit changed();
    return true;
}
