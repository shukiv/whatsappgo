#include "rpcclient.h"
#include "profilemonitor.h"

#include <QDir>
#include <QCoreApplication>
#include <QClipboard>
#include <QDesktopServices>
#include <QDateTime>
#include <QFileInfo>
#include <QFile>
#include <QGuiApplication>
#include <QImage>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLocalServer>
#include <QMimeData>
#include <QProcess>
#include <QRegularExpression>
#include <QSettings>
#include <QStandardPaths>
#include <QUrl>
#include <QUuid>

#include <utility>

namespace {
constexpr auto protocolVersion = 1;

QString daemonExecutable()
{
    const auto applicationDir = QCoreApplication::applicationDirPath();
    const QStringList candidates{
        qEnvironmentVariable("WHATSAPPGO_BACKEND"),
        QDir(applicationDir).filePath(QStringLiteral("whatsappd")),
        QDir(applicationDir).filePath(QStringLiteral("../../bin/whatsappd")),
        QStandardPaths::findExecutable(QStringLiteral("whatsappd")),
    };
    for (const auto &candidate : candidates) {
        if (!candidate.isEmpty() && QFileInfo(candidate).isExecutable())
            return QDir::cleanPath(candidate);
    }
    return QStringLiteral("whatsappd");
}

QString socketPathForProfile(const QString &profile)
{
    auto runtime = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (runtime.isEmpty())
        runtime = QStandardPaths::writableLocation(QStandardPaths::RuntimeLocation);
    return QDir(runtime).filePath(profile == QStringLiteral("default")
                                     ? QStringLiteral("whatsappgo/whatsappd.sock")
                                     : QStringLiteral("whatsappgo/whatsappd-%1.sock").arg(profile));
}
}

RpcClient::RpcClient(const QString &initialProfile, const QString &initialChat, QObject *parent)
    : QObject(parent)
{
    m_initialChat = initialChat;
    QSettings settings;
    const QRegularExpression validProfile(QStringLiteral("^[a-z0-9][a-z0-9_-]{0,31}$"));
    const auto savedProfiles = settings.value(QStringLiteral("accounts/profiles")).toStringList();
    if (!savedProfiles.isEmpty()) {
        m_profiles.clear();
        for (const auto &saved : savedProfiles) {
            if (validProfile.match(saved).hasMatch() && !m_profiles.contains(saved))
                m_profiles.append(saved);
        }
    }
    if (!m_profiles.contains(QStringLiteral("default")))
        m_profiles.prepend(QStringLiteral("default"));
    m_profile = settings.value(QStringLiteral("accounts/current"), QStringLiteral("default")).toString();
    if (!m_profiles.contains(m_profile)) {
        m_profile = QStringLiteral("default");
        if (!m_profiles.contains(m_profile))
            m_profiles.prepend(m_profile);
    }
    if (validProfile.match(initialProfile).hasMatch()) {
        if (!m_profiles.contains(initialProfile))
            m_profiles.append(initialProfile);
        m_profile = initialProfile;
        settings.setValue(QStringLiteral("accounts/profiles"), m_profiles);
        settings.setValue(QStringLiteral("accounts/current"), m_profile);
    }
    m_reconnectTimer.setInterval(1500);
    m_reconnectTimer.setSingleShot(true);
    connect(&m_reconnectTimer, &QTimer::timeout, this, &RpcClient::connectSocket);
    connect(&m_socket, &QLocalSocket::connected, this, [this] {
        emit daemonConnectedChanged();
        refreshStatus();
        refreshChats();
        refreshArchived();
        if (!m_initialChat.isEmpty()) {
            const auto chat = m_initialChat;
            m_initialChat.clear();
            openChat(chat, chat);
        }
    });
    connect(&m_socket, &QLocalSocket::disconnected, this, [this] {
        m_pending.clear();
        emit daemonConnectedChanged();
        if (!m_reconnectTimer.isActive())
            m_reconnectTimer.start();
    });
    connect(&m_socket, &QLocalSocket::readyRead, this, [this] {
        m_readBuffer += m_socket.readAll();
        while (true) {
            const auto newline = m_readBuffer.indexOf('\n');
            if (newline < 0)
                break;
            const auto line = m_readBuffer.left(newline);
            m_readBuffer.remove(0, newline + 1);
            if (!line.trimmed().isEmpty())
                processLine(line);
        }
    });
    connect(&m_socket, &QLocalSocket::errorOccurred, this, [this](QLocalSocket::LocalSocketError error) {
        if (error == QLocalSocket::ServerNotFoundError || error == QLocalSocket::ConnectionRefusedError)
            startBackendForProfile(m_profile);
        if (!m_reconnectTimer.isActive())
            m_reconnectTimer.start();
    });
    connect(QGuiApplication::clipboard(), &QClipboard::dataChanged, this, &RpcClient::clipboardChanged);
    if (qEnvironmentVariableIntValue("WHATSAPPGO_DISABLE_PROFILE_MONITORS") <= 0) {
        for (const auto &profile : std::as_const(m_profiles))
            ensureProfileMonitor(profile);
    }
    connectSocket();
}

RpcClient::~RpcClient()
{
    // QLocalSocket::abort() can synchronously emit disconnected. Disconnect
    // the member callbacks before member destruction begins so they cannot
    // access containers that C++ has already destroyed in reverse order.
    QObject::disconnect(&m_socket, nullptr, this, nullptr);
    QObject::disconnect(&m_reconnectTimer, nullptr, this, nullptr);
    m_reconnectTimer.stop();
    m_socket.abort();
    m_pending.clear();
    stopOwnedBackends();
}

