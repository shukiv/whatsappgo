#include "rpcclient.h"

#include <QDir>
#include <QCoreApplication>
#include <QClipboard>
#include <QDesktopServices>
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
            startBackendForCurrentProfile();
        if (!m_reconnectTimer.isActive())
            m_reconnectTimer.start();
    });
    connect(QGuiApplication::clipboard(), &QClipboard::dataChanged, this, &RpcClient::clipboardChanged);
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

void RpcClient::startBackendForCurrentProfile()
{
    auto *running = m_ownedBackends.value(m_profile, nullptr);
    if (running != nullptr && running->state() != QProcess::NotRunning)
        return;
    if (running != nullptr) {
        m_ownedBackends.remove(m_profile);
        running->deleteLater();
    }

    const auto executable = daemonExecutable();
    if (!QFileInfo(executable).isExecutable()) {
        emit errorOccurred(tr("The bundled WhatsApp backend was not found. Rebuild with 'make desktop'."));
        return;
    }

    // A crashed process can leave its filesystem socket behind. This method is
    // called only after connecting failed, so no live listener owns the path.
    QLocalServer::removeServer(socketPath());
    const auto profile = m_profile;
    auto *process = new QProcess(this);
    process->setObjectName(QStringLiteral("whatsappBackend-%1").arg(profile));
    process->setProgram(executable);
    process->setArguments({QStringLiteral("--profile"), profile});
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
    const auto runtime = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (!runtime.isEmpty())
        return QDir(runtime).filePath(m_profile == QStringLiteral("default") ? QStringLiteral("whatsappgo/whatsappd.sock") : QStringLiteral("whatsappgo/whatsappd-%1.sock").arg(m_profile));
    return QDir(QStandardPaths::writableLocation(QStandardPaths::RuntimeLocation))
        .filePath(m_profile == QStringLiteral("default") ? QStringLiteral("whatsappgo/whatsappd.sock") : QStringLiteral("whatsappgo/whatsappd-%1.sock").arg(m_profile));
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
		if (!m_pendingCopyImageId.isEmpty()
			&& message.value(QStringLiteral("id")).toString() == m_pendingCopyImageId
			&& copyImageFile(message.value(QStringLiteral("media_path")).toString()))
			m_pendingCopyImageId.clear();
        refreshChats();
    } else if (name == QStringLiteral("message.revoked") || name == QStringLiteral("message.reaction") || name == QStringLiteral("message.edited")) {
        if (data.toObject().value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString())
            openChat(m_selectedChat.value(QStringLiteral("jid")).toString(), m_selectedChat.value(QStringLiteral("title")).toString());
    } else if (name == QStringLiteral("chat.updated") || name == QStringLiteral("directory.synced")) {
        refreshChats();
    } else if (name == QStringLiteral("history.synced")) {
        refreshChats();
        refreshStatuses();
        refreshCalls();
		if (m_waitingRemoteHistory && !m_selectedChat.isEmpty())
			loadRemoteHistoryPage();
	} else if (name == QStringLiteral("call.upsert") || name == QStringLiteral("calls.synced")) {
		refreshCalls();
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
                            m_selectedChat.insert(QStringLiteral("title"), chat.value(QStringLiteral("title")));
                            const auto avatar = chat.value(QStringLiteral("avatar_path"));
                            if (!avatar.toString().isEmpty())
                                m_selectedChat.insert(QStringLiteral("avatar_path"), avatar);
                            emit selectedChatChanged();
                            break;
                        }
                    }
                    emit chatsChanged();
                });
}

void RpcClient::openChat(const QString &jid, const QString &title)
{
	m_waitingRemoteHistory = false;
    m_selectedChat = {{QStringLiteral("jid"), jid}, {QStringLiteral("title"), title}};
    m_messages.clear();
    m_hasMore = false;
    m_nextBefore = 0;
    emit selectedChatChanged();
    emit messagesChanged();
    sendRequest(QStringLiteral("messages.list"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("before"), 0}, {QStringLiteral("limit"), 50}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    const auto page = result.toObject();
                    m_messages = page.value(QStringLiteral("messages")).toArray().toVariantList();
                    m_hasMore = page.value(QStringLiteral("has_more")).toBool();
                    m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
                    emit messagesChanged();
					if (!m_messages.isEmpty() && !m_hasMore)
						requestRemoteHistory();
                    QHash<QString, QJsonArray> unreadBySender;
                    QHash<QString, qint64> latestBySender;
                    for (const auto &entry : std::as_const(m_messages)) {
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
    m_selectedChat.clear();
    m_searchResults.clear();
    m_messages.clear();
    emit selectedChatChanged();
    emit searchResultsChanged();
    emit messagesChanged();
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
                    auto older = page.value(QStringLiteral("messages")).toArray().toVariantList();
                    older.append(m_messages);
                    m_messages = older;
                    m_hasMore = page.value(QStringLiteral("has_more")).toBool();
                    m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
                    emit messagesChanged();
					if (!m_hasMore)
						requestRemoteHistory();
                });
}

void RpcClient::requestRemoteHistory()
{
	if (m_waitingRemoteHistory || m_messages.isEmpty() || m_selectedChat.isEmpty())
		return;
	const auto oldest = m_messages.constFirst().toMap();
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
			auto older = page.value(QStringLiteral("messages")).toArray().toVariantList();
			older.append(m_messages);
			m_messages = older;
			m_hasMore = page.value(QStringLiteral("has_more")).toBool();
			m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
			emit messagesChanged();
		});
}

void RpcClient::sendMessage(const QString &text, const QString &replyTo)
{
    if (text.trimmed().isEmpty() || m_selectedChat.isEmpty())
        return;
    setBusy(true);
    sendRequest(QStringLiteral("message.send"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("text"), text}, {QStringLiteral("reply_to"), replyTo}},
                [this](const QJsonValue &, const QJsonObject &) { setBusy(false); emit messageSent(); });
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
    m_callLogs.clear();
    m_channels.clear();
    m_communities.clear();
    m_pairingQr.clear();
    m_pairingCode.clear();
    QSettings().setValue(QStringLiteral("accounts/current"), m_profile);
    emit profileChanged();
    emit statusChanged();
    emit chatsChanged();
    emit messagesChanged();
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
			const auto path = message.value(QStringLiteral("media_path")).toString();
			if (!path.isEmpty())
				openFile(path);
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

void RpcClient::refreshStatuses()
{
    sendRequest(QStringLiteral("statuses.list"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_statusUpdates = result.toArray().toVariantList();
        emit statusUpdatesChanged();
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
    const auto id = message.value(QStringLiteral("id"));
    for (qsizetype i = 0; i < m_messages.size(); ++i) {
        if (m_messages.at(i).toMap().value(QStringLiteral("id")) == id) {
            m_messages[i] = message;
            emit messagesChanged();
            return;
        }
    }
    m_messages.append(message);
    emit messagesChanged();
}
