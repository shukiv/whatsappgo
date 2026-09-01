#include "profilemonitor.h"

#include <QJsonDocument>
#include <QJsonObject>

ProfileMonitor::ProfileMonitor(const QString &profile, const QString &socketPath, QObject *parent)
    : QObject(parent)
    , m_profile(profile)
    , m_socketPath(socketPath)
{
    m_reconnectTimer.setInterval(1500);
    m_reconnectTimer.setSingleShot(true);
    m_refreshTimer.setInterval(40);
    m_refreshTimer.setSingleShot(true);

    connect(&m_reconnectTimer, &QTimer::timeout, this, &ProfileMonitor::connectSocket);
    connect(&m_refreshTimer, &QTimer::timeout, this, &ProfileMonitor::refresh);
    connect(&m_socket, &QLocalSocket::connected, this, &ProfileMonitor::refresh);
    connect(&m_socket, &QLocalSocket::disconnected, this, [this] {
        m_pendingId.clear();
        m_refreshAgain = false;
        if (!m_reconnectTimer.isActive())
            m_reconnectTimer.start();
    });
    connect(&m_socket, &QLocalSocket::errorOccurred, this, [this](QLocalSocket::LocalSocketError error) {
        if (error == QLocalSocket::ServerNotFoundError || error == QLocalSocket::ConnectionRefusedError)
            emit backendUnavailable(m_profile);
        if (!m_reconnectTimer.isActive())
            m_reconnectTimer.start();
    });
    connect(&m_socket, &QLocalSocket::readyRead, this, [this] {
        m_buffer += m_socket.readAll();
        while (true) {
            const auto newline = m_buffer.indexOf('\n');
            if (newline < 0)
                break;
            const auto line = m_buffer.left(newline);
            m_buffer.remove(0, newline + 1);
            if (!line.trimmed().isEmpty())
                processLine(line);
        }
    });
    // Defer the first attempt until the owner has connected backendUnavailable;
    // QLocalSocket may report a missing server synchronously.
    QTimer::singleShot(0, this, &ProfileMonitor::connectSocket);
}

void ProfileMonitor::connectSocket()
{
    if (m_socket.state() == QLocalSocket::UnconnectedState)
        m_socket.connectToServer(m_socketPath, QIODevice::ReadWrite);
}

void ProfileMonitor::refresh()
{
    if (m_socket.state() != QLocalSocket::ConnectedState)
        return;
    if (!m_pendingId.isEmpty()) {
        m_refreshAgain = true;
        return;
    }
    m_pendingId = QStringLiteral("unread-%1").arg(++m_requestId);
    const QJsonObject request{{QStringLiteral("version"), 1},
                              {QStringLiteral("id"), m_pendingId},
                              {QStringLiteral("method"), QStringLiteral("chats.unread_count")},
                              {QStringLiteral("params"), QJsonObject{}}};
    m_socket.write(QJsonDocument(request).toJson(QJsonDocument::Compact) + '\n');
}

void ProfileMonitor::processLine(const QByteArray &line)
{
    const auto document = QJsonDocument::fromJson(line);
    if (!document.isObject())
        return;
    const auto object = document.object();
    if (object.contains(QStringLiteral("event"))) {
        const auto event = object.value(QStringLiteral("event")).toString();
        if (event == QStringLiteral("message.upsert") || event == QStringLiteral("chat.updated")
            || event == QStringLiteral("history.synced") || event == QStringLiteral("directory.synced")) {
            if (m_pendingId.isEmpty())
                m_refreshTimer.start();
            else
                m_refreshAgain = true;
        }
        return;
    }
    if (object.value(QStringLiteral("id")).toString() != m_pendingId)
        return;
    m_pendingId.clear();
    if (m_refreshAgain) {
        m_refreshAgain = false;
        m_refreshTimer.start();
    }
    if (!object.value(QStringLiteral("error")).toObject().isEmpty())
        return;
    const auto next = object.value(QStringLiteral("result")).toObject().value(QStringLiteral("count")).toInt();
    if (next != m_count) {
        m_count = next;
        emit countChanged(m_profile, m_count);
    }
}