void RpcClient::startBackendForProfile(const QString &profile)
{
    auto *running = m_ownedBackends.value(profile, nullptr);
    if (running != nullptr && running->state() != QProcess::NotRunning)
        return;
    if (running != nullptr) {
        m_ownedBackends.remove(profile);
        running->deleteLater();
    }

    const auto executable = daemonExecutable();
    if (!QFileInfo(executable).isExecutable()) {
        emit errorOccurred(tr("The bundled WhatsApp backend was not found. Rebuild with 'make desktop'."));
        return;
    }

    // A crashed process can leave its filesystem socket behind. This method is
    // called only after connecting failed, so no live listener owns the path.
    QLocalServer::removeServer(socketPathForProfile(profile));
    auto *process = new QProcess(this);
    process->setObjectName(QStringLiteral("whatsappBackend-%1").arg(profile));
    process->setProgram(executable);
    // Native notifications belong to the backend on every desktop. A tray
    // host can appear or disappear while the application is running, and
    // notification delivery must not depend on that startup-time condition.
    process->setArguments({QStringLiteral("--profile"), profile,
                           QStringLiteral("--notifications=true")});
    if (qEnvironmentVariableIntValue("WHATSAPPGO_BACKEND_LOGS") > 0) {
        process->setProcessChannelMode(QProcess::ForwardedChannels);
    } else {
        process->setStandardOutputFile(QProcess::nullDevice());
        process->setStandardErrorFile(QProcess::nullDevice());
    }
    connect(process, &QProcess::started, this, [this] {
        m_reconnectTimer.start(50);
    });
    connect(process, &QProcess::errorOccurred, this, [this, profile](QProcess::ProcessError error) {
        if (!m_shuttingDown && error == QProcess::FailedToStart)
            emit errorOccurred(tr("Could not start the bundled WhatsApp backend for account '%1'.").arg(profile));
    });
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this, process, profile](int exitCode, QProcess::ExitStatus exitStatus) {
        if (m_ownedBackends.value(profile) == process)
            m_ownedBackends.remove(profile);
        if (!m_shuttingDown && (exitStatus != QProcess::NormalExit || exitCode != 0))
            emit errorOccurred(tr("The WhatsApp backend for account '%1' stopped unexpectedly.").arg(profile));
        process->deleteLater();
    });
    m_ownedBackends.insert(profile, process);
    process->start();
}

void RpcClient::ensureProfileMonitor(const QString &profile)
{
    if (qEnvironmentVariableIntValue("WHATSAPPGO_DISABLE_PROFILE_MONITORS") > 0)
        return;
    if (m_profileMonitors.contains(profile))
        return;
    auto *monitor = new ProfileMonitor(profile, socketPathForProfile(profile), this);
    m_profileMonitors.insert(profile, monitor);
    connect(monitor, &ProfileMonitor::backendUnavailable, this, &RpcClient::startBackendForProfile);
    connect(monitor, &ProfileMonitor::countChanged, this, [this](const QString &changedProfile, int count) {
        if (m_profileUnreadCounts.value(changedProfile).toInt() == count
            && m_profileUnreadCounts.contains(changedProfile)) {
            return;
        }
        m_profileUnreadCounts.insert(changedProfile, count);
        emit profileUnreadCountsChanged();
    });
}

void RpcClient::stopOwnedBackends()
{
    m_shuttingDown = true;
    const auto processes = m_ownedBackends.values();
    for (auto *process : processes) {
        QObject::disconnect(process, nullptr, this, nullptr);
        if (process->state() != QProcess::NotRunning)
            process->terminate();
    }
    for (auto *process : processes) {
        if (process->state() != QProcess::NotRunning && !process->waitForFinished(1500)) {
            process->kill();
            process->waitForFinished(500);
        }
    }
    m_ownedBackends.clear();
}

bool RpcClient::daemonConnected() const
{
    return m_socket.state() == QLocalSocket::ConnectedState;
}

bool RpcClient::loggedIn() const
{
    return m_status.value(QStringLiteral("logged_in")).toBool();
}

bool RpcClient::clipboardHasImage() const
{
    return QGuiApplication::clipboard()->mimeData()->hasImage();
}

QString RpcClient::socketPath() const
{
    return socketPathForProfile(m_profile);
}

void RpcClient::connectSocket()
{
    if (m_socket.state() != QLocalSocket::UnconnectedState)
        return;
    m_socket.connectToServer(socketPath(), QIODevice::ReadWrite);
}

void RpcClient::reconnect()
{
    m_socket.abort();
    connectSocket();
}

void RpcClient::sendRequest(const QString &method, const QJsonObject &params, Callback callback)
{
    if (!daemonConnected()) {
        emit errorOccurred(tr("The background service is not connected yet."));
        reconnect();
        return;
    }
    const auto id = QString::number(++m_nextId);
    QJsonObject request{
        {QStringLiteral("version"), protocolVersion},
        {QStringLiteral("id"), id},
        {QStringLiteral("method"), method},
        {QStringLiteral("params"), params},
    };
    if (callback)
        m_pending.insert(id, std::move(callback));
    auto payload = QJsonDocument(request).toJson(QJsonDocument::Compact);
    payload.append('\n');
    m_socket.write(payload);
}

void RpcClient::processLine(const QByteArray &line)
{
    QJsonParseError parseError;
    const auto document = QJsonDocument::fromJson(line, &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        emit errorOccurred(tr("The daemon returned malformed data."));
        return;
    }
    const auto object = document.object();
    if (object.contains(QStringLiteral("event"))) {
        processEvent(object.value(QStringLiteral("event")).toString(), object.value(QStringLiteral("data")));
        return;
    }
    const auto id = object.value(QStringLiteral("id")).toString();
    const auto error = object.value(QStringLiteral("error")).toObject();
    if (!error.isEmpty())
        emit errorOccurred(error.value(QStringLiteral("message")).toString(tr("Request failed.")));
    if (auto it = m_pending.find(id); it != m_pending.end()) {
        const auto callback = std::move(it.value());
        m_pending.erase(it);
        callback(object.value(QStringLiteral("result")), error);
    }
}

