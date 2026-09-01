#pragma once

#include <QByteArray>
#include <QLocalSocket>
#include <QObject>
#include <QTimer>

// A lightweight second RPC connection keeps an account's unread total live
// even while another account is selected in the main window.
class ProfileMonitor final : public QObject
{
    Q_OBJECT

public:
    explicit ProfileMonitor(const QString &profile, const QString &socketPath, QObject *parent = nullptr);
    int count() const { return m_count; }

signals:
    void countChanged(const QString &profile, int count);
    void backendUnavailable(const QString &profile);

private:
    void connectSocket();
    void refresh();
    void processLine(const QByteArray &line);

    QString m_profile;
    QString m_socketPath;
    QLocalSocket m_socket;
    QTimer m_reconnectTimer;
    QTimer m_refreshTimer;
    QByteArray m_buffer;
    quint64 m_requestId = 0;
    QString m_pendingId;
    bool m_refreshAgain = false;
    int m_count = -1;
};
