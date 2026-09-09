#pragma once

#include <QObject>
#include <QVariantMap>
#include <QtQml/qqmlregistration.h>

// Desktop-wide integration preferences, deliberately independent of profiles
// and RPC. Credentials never enter the WhatsApp store or diagnostic context.
class AppSettings : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON
    Q_PROPERTY(QString gifProvider READ gifProvider NOTIFY changed)
    Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
    explicit AppSettings(QObject *parent = nullptr);
    QString gifProvider() const { return m_provider; }
    QString error() const { return m_error; }
    Q_INVOKABLE QString apiKey(const QString &provider) const;
    Q_INVOKABLE bool saveGifProviders(const QString &provider, const QVariantMap &keys);

signals:
    void changed();
    void errorChanged();

private:
    bool fail(const QString &message);
    QString m_path;
    QString m_provider = QStringLiteral("none");
    QString m_error;
    QVariantMap m_keys;
};