void RpcClient::processEvent(const QString &name, const QJsonValue &data)
{
    if (name == QStringLiteral("connection.changed")) {
        m_status = data.toObject().toVariantMap();
        emit statusChanged();
    } else if (name == QStringLiteral("pairing.qr")) {
        const auto payload = data.toObject();
        m_pairingQr = QStringLiteral("data:image/png;base64,") + payload.value(QStringLiteral("png_base64")).toString();
        emit pairingQrChanged();
    } else if (name == QStringLiteral("pairing.success")) {
        m_pairingQr.clear();
        emit pairingQrChanged();
        refreshStatus();
    } else if (name == QStringLiteral("message.upsert")) {
        const auto message = data.toObject().toVariantMap();
        if (message.value(QStringLiteral("chat_jid")).toString() == QStringLiteral("status@broadcast"))
            refreshStatuses();
        if (message.value(QStringLiteral("chat_jid")) == m_selectedChat.value(QStringLiteral("jid")))
            upsertMessage(message);
        const auto cached = message.value(QStringLiteral("media_path")).toString();
        if (!cached.isEmpty())
            emit mediaReady(message.value(QStringLiteral("id")).toString(), cached);
		if (!m_pendingCopyImageId.isEmpty()
			&& message.value(QStringLiteral("id")).toString() == m_pendingCopyImageId
			&& copyImageFile(message.value(QStringLiteral("media_path")).toString()))
			m_pendingCopyImageId.clear();
        refreshChats();
    } else if (name == QStringLiteral("message.receipt")) {
        // Receipts were stored but never reached the open conversation, so a
        // sent message kept its single mark until the chat was reopened.
        const auto payload = data.toObject();
        if (payload.value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString()) {
            QStringList ids;
            const auto reported = payload.value(QStringLiteral("message_ids")).toArray();
            for (const auto &id : reported)
                ids.append(id.toString());
            m_messages.applyReceipt(ids, payload.value(QStringLiteral("status")).toString());
        }
        refreshChats();
    } else if (name == QStringLiteral("message.revoked") || name == QStringLiteral("message.reaction") || name == QStringLiteral("message.edited")) {
        if (data.toObject().value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString())
            openChat(m_selectedChat.value(QStringLiteral("jid")).toString(), m_selectedChat.value(QStringLiteral("title")).toString());
    } else if (name == QStringLiteral("message.pinned")) {
        if (data.toObject().value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString())
            refreshChatInfo();
    } else if (name == QStringLiteral("chat.updated") || name == QStringLiteral("directory.synced")) {
        refreshChats();
        refreshArchived();
    } else if (name == QStringLiteral("history.synced")) {
        refreshChats();
        refreshStatuses();
        refreshCalls();
		const auto syncedChats = data.toObject().value(QStringLiteral("chat_jids")).toArray();
		for (const auto &chat : syncedChats) {
			if (chat.toString() == m_selectedChat.value(QStringLiteral("jid")).toString()) {
				refreshOpenMessages();
				break;
			}
		}
		if (m_waitingRemoteHistory && !m_selectedChat.isEmpty())
			loadRemoteHistoryPage();
    } else if (name == QStringLiteral("call.upsert") || name == QStringLiteral("calls.synced")) {
		refreshCalls();
    } else if (name == QStringLiteral("notification.received")) {
        const auto payload = data.toObject();
        emit notificationRequested(payload.value(QStringLiteral("chat_jid")).toString(),
                                   payload.value(QStringLiteral("title")).toString(),
                                   payload.value(QStringLiteral("body")).toString());
    } else if (name == QStringLiteral("daemon.error") || name == QStringLiteral("pairing.error")) {
        emit errorOccurred(data.toObject().value(QStringLiteral("message")).toString());
    }
}

void RpcClient::refreshStatus()
{
    sendRequest(QStringLiteral("status.get"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_status = result.toObject().toVariantMap();
        emit statusChanged();
		// On first launch the QML page is often created before the daemon socket
		// connects. Start pairing after status confirms this profile is unlinked,
		// so the first QR code appears without requiring a button click.
		if (!loggedIn() && m_pairingQr.isEmpty())
			startPairing();
    });
}

void RpcClient::refreshChats(const QString &query)
{
    sendRequest(QStringLiteral("chats.list"),
                {{QStringLiteral("limit"), 200}, {QStringLiteral("offset"), 0}, {QStringLiteral("query"), query}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    m_chats = result.toArray().toVariantList();
                    const auto selectedJid = m_selectedChat.value(QStringLiteral("jid")).toString();
                    if (!selectedJid.isEmpty()) {
                        for (const auto &entry : std::as_const(m_chats)) {
                            const auto chat = entry.toMap();
                            if (chat.value(QStringLiteral("jid")).toString() != selectedJid)
                                continue;
                            const auto retainedAvatar = m_selectedChat.value(QStringLiteral("avatar_path"));
                            m_selectedChat = chat;
                            if (m_selectedChat.value(QStringLiteral("avatar_path")).toString().isEmpty()
                                && !retainedAvatar.toString().isEmpty())
                                m_selectedChat.insert(QStringLiteral("avatar_path"), retainedAvatar);
                            emit selectedChatChanged();
                            break;
                        }
                    }
                    emit chatsChanged();
                });
}

void RpcClient::refreshArchived()
{
    sendRequest(QStringLiteral("chats.list"),
                {{QStringLiteral("limit"), 200}, {QStringLiteral("offset"), 0},
                 {QStringLiteral("query"), QString()}, {QStringLiteral("archived"), true}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    m_archivedChats = result.toArray().toVariantList();
                    m_archivedCount = static_cast<int>(m_archivedChats.size());
                    emit archivedChatsChanged();
                });
}

// Pinning, muting, archiving and read state belong to the account, so each is
// asked of the daemon and the lists are refreshed from what it reports.
void RpcClient::setChatPinned(const QString &jid, bool pinned)
{
    sendRequest(QStringLiteral("chat.pin"), {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("value"), pinned}},
                [this](const QJsonValue &, const QJsonObject &) { refreshChats(); });
}

void RpcClient::setChatMuted(const QString &jid, bool muted)
{
    sendRequest(QStringLiteral("chat.mute"), {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("value"), muted}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    refreshChats();
                    if (!m_chatInfo.isEmpty())
                        refreshChatInfo();
                });
}

void RpcClient::setChatArchived(const QString &jid, bool archived)
{
    sendRequest(QStringLiteral("chat.archive"), {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("value"), archived}},
                [this](const QJsonValue &, const QJsonObject &) { refreshChats(); refreshArchived(); });
}

void RpcClient::setChatRead(const QString &jid, bool read)
{
    sendRequest(QStringLiteral("chat.set_read"), {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("value"), read}},
                [this](const QJsonValue &, const QJsonObject &) { refreshChats(); });
}

void RpcClient::openChat(const QString &jid, const QString &title)
{
	m_waitingRemoteHistory = false;
    m_mediaQueue.clear();
    m_requestedMedia.clear();
    clearChatInfo();
    m_selectedChat = {{QStringLiteral("jid"), jid}, {QStringLiteral("title"), title}};
    m_messages.clear();
    m_hasMore = false;
    m_nextBefore = 0;
    emit selectedChatChanged();
	refreshChatInfo();
	if (!m_refreshedHistoryChats.contains(jid)) {
		m_refreshedHistoryChats.insert(jid);
		sendRequest(QStringLiteral("history.refresh"),
			{{QStringLiteral("chat_jid"), jid}, {QStringLiteral("limit"), 100}});
	}
    sendRequest(QStringLiteral("messages.list"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("before"), 0}, {QStringLiteral("limit"), 50}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    const auto page = result.toObject();
                    m_messages.reset(page.value(QStringLiteral("messages")).toArray().toVariantList());
                    m_hasMore = page.value(QStringLiteral("has_more")).toBool();
                    m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
					if (!m_messages.isEmpty() && !m_hasMore)
						requestRemoteHistory();
                    QHash<QString, QJsonArray> unreadBySender;
                    QHash<QString, qint64> latestBySender;
                    const auto loaded = m_messages.items();
                    for (const auto &entry : loaded) {
                        const auto message = entry.toMap();
                        if (message.value(QStringLiteral("from_me")).toBool())
                            continue;
                        const auto sender = message.value(QStringLiteral("sender_jid")).toString();
                        unreadBySender[sender].append(message.value(QStringLiteral("id")).toString());
                        latestBySender[sender] = qMax(latestBySender.value(sender), message.value(QStringLiteral("timestamp")).toLongLong());
                    }
                    for (auto it = unreadBySender.cbegin(); it != unreadBySender.cend(); ++it) {
                        sendRequest(QStringLiteral("chat.read"),
                                    {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                                     {QStringLiteral("sender_jid"), it.key()}, {QStringLiteral("message_ids"), it.value()},
                                     {QStringLiteral("timestamp"), latestBySender.value(it.key())}});
                    }
                    if (unreadBySender.isEmpty())
                        sendRequest(QStringLiteral("chat.read"), {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()}});
                });
    sendRequest(QStringLiteral("chat.avatar"), {{QStringLiteral("chat_jid"), jid}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    const auto path = result.toObject().value(QStringLiteral("path")).toString();
                    if (!path.isEmpty()) {
                        m_selectedChat.insert(QStringLiteral("avatar_path"), path);
                        emit selectedChatChanged();
                        refreshChats();
                    }
                });
}

void RpcClient::closeChat()
{
	m_waitingRemoteHistory = false;
    clearComposerLinkPreview();
    m_selectedChat.clear();
    clearChatInfo();
    m_searchResults.clear();
    m_messages.clear();
    emit selectedChatChanged();
    emit searchResultsChanged();
}

void RpcClient::refreshChatInfo()
{
    if (m_selectedChat.isEmpty())
        return;
    const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
    sendRequest(QStringLiteral("chat.info"), {{QStringLiteral("chat_jid"), chatJid}},
                [this, chatJid](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
                        return;
                    m_chatInfo = result.toObject().toVariantMap();
                    emit chatInfoChanged();
                });
}

void RpcClient::refreshSharedContent(const QString &category, bool append)
{
    if (m_selectedChat.isEmpty())
        return;
    const auto normalized = category.isEmpty() ? QStringLiteral("media") : category;
    const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
    const int offset = append && normalized == m_sharedContentCategory ? m_sharedContent.size() : 0;
    if (!append || normalized != m_sharedContentCategory) {
        m_sharedContent.clear();
        m_sharedContentHasMore = false;
    }
    m_sharedContentCategory = normalized;
    m_sharedContentLoading = true;
    emit sharedContentChanged();
    sendRequest(QStringLiteral("chat.shared"),
                {{QStringLiteral("chat_jid"), chatJid}, {QStringLiteral("category"), normalized},
                 {QStringLiteral("offset"), offset}, {QStringLiteral("limit"), 60}},
                [this, chatJid, normalized, append, offset](const QJsonValue &result, const QJsonObject &error) {
                    if (m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid
                        || m_sharedContentCategory != normalized)
                        return;
                    m_sharedContentLoading = false;
                    if (!error.isEmpty()) {
                        emit sharedContentChanged();
                        return;
                    }
                    const auto page = result.toObject();
                    const auto items = page.value(QStringLiteral("messages")).toArray().toVariantList();
                    if (append && offset > 0 && normalized == m_sharedContentCategory)
                        m_sharedContent += items;
                    else
                        m_sharedContent = items;
                    m_sharedContentCategory = normalized;
                    m_sharedContentHasMore = page.value(QStringLiteral("has_more")).toBool();
                    emit sharedContentChanged();
                });
}

void RpcClient::clearChatInfo()
{
    const bool hadInfo = !m_chatInfo.isEmpty();
    const bool hadContent = !m_sharedContent.isEmpty() || !m_sharedContentCategory.isEmpty()
        || m_sharedContentHasMore || m_sharedContentLoading;
    m_chatInfo.clear();
    m_sharedContent.clear();
    m_sharedContentCategory.clear();
    m_sharedContentHasMore = false;
    m_sharedContentLoading = false;
    if (hadInfo)
        emit chatInfoChanged();
    if (hadContent)
        emit sharedContentChanged();
}

void RpcClient::loadOlderMessages()
{
    if (m_loadingOlder || m_selectedChat.isEmpty())
        return;
	if (!m_hasMore) {
		requestRemoteHistory();
		return;
	}
    m_loadingOlder = true;
    sendRequest(QStringLiteral("messages.list"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("before"), m_nextBefore}, {QStringLiteral("limit"), 50}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    m_loadingOlder = false;
                    if (!error.isEmpty())
                        return;
                    const auto page = result.toObject();
                    m_messages.prepend(page.value(QStringLiteral("messages")).toArray().toVariantList());
                    m_hasMore = page.value(QStringLiteral("has_more")).toBool();
                    m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
					if (!m_hasMore)
						requestRemoteHistory();
                });
}

void RpcClient::requestRemoteHistory()
{
	if (m_waitingRemoteHistory || m_messages.isEmpty() || m_selectedChat.isEmpty())
		return;
	const auto oldest = m_messages.oldest();
	const auto boundary = m_selectedChat.value(QStringLiteral("jid")).toString()
		+ QStringLiteral(":") + oldest.value(QStringLiteral("id")).toString();
	if (m_requestedHistoryBoundaries.contains(boundary))
		return;
	m_requestedHistoryBoundaries.insert(boundary);
	m_waitingRemoteHistory = true;
	sendRequest(QStringLiteral("history.request"),
		{{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
		 {QStringLiteral("limit"), 50}},
		[this, boundary](const QJsonValue &, const QJsonObject &error) {
			if (!error.isEmpty()) {
				m_requestedHistoryBoundaries.remove(boundary);
				m_waitingRemoteHistory = false;
			}
		});
}

void RpcClient::loadRemoteHistoryPage()
{
	if (m_loadingOlder || m_selectedChat.isEmpty())
		return;
	m_loadingOlder = true;
	const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
	sendRequest(QStringLiteral("messages.list"),
		{{QStringLiteral("chat_jid"), chatJid}, {QStringLiteral("before"), m_nextBefore}, {QStringLiteral("limit"), 50}},
		[this, chatJid](const QJsonValue &result, const QJsonObject &error) {
			m_loadingOlder = false;
			m_waitingRemoteHistory = false;
			if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
				return;
			const auto page = result.toObject();
			m_messages.prepend(page.value(QStringLiteral("messages")).toArray().toVariantList());
			m_hasMore = page.value(QStringLiteral("has_more")).toBool();
			m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
		});
}

void RpcClient::refreshOpenMessages()
{
	if (m_selectedChat.isEmpty())
		return;
	const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
	const auto limit = qMax(50, m_messages.rowCount());
	sendRequest(QStringLiteral("messages.list"),
		{{QStringLiteral("chat_jid"), chatJid}, {QStringLiteral("before"), 0}, {QStringLiteral("limit"), limit}},
		[this, chatJid](const QJsonValue &result, const QJsonObject &error) {
			if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
				return;
			const auto page = result.toObject();
			m_messages.reset(page.value(QStringLiteral("messages")).toArray().toVariantList());
			m_hasMore = page.value(QStringLiteral("has_more")).toBool();
			m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
		});
}

void RpcClient::sendMessage(const QString &text, const QString &replyTo)
{
    if (text.trimmed().isEmpty() || m_selectedChat.isEmpty())
        return;
    setBusy(true);
    QJsonObject params{
        {QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
        {QStringLiteral("text"), text},
        {QStringLiteral("reply_to"), replyTo},
    };
    const auto previewURL = m_composerLinkPreview.value(QStringLiteral("url")).toString();
    if (!previewURL.isEmpty() && text.contains(previewURL)) {
        auto wirePreview = m_composerLinkPreview;
        wirePreview.remove(QStringLiteral("thumbnail_source"));
        params.insert(QStringLiteral("link_preview"), QJsonObject::fromVariantMap(wirePreview));
    }
    sendRequest(QStringLiteral("message.send"), params,
                [this](const QJsonValue &, const QJsonObject &error) {
                    setBusy(false);
                    if (error.isEmpty()) {
                        clearComposerLinkPreview();
                        emit messageSent();
                    }
                });
}

void RpcClient::sendStatusReply(const QString &recipientJid, const QString &statusMessageId, const QString &text)
{
    const auto reply = text.trimmed();
    if (recipientJid.isEmpty() || statusMessageId.isEmpty() || reply.isEmpty()) {
        emit statusReplyFinished(recipientJid, statusMessageId, false, tr("This status cannot be replied to."));
        return;
    }
    if (!daemonConnected()) {
        emit statusReplyFinished(recipientJid, statusMessageId, false, tr("The background service is not connected yet."));
        reconnect();
        return;
    }
    sendRequest(QStringLiteral("message.send"), {
        {QStringLiteral("chat_jid"), recipientJid},
        {QStringLiteral("text"), reply},
        {QStringLiteral("reply_to"), statusMessageId},
        {QStringLiteral("reply_chat_jid"), QStringLiteral("status@broadcast")},
    }, [this, recipientJid, statusMessageId](const QJsonValue &, const QJsonObject &error) {
        if (error.isEmpty()) {
            emit statusReplyFinished(recipientJid, statusMessageId, true, tr("Reply sent"));
            return;
        }
        emit statusReplyFinished(recipientJid, statusMessageId, false,
                                 error.value(QStringLiteral("message")).toString(tr("Could not send the reply.")));
    });
}

void RpcClient::requestLinkPreview(const QString &text)
{
    static const QRegularExpression urlPattern(QStringLiteral("https?://[^\\s<>\\\"']+"),
                                                QRegularExpression::CaseInsensitiveOption);
    const auto match = urlPattern.match(text);
    if (!match.hasMatch()) {
        clearComposerLinkPreview();
        return;
    }
    const auto requestText = text;
    m_linkPreviewRequestText = requestText;
    sendRequest(QStringLiteral("link.preview"), {{QStringLiteral("text"), text}},
                [this, requestText](const QJsonValue &result, const QJsonObject &error) {
                    if (requestText != m_linkPreviewRequestText)
                        return;
                    QVariantMap preview;
                    if (error.isEmpty())
                        preview = result.toObject().toVariantMap();
                    const auto thumbnail = preview.value(QStringLiteral("thumbnail")).toString();
                    if (!thumbnail.isEmpty()) {
                        const auto mime = preview.value(QStringLiteral("thumbnail_mime"), QStringLiteral("image/jpeg")).toString();
                        preview.insert(QStringLiteral("thumbnail_source"),
                                       QStringLiteral("data:%1;base64,%2").arg(mime, thumbnail));
                    }
                    if (preview == m_composerLinkPreview)
                        return;
                    m_composerLinkPreview = preview;
                    emit composerLinkPreviewChanged();
                });
}

void RpcClient::clearComposerLinkPreview()
{
    m_linkPreviewRequestText.clear();
    if (m_composerLinkPreview.isEmpty())
        return;
    m_composerLinkPreview.clear();
    emit composerLinkPreviewChanged();
}

void RpcClient::sendFile(const QString &localUrl, const QString &caption)
{
    if (m_selectedChat.isEmpty())
        return;
    const auto path = QUrl(localUrl).toLocalFile();
    if (path.isEmpty())
        return;
    setBusy(true);
    sendRequest(QStringLiteral("message.send_media"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("path"), path}, {QStringLiteral("caption"), caption}},
                [this](const QJsonValue &, const QJsonObject &) { setBusy(false); });
}

void RpcClient::sendVoice(const QString &localUrl)
{
    if (m_selectedChat.isEmpty())
        return;
    const auto path = QUrl(localUrl).toLocalFile();
    if (path.isEmpty())
        return;
    setBusy(true);
    sendRequest(QStringLiteral("message.send_media"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("path"), path}, {QStringLiteral("voice"), true}},
                [this](const QJsonValue &, const QJsonObject &) { setBusy(false); });
}

void RpcClient::editMessage(const QString &messageId, const QString &text)
{
    if (m_selectedChat.isEmpty() || text.trimmed().isEmpty())
        return;
    sendRequest(QStringLiteral("message.edit"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("text"), text}});
}

void RpcClient::deleteMessage(const QString &messageId, const QString &senderJid)
{
    if (m_selectedChat.isEmpty())
        return;
    sendRequest(QStringLiteral("message.delete"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("sender_jid"), senderJid}});
}

void RpcClient::reactMessage(const QString &messageId, const QString &senderJid, const QString &reaction)
{
    if (m_selectedChat.isEmpty())
        return;
    sendRequest(QStringLiteral("message.react"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("sender_jid"), senderJid},
                 {QStringLiteral("emoji"), reaction}});
}

void RpcClient::pinMessage(const QString &messageId, const QString &senderJid, int durationSeconds)
{
    if (m_selectedChat.isEmpty())
        return;
    sendRequest(QStringLiteral("message.pin"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("sender_jid"), senderJid},
                 {QStringLiteral("duration_seconds"), durationSeconds}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (error.isEmpty())
                        refreshChatInfo();
                });
}

void RpcClient::unpinMessage(const QString &messageId, const QString &senderJid)
{
    if (m_selectedChat.isEmpty())
        return;
    sendRequest(QStringLiteral("message.unpin"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("sender_jid"), senderJid}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (error.isEmpty())
                        refreshChatInfo();
                });
}

int RpcClient::messageIndex(const QString &messageId) const
{
    const auto messages = m_messages.items();
    for (int index = 0; index < messages.size(); ++index) {
        if (messages.at(index).toMap().value(QStringLiteral("id")).toString() == messageId)
            return index;
    }
    return -1;
}

void RpcClient::startChat(const QString &phone)
{
    setBusy(true);
    sendRequest(QStringLiteral("contact.resolve"), {{QStringLiteral("phone"), phone}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    setBusy(false);
                    if (!error.isEmpty())
                        return;
                    const auto chat = result.toObject();
                    refreshChats();
                    openChat(chat.value(QStringLiteral("jid")).toString(), chat.value(QStringLiteral("title")).toString());
                });
}

void RpcClient::setTyping(bool typing)
{
    if (!m_selectedChat.isEmpty())
        sendRequest(QStringLiteral("chat.typing"), {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()}, {QStringLiteral("typing"), typing}});
}

void RpcClient::startPairing()
{
    m_pairingCode.clear();
	m_requestedHistoryBoundaries.clear();
	m_waitingRemoteHistory = false;
    emit pairingCodeChanged();
    sendRequest(QStringLiteral("pairing.start"), {});
}

void RpcClient::pairPhone(const QString &phone)
{
    setBusy(true);
    sendRequest(QStringLiteral("pairing.phone"), {{QStringLiteral("phone"), phone}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    setBusy(false);
                    if (!error.isEmpty())
                        return;
                    m_pairingCode = result.toObject().value(QStringLiteral("code")).toString();
                    emit pairingCodeChanged();
                });
}

void RpcClient::logout()
{
    sendRequest(QStringLiteral("account.logout"), {}, [this](const QJsonValue &, const QJsonObject &error) {
        if (error.isEmpty()) {
            closeChat();
            refreshStatus();
        }
    });
}

void RpcClient::switchProfile(const QString &profile)
{
    if (profile == m_profile || !m_profiles.contains(profile))
        return;
    m_reconnectTimer.stop();
    m_socket.abort();
    m_pending.clear();
    m_profile = profile;
    m_status.clear();
    m_chats.clear();
    m_messages.clear();
    m_selectedChat.clear();
    m_searchResults.clear();
    m_statusUpdates.clear();
    m_requestedStatusAvatars.clear();
    m_requestedStatusMedia.clear();
    m_callLogs.clear();
    m_channels.clear();
    m_communities.clear();
	m_refreshedHistoryChats.clear();
    m_pairingQr.clear();
    m_pairingCode.clear();
	clearComposerLinkPreview();
    QSettings().setValue(QStringLiteral("accounts/current"), m_profile);
    emit profileChanged();
    emit statusChanged();
    emit chatsChanged();
    emit selectedChatChanged();
    emit searchResultsChanged();
    emit statusUpdatesChanged();
    emit callLogsChanged();
    emit channelsChanged();
    emit communitiesChanged();
    emit pairingQrChanged();
    emit pairingCodeChanged();
    connectSocket();
}

void RpcClient::addProfile(const QString &name)
{
    auto slug = name.trimmed().toLower();
    slug.replace(QRegularExpression(QStringLiteral("[^a-z0-9_-]+")), QStringLiteral("-"));
    slug.remove(QRegularExpression(QStringLiteral("^[^a-z0-9]+")));
    if (slug.size() > 32)
        slug.truncate(32);
    if (slug.isEmpty())
        slug = QStringLiteral("account");
    const auto base = slug;
    int suffix = 2;
    while (m_profiles.contains(slug)) {
        const auto suffixText = QStringLiteral("-%1").arg(suffix++);
        slug = base.left(32 - suffixText.size()) + suffixText;
    }
    m_profiles.append(slug);
    QSettings settings;
    settings.setValue(QStringLiteral("accounts/profiles"), m_profiles);
    ensureProfileMonitor(slug);
    emit profilesChanged();
    switchProfile(slug);
}

void RpcClient::searchMessages(const QString &query)
{
    if (query.trimmed().isEmpty()) {
        m_searchResults.clear();
        emit searchResultsChanged();
        return;
    }
    sendRequest(QStringLiteral("messages.search"), {{QStringLiteral("query"),query},{QStringLiteral("limit"),100}},
                [this](const QJsonValue &result,const QJsonObject &error){if(!error.isEmpty())return;m_searchResults=result.toArray().toVariantList();emit searchResultsChanged();});
}

void RpcClient::openFile(const QString &path)
{
    if (!path.isEmpty())
        QDesktopServices::openUrl(QUrl::fromLocalFile(path));
}

// ensureMedia fetches media that arrived without a preview. History
// synchronisation strips the small picture WhatsApp normally embeds in a
// message, so old photos can only be shown by downloading them. Requests are
// limited to a few at a time: a conversation can hold thousands of pictures,
// and the view asks for every one it shows.
void RpcClient::ensureMedia(const QString &messageId)
{
    if (messageId.isEmpty() || m_selectedChat.isEmpty() || !daemonConnected())
        return;
    if (m_requestedMedia.contains(messageId))
        return;
    m_requestedMedia.insert(messageId);
    m_mediaQueue.append(messageId);
    pumpMediaQueue();
}

void RpcClient::pumpMediaQueue()
{
    constexpr int concurrentDownloads = 3;
    while (m_mediaInFlight < concurrentDownloads && !m_mediaQueue.isEmpty()) {
        const auto messageId = m_mediaQueue.takeFirst();
        const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
        if (chatJid.isEmpty())
            return;
        ++m_mediaInFlight;
        sendRequest(QStringLiteral("message.download"),
            {{QStringLiteral("chat_jid"), chatJid}, {QStringLiteral("message_id"), messageId}},
            [this](const QJsonValue &result, const QJsonObject &error) {
                --m_mediaInFlight;
                if (error.isEmpty()) {
                    const auto message = result.toObject().toVariantMap();
                    upsertMessage(message);
                }
                pumpMediaQueue();
            });
    }
}

QVariantMap RpcClient::nextAudioAfter(const QString &messageId) const
{
    return ::nextAudioAfter(m_messages.items(), messageId);
}

void RpcClient::downloadMedia(const QString &messageId)
{
	if (m_selectedChat.isEmpty() || messageId.isEmpty())
		return;
	sendRequest(QStringLiteral("message.download"),
		{{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
		 {QStringLiteral("message_id"), messageId}},
		[this](const QJsonValue &result, const QJsonObject &error) {
			if (!error.isEmpty())
				return;
			const auto message = result.toObject().toVariantMap();
			upsertMessage(message);
			// Downloading only makes the file available. What happens next is
			// the caller's decision: audio and video play inside the window,
			// documents are opened with their desktop application.
			const auto path = message.value(QStringLiteral("media_path")).toString();
			if (!path.isEmpty())
				emit mediaReady(message.value(QStringLiteral("id")).toString(), path);
		});
}

QString RpcClient::clipboardDirectory() const
{
	return QDir(QStandardPaths::writableLocation(QStandardPaths::CacheLocation)).filePath(QStringLiteral("clipboard"));
}

bool RpcClient::isClipboardFile(const QString &path) const
{
	const auto directory = QDir(clipboardDirectory()).absolutePath() + QDir::separator();
	return QFileInfo(path).absoluteFilePath().startsWith(directory);
}

QString RpcClient::prepareClipboardImage()
{
	const auto image = QGuiApplication::clipboard()->image();
	if (image.isNull()) {
		emit errorOccurred(tr("The clipboard does not contain an image."));
		return {};
	}
	const auto directory = clipboardDirectory();
	if (!QDir().mkpath(directory)) {
		emit errorOccurred(tr("Could not prepare the clipboard image."));
		return {};
	}
	const auto path = QDir(directory).filePath(QStringLiteral("paste-%1.png").arg(QUuid::createUuid().toString(QUuid::WithoutBraces)));
	if (!image.save(path, "PNG")) {
		emit errorOccurred(tr("Could not save the clipboard image."));
		return {};
	}
	return QUrl::fromLocalFile(path).toString();
}

void RpcClient::sendClipboardImage(const QString &localUrl, const QString &caption)
{
	if (m_selectedChat.isEmpty())
		return;
	const auto path = QUrl(localUrl).toLocalFile();
	if (path.isEmpty() || !isClipboardFile(path)) {
		emit errorOccurred(tr("The pasted image is no longer available."));
		return;
	}
	setBusy(true);
	sendRequest(QStringLiteral("message.send_media"),
		{{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
		 {QStringLiteral("path"), path}, {QStringLiteral("caption"), caption}},
		[this, path](const QJsonValue &, const QJsonObject &error) {
			setBusy(false);
			QFile::remove(path);
			if (error.isEmpty())
				emit noticeOccurred(tr("Image sent"));
		});
}

void RpcClient::discardClipboardImage(const QString &localUrl)
{
	const auto path = QUrl(localUrl).toLocalFile();
	if (!path.isEmpty() && isClipboardFile(path))
		QFile::remove(path);
}

bool RpcClient::copyImageFile(const QString &path)
{
	if (path.isEmpty())
		return false;
	QImage image(path);
	if (image.isNull()) {
		emit errorOccurred(tr("Could not read this image."));
		return false;
	}
	QGuiApplication::clipboard()->setImage(image);
	emit noticeOccurred(tr("Image copied"));
	return true;
}

void RpcClient::copyImage(const QString &messageId, const QString &path)
{
	if (copyImageFile(path))
		return;
	if (messageId.isEmpty() || m_selectedChat.isEmpty())
		return;
	m_pendingCopyImageId = messageId;
	sendRequest(QStringLiteral("message.download"),
		{{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
		 {QStringLiteral("message_id"), messageId}},
		[this, messageId](const QJsonValue &result, const QJsonObject &error) {
			if (!error.isEmpty()) {
				m_pendingCopyImageId.clear();
				return;
			}
			const auto message = result.toObject().toVariantMap();
			upsertMessage(message);
			if (messageId == m_pendingCopyImageId && copyImageFile(message.value(QStringLiteral("media_path")).toString()))
				m_pendingCopyImageId.clear();
		});
}

void RpcClient::copyText(const QString &text)
{
	if (text.isEmpty())
		return;
	QGuiApplication::clipboard()->setText(text);
	emit noticeOccurred(tr("Text copied"));
}

void RpcClient::refreshStatuses()
{
    sendRequest(QStringLiteral("statuses.list"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_statusUpdates = result.toArray().toVariantList();
        emit statusUpdatesChanged();
    });
}

void RpcClient::refreshChatAvatar(const QString &jid)
{
    if (jid.isEmpty() || m_pendingChatAvatars.contains(jid))
        return;
    const auto now = QDateTime::currentMSecsSinceEpoch();
    // Model refreshes can recreate every visible delegate. Keep visibility-
    // driven refreshes cheap while still revisiting avatars during a session.
    constexpr qint64 refreshCooldownMs = 30 * 1000;
    if (now - m_chatAvatarRequestedAt.value(jid, 0) < refreshCooldownMs)
        return;
    m_chatAvatarRequestedAt.insert(jid, now);
    m_pendingChatAvatars.insert(jid);
    sendRequest(QStringLiteral("chat.avatar"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("refresh"), true}},
                [this, jid](const QJsonValue &, const QJsonObject &) {
                    m_pendingChatAvatars.remove(jid);
                    refreshChats();
                });
}

void RpcClient::fetchStatusAvatar(const QString &jid)
{
    if (jid.isEmpty() || m_requestedStatusAvatars.contains(jid))
        return;
    m_requestedStatusAvatars.insert(jid);
    sendRequest(QStringLiteral("chat.avatar"), {{QStringLiteral("chat_jid"), jid}},
                [this](const QJsonValue &, const QJsonObject &) {
                    refreshStatuses();
                });
}

void RpcClient::ensureStatusMedia(const QString &messageId)
{
    if (messageId.isEmpty() || m_requestedStatusMedia.contains(messageId))
        return;
    m_requestedStatusMedia.insert(messageId);
    sendRequest(QStringLiteral("message.download"),
                {{QStringLiteral("chat_jid"), QStringLiteral("status@broadcast")},
                 {QStringLiteral("message_id"), messageId}},
                [this](const QJsonValue &, const QJsonObject &) {
                    refreshStatuses();
                });
}

void RpcClient::refreshCalls()
{
    sendRequest(QStringLiteral("calls.list"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_callLogs = result.toArray().toVariantList();
        emit callLogsChanged();
    });
}

void RpcClient::refreshChannels()
{
    sendRequest(QStringLiteral("channels.list"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_channels = result.toArray().toVariantList();
        emit channelsChanged();
    });
}

void RpcClient::refreshCommunities()
{
    sendRequest(QStringLiteral("communities.list"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_communities = result.toArray().toVariantList();
        emit communitiesChanged();
    });
}

void RpcClient::setBusy(bool value)
{
    if (m_busy == value)
        return;
    m_busy = value;
    emit busyChanged();
}

void RpcClient::upsertMessage(const QVariantMap &message)
{
    m_messages.upsert(message);
}
