#include "rpcclient.h"

#ifdef Q_OS_LINUX
#include <sys/prctl.h>
#include <unistd.h>
#include <csignal>
#include <cstdlib>
#endif
#ifdef Q_OS_WIN
// windows.h defines min and max as macros and drags in most of the Win32 API;
// both break Qt headers compiled after it in the same translation unit.
#define NOMINMAX
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#endif
#include "profilemonitor.h"

#include <QDir>
#include <QCoreApplication>
#include <QClipboard>
#include <QDesktopServices>
#include "updateinstaller.h"
#include <QDateTime>
#include <QFileInfo>
#include <QFile>
#include <QGuiApplication>
#include <QImage>
#include <QImageReader>
#include <QTransform>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLocalServer>
#include <QMimeData>
#include <QProcess>
#include <QRegularExpression>
#include <QSaveFile>
#include <QSettings>
#include <QStandardPaths>
#include <QUrl>
#include <QUuid>

#include <utility>

namespace {
constexpr auto protocolVersion = 1;

QString daemonName()
{
#ifdef Q_OS_WIN
    return QStringLiteral("whatsappd.exe");
#else
    return QStringLiteral("whatsappd");
#endif
}

QString daemonExecutable()
{
    const auto applicationDir = QCoreApplication::applicationDirPath();
    const auto name = daemonName();
    const QStringList candidates{
        qEnvironmentVariable("WHATSAPPGO_BACKEND"),
        QDir(applicationDir).filePath(name),
        QDir(applicationDir).filePath(QStringLiteral("../../bin/") + name),
        QStandardPaths::findExecutable(QStringLiteral("whatsappd")),
    };
    for (const auto &candidate : candidates) {
        if (!candidate.isEmpty() && QFileInfo(candidate).isExecutable())
            return QDir::cleanPath(candidate);
    }
    return name;
}

// The next four helpers mirror internal/config/paths.go and must keep mirroring
// it: the daemon and this client have to agree on every directory, or the
// client shows an account whose data the daemon writes somewhere else. The XDG
// variables win on every platform, exactly as they do in Go, because that is
// how the tests and sandboxed runs redirect a whole profile.
#ifdef Q_OS_WIN
// One job for the whole client. Every daemon joins it, and Windows kills them
// all when the last handle to the job closes - which happens when this process
// exits, however it exits.
void assignToShutdownJob(qint64 pid)
{
    static HANDLE job = [] {
        HANDLE created = CreateJobObjectW(nullptr, nullptr);
        if (created == nullptr)
            return created;
        JOBOBJECT_EXTENDED_LIMIT_INFORMATION limits{};
        limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
        if (!SetInformationJobObject(created, JobObjectExtendedLimitInformation, &limits, sizeof(limits))) {
            CloseHandle(created);
            return HANDLE(nullptr);
        }
        return created;
    }();
    if (job == nullptr || pid <= 0)
        return;
    HANDLE child = OpenProcess(PROCESS_SET_QUOTA | PROCESS_TERMINATE, FALSE, DWORD(pid));
    if (child == nullptr)
        return;
    AssignProcessToJobObject(job, child);
    CloseHandle(child);
}
#endif

QString dataBaseDir()
{
    const auto configured = qEnvironmentVariable("XDG_DATA_HOME");
    if (!configured.isEmpty())
        return configured;
#if defined(Q_OS_WIN)
    return qEnvironmentVariable("APPDATA");
#elif defined(Q_OS_MACOS)
    return QDir::home().filePath(QStringLiteral("Library/Application Support"));
#else
    return QDir::home().filePath(QStringLiteral(".local/share"));
#endif
}

QString cacheBaseDir()
{
    const auto configured = qEnvironmentVariable("XDG_CACHE_HOME");
    if (!configured.isEmpty())
        return configured;
#if defined(Q_OS_WIN)
    return qEnvironmentVariable("LOCALAPPDATA");
#elif defined(Q_OS_MACOS)
    return QDir::home().filePath(QStringLiteral("Library/Caches"));
#else
    return QDir::home().filePath(QStringLiteral(".cache"));
#endif
}

// The default profile lives at the roots of those directories rather than under
// profiles/, which is why it is never removable: deleting it would take every
// other account's data with it.
QString profileDataDir(const QString &profile)
{
    return QDir(dataBaseDir()).filePath(QStringLiteral("whatsappgo/profiles/%1").arg(profile));
}

QString profileCacheDir(const QString &profile)
{
    return QDir(cacheBaseDir()).filePath(QStringLiteral("whatsappgo/profiles/%1").arg(profile));
}

#ifdef Q_OS_WIN
// Mirrors config.PipeUserSegment in internal/config/pipeuser_windows.go. The
// two must produce the same string or this client dials a pipe name nobody is
// listening on.
QString pipeUserSegment()
{
    const auto account = qEnvironmentVariable("USERNAME").toLower();
    QString segment;
    for (const auto character : account) {
        const auto latin = character.toLatin1();
        const bool legal = (latin >= 'a' && latin <= 'z') || (latin >= '0' && latin <= '9')
            || latin == '-' || latin == '_';
        segment += legal ? QChar::fromLatin1(latin) : QLatin1Char('_');
        if (segment.size() >= 32)
            break;
    }
    return segment.isEmpty() ? QStringLiteral("user") : segment;
}
#endif


QString bareJid(QString jid)
{
    jid.replace(QRegularExpression(QStringLiteral(":\\d+@")), QStringLiteral("@"));
    return jid;
}

QString localFilePath(const QString &pathOrUrl)
{
    const QUrl url(pathOrUrl);
    return url.isLocalFile() ? url.toLocalFile() : pathOrUrl;
}
}

RpcClient::RpcClient(const QString &initialProfile, const QString &initialChat, QObject *parent)
    : QObject(parent)
{
    m_initialChat = initialChat;
    sweepClipboardDirectory(clipboardDirectory(), 24 * 60 * 60);
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
    settings.beginGroup(QStringLiteral("accounts/displayNames"));
    for (const auto &profile : std::as_const(m_profiles)) {
        const auto displayName = settings.value(profile).toString().trimmed();
        if (!displayName.isEmpty())
            m_profileDisplayNames.insert(profile, displayName);
    }
    settings.endGroup();
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
    // Chat-list refreshes arrive in bursts, and each one would otherwise
    // replay the live query. One replay per burst is enough and keeps the
    // request queue clear for the search the reader is waiting on.
    m_searchReplayTimer.setInterval(300);
    m_searchReplayTimer.setSingleShot(true);
    connect(&m_searchReplayTimer, &QTimer::timeout, this, [this] {
        if (!m_chatQuery.trimmed().isEmpty())
            runSidebarSearch(m_chatQuery);
    });
    connect(&m_socket, &QLocalSocket::connected, this, [this] {
        emit daemonConnectedChanged();
        refreshStatus();
        refreshChats();
        refreshArchived();
        refreshUpdateStatus();
        // Anything that arrived while the connection was down is missing from
        // the open conversation, and the presence subscription went with the
        // socket. Reopening the chat by hand used to be the only way back.
        if (!m_selectedChat.isEmpty()) {
            const auto jid = m_selectedChat.value(QStringLiteral("jid")).toString();
            if (!jid.isEmpty() && !jid.endsWith(QStringLiteral("@g.us"))
                    && !jid.endsWith(QStringLiteral("@broadcast")))
                sendRequest(QStringLiteral("contact.presence.subscribe"), {{QStringLiteral("chat_jid"), jid}},
                            {}, OnFailure::StayQuiet);
            refreshOpenMessages();
            refreshChatInfo();
        }
        if (!m_initialChat.isEmpty()) {
            const auto chat = m_initialChat;
            m_initialChat.clear();
            openChat(chat, chat);
        }
    });
    connect(&m_socket, &QLocalSocket::disconnected, this, [this] {
        abandonPendingRequests(tr("The background service disconnected."), true);
        if (!m_selectedPresence.isEmpty()) {
            m_selectedPresence.clear();
            emit selectedPresenceChanged();
        }
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

QString RpcClient::socketPathForProfile(const QString &profile)
{
#ifdef Q_OS_WIN
    // Windows has no filesystem socket here: QLocalSocket is a named pipe, and
    // QLocalServer builds \\.\pipe\<name> from the bare name below. The
    // daemon listens on the same name; see internal/config/socket_windows.go.
    auto name = QStringLiteral("whatsappgo-") + pipeUserSegment();
    if (profile != QStringLiteral("default"))
        name += QLatin1Char('-') + profile;
    return name;
#else
    auto runtime = qEnvironmentVariable("XDG_RUNTIME_DIR");
    if (runtime.isEmpty()) {
#if defined(Q_OS_MACOS)
        // Named outright rather than taken from QStandardPaths::RuntimeLocation,
        // because config.runtimeBaseDir in internal/config/paths_darwin.go names
        // it outright too. If the two ever disagreed the client would dial a
        // socket the daemon is not listening on, and nothing here would say so.
        runtime = QDir::home().filePath(QStringLiteral("Library/Application Support"));
#else
        runtime = QStandardPaths::writableLocation(QStandardPaths::RuntimeLocation);
#endif
    }
    return QDir(runtime).filePath(profile == QStringLiteral("default")
                                     ? QStringLiteral("whatsappgo/whatsappd.sock")
                                     : QStringLiteral("whatsappgo/whatsappd-%1.sock").arg(profile));
#endif
}

bool RpcClient::backendIsListening(const QString &socketPath)
{
    QLocalSocket probe;
    probe.connectToServer(socketPath, QIODevice::ReadOnly);
    if (!probe.waitForConnected(250))
        return false;
    probe.abort();
    return true;
}

void RpcClient::startBackendForProfile(const QString &profile)
{
    // A removed account must never get its daemon back. Its monitor and socket
    // both keep reporting the missing server for a moment after removal, and
    // answering them would recreate the data that was just deleted.
    if (!m_profiles.contains(profile))
        return;
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

    // A crashed process can leave its socket behind, but a refused connection
    // does not prove the daemon is gone: a full listen backlog or a busy
    // accept loop refuses too. Removing the socket in that case orphans a live
    // daemon, which keeps running on an unlinked path that nothing can reach,
    // and every retry then leaks another process. Probe first and only clear a
    // path that answers nobody. Both callers restart a reconnect timer, so
    // returning here simply retries against the daemon that is already there.
    const auto socketPath = socketPathForProfile(profile);
    if (backendIsListening(socketPath))
        return;
    QLocalServer::removeServer(socketPath);
    auto *process = new QProcess(this);
    process->setObjectName(QStringLiteral("whatsappBackend-%1").arg(profile));
    process->setProgram(executable);
    // Native notifications belong to the backend on every desktop. A tray
    // host can appear or disappear while the application is running, and
    // notification delivery must not depend on that startup-time condition.
    // stopOwnedBackends only runs on a clean exit. A client that is killed or
    // that crashes used to leave its daemons running - reparented to init,
    // holding the account databases, connected to WhatsApp, with no window
    // left to stop them from. Each of the three mechanisms below covers that
    // on one platform; the flag is the portable one.
    process->setArguments({QStringLiteral("--profile"), profile,
                           QStringLiteral("--notifications=true"),
                           QStringLiteral("--exit-with-parent")});
#ifdef Q_OS_LINUX
    // Immediate rather than within the daemon's polling interval, and it
    // survives a SIGKILL of the client.
    process->setChildProcessModifier([] {
        prctl(PR_SET_PDEATHSIG, SIGTERM);
        // The client can already be gone by the time this runs, in which case
        // the signal above will never arrive.
        if (getppid() == 1)
            _exit(EXIT_FAILURE);
    });
#endif
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
#ifdef Q_OS_WIN
    // Windows does not reparent an orphan, so the daemon cannot notice that
    // the client has gone. A job object that kills its members when the last
    // handle closes does it from the other side: the handle is this process's,
    // so it closes however the client dies.
    if (process->waitForStarted(2000))
        assignToShutdownJob(process->processId());
#endif
}

void RpcClient::ensureProfileMonitor(const QString &profile)
{
    if (qEnvironmentVariableIntValue("WHATSAPPGO_DISABLE_PROFILE_MONITORS") > 0)
        return;
    if (!m_profiles.contains(profile))
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

// A request that is dropped still has to answer its caller. Sending was left
// disabled after a disconnection because only the send's own callback clears
// the busy flag, and automatic downloads stopped for good after an account
// switch because the count of transfers in flight is only decremented by the
// callbacks that were thrown away.
void RpcClient::abandonPendingRequests(const QString &reason, bool announce)
{
    const auto abandoned = std::exchange(m_pending, {});
    const auto quiet = std::exchange(m_quietRequests, {});
    m_mediaQueue.clear();
    m_mediaInFlight = 0;
    const QJsonObject error{{QStringLiteral("code"), QStringLiteral("disconnected")},
                            {QStringLiteral("message"), reason}};
    bool anybodyWasWaiting = false;
    for (auto it = abandoned.cbegin(); it != abandoned.cend(); ++it) {
        if (!quiet.contains(it.key()))
            anybodyWasWaiting = true;
        if (it.value())
            it.value()(QJsonValue(), error);
    }
    // One line, however many requests were dropped: a daemon restart abandons
    // a handful at once and nobody needs to be told about each of them.
    if (announce && anybodyWasWaiting)
        emit errorOccurred(reason);
}

void RpcClient::sendRequest(const QString &method, const QJsonObject &params, Callback callback,
                            OnFailure onFailure)
{
    if (!daemonConnected()) {
        if (onFailure == OnFailure::Report) {
            emit errorOccurred(tr("The background service is not connected yet."));
        }
        // The caller is waiting on this answer as much as on a delivered one:
        // a send that is refused here used to leave the composer disabled.
        if (callback) {
            callback(QJsonValue(), QJsonObject{{QStringLiteral("code"), QStringLiteral("not_connected")},
                                               {QStringLiteral("message"), tr("The background service is not connected yet.")}});
        }
        reconnect();
        return;
    }
    const auto id = QString::number(++m_nextId);
    if (onFailure == OnFailure::StayQuiet) {
        m_quietRequests.insert(id);
    }
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
    // Work the window started on its own - fetching a picture, resolving a
    // link - fails all the time on a network that blinks. Its failure belongs
    // in the caller that asked for it, not across the bottom of the screen.
    const bool quiet = m_quietRequests.remove(id);
    if (!error.isEmpty() && !quiet)
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
    } else if (name == QStringLiteral("contact.presence")) {
        const auto payload = data.toObject();
        if (bareJid(payload.value(QStringLiteral("jid")).toString())
                != bareJid(m_selectedChat.value(QStringLiteral("jid")).toString()))
            return;
        const auto chatState = m_selectedPresence.value(QStringLiteral("chat_state"));
        const auto chatMedia = m_selectedPresence.value(QStringLiteral("media"));
        m_selectedPresence = payload.toVariantMap();
        m_selectedPresence.insert(QStringLiteral("state"),
                                  payload.value(QStringLiteral("unavailable")).toBool()
                                      ? QStringLiteral("offline") : QStringLiteral("online"));
        if (!chatState.toString().isEmpty()) {
            m_selectedPresence.insert(QStringLiteral("chat_state"), chatState);
            m_selectedPresence.insert(QStringLiteral("media"), chatMedia);
        }
        emit selectedPresenceChanged();
    } else if (name == QStringLiteral("chat.presence")) {
        const auto payload = data.toObject();
        if (bareJid(payload.value(QStringLiteral("chat_jid")).toString())
                != bareJid(m_selectedChat.value(QStringLiteral("jid")).toString()))
            return;
        const auto state = payload.value(QStringLiteral("state")).toString();
        if (state == QStringLiteral("paused")) {
            m_selectedPresence.remove(QStringLiteral("chat_state"));
            m_selectedPresence.remove(QStringLiteral("media"));
            m_selectedPresence.remove(QStringLiteral("sender_jid"));
        } else {
            m_selectedPresence.insert(QStringLiteral("chat_state"), state);
            m_selectedPresence.insert(QStringLiteral("media"), payload.value(QStringLiteral("media")).toString());
            m_selectedPresence.insert(QStringLiteral("sender_jid"), payload.value(QStringLiteral("sender_jid")).toString());
        }
        emit selectedPresenceChanged();
    } else if (name == QStringLiteral("pairing.qr")) {
        const auto payload = data.toObject();
        m_pairingQr = QStringLiteral("data:image/png;base64,") + payload.value(QStringLiteral("png_base64")).toString();
        emit pairingQrChanged();
    } else if (name == QStringLiteral("update.available")) {
        m_updateStatus = data.toObject().toVariantMap();
        emit updateStatusChanged();
        emit updateAvailable(m_updateStatus.value(QStringLiteral("latest")).toString());
    } else if (name == QStringLiteral("update.progress")) {
        const auto payload = data.toObject();
        emit updateProgress(payload.value(QStringLiteral("received")).toVariant().toLongLong(),
                            payload.value(QStringLiteral("total")).toVariant().toLongLong());
    } else if (name == QStringLiteral("update.ready")) {
        const auto payload = data.toObject();
        m_updateStatus.insert(QStringLiteral("downloaded"), payload.value(QStringLiteral("path")).toString());
        m_updateStatus.insert(QStringLiteral("downloading"), false);
        emit updateStatusChanged();
        emit updateReady(payload.value(QStringLiteral("path")).toString(),
                         payload.value(QStringLiteral("version")).toString());
    } else if (name == QStringLiteral("update.failed")) {
        const auto message = data.toObject().value(QStringLiteral("error")).toString();
        m_updateStatus.insert(QStringLiteral("downloading"), false);
        m_updateStatus.insert(QStringLiteral("error"), message);
        emit updateStatusChanged();
        emit updateFailed(message);
    } else if (name == QStringLiteral("pairing.success")) {
        m_pairingQr.clear();
        emit pairingQrChanged();
        refreshStatus();
    } else if (name == QStringLiteral("message.upsert")) {
        const auto message = data.toObject().toVariantMap();
        if (message.value(QStringLiteral("chat_jid")).toString() == QStringLiteral("status@broadcast"))
            refreshStatuses();
        if (message.value(QStringLiteral("chat_jid")) == m_selectedChat.value(QStringLiteral("jid"))) {
            upsertMessage(message);
            // The reader is looking at it. Only the page that opens a chat used
            // to send receipts, so a message that arrived while the
            // conversation was open stayed unread until it was reopened.
            acknowledgeIncoming(message);
        }
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
            m_messages.applyReceipt(ids, payload.value(QStringLiteral("status")).toString(),
							payload.value(QStringLiteral("timestamp")).toVariant().toLongLong());
        }
        refreshChats();
    } else if (name == QStringLiteral("message.revoked") || name == QStringLiteral("message.reaction") || name == QStringLiteral("message.edited")) {
        // Reloading the conversation for a reaction threw away every older page
        // the reader had loaded and dropped them back at the newest one.
        const auto payload = data.toObject();
        if (payload.value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString())
            refreshOneMessage(payload.value(QStringLiteral("message_id")).toString());
    } else if (name == QStringLiteral("message.starred")) {
        // A star set on the phone reaches us only as this event, so the open
        // conversation has to answer to it as well as to our own action.
        const auto payload = data.toObject();
        if (payload.value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString())
            applyStarToOpenConversation(payload.value(QStringLiteral("message_id")).toString(),
                                        payload.value(QStringLiteral("starred")).toBool());
    } else if (name == QStringLiteral("message.pinned")) {
        if (data.toObject().value(QStringLiteral("chat_jid")).toString() == m_selectedChat.value(QStringLiteral("jid")).toString())
            refreshChatInfo();
    } else if (name == QStringLiteral("chat.updated")) {
        const auto payload = data.toObject();
        const auto avatarPath = payload.value(QStringLiteral("avatar_path")).toString();
        if (!avatarPath.isEmpty()) {
            applyChatAvatar(payload.value(QStringLiteral("jid")).toString(), avatarPath);
            return;
        }
        refreshChats();
        refreshArchived();
        if (!m_chatInfo.isEmpty())
            refreshChatInfo();
    } else if (name == QStringLiteral("directory.synced")) {
        refreshChats();
        refreshArchived();
        if (!m_chatInfo.isEmpty())
            refreshChatInfo();
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
        // "handled" means the daemon already showed this through the desktop's
        // own notification service, which is what happens on Linux. Presenting
        // it again here would double every notification.
        if (payload.value(QStringLiteral("handled")).toString() != QStringLiteral("1")) {
            emit notificationRequested(payload.value(QStringLiteral("chat_jid")).toString(),
                                       payload.value(QStringLiteral("title")).toString(),
                                       payload.value(QStringLiteral("body")).toString());
        }
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

void RpcClient::refreshChats()
{
    // The unfiltered list backs the sidebar, the unread total and the new-chat
    // picker, so a search never narrows it; the query keeps its own results.
    // A refresh asks for as much as the sidebar had scrolled to, or the pages
    // a reader had loaded would be replaced by the first one every time a
    // message arrived.
    const int wanted = qBound(chatPageSize, static_cast<int>(m_chats.size()), chatListCeiling);
    sendRequest(QStringLiteral("chats.list"),
                {{QStringLiteral("limit"), wanted}, {QStringLiteral("offset"), 0}, {QStringLiteral("query"), QString()}},
                [this, wanted](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    m_chats = result.toArray().toVariantList();
                    // Whether there is another page to ask for: a full answer
                    // means there may be, a short one means this is the end.
                    m_moreChats = static_cast<int>(m_chats.size()) >= wanted;
                    m_loadingChats = false;
                    syncChatListModel();
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
    // A conversation that changed while a search is open should move in the
    // results too, so the query is replayed - coalesced, because refreshes
    // come in bursts.
    if (!m_chatQuery.trimmed().isEmpty())
        m_searchReplayTimer.start();
}

// WhatsApp Web answers a sidebar query with three groups - matching chats,
// matching contacts and matching messages - so all three are fetched together.
void RpcClient::searchChats(const QString &query)
{
    if (m_chatQuery == query)
        return;
    m_chatQuery = query;
    emit chatQueryChanged();
    m_searchReplayTimer.stop();
    // Results for the previous query would be read as answers to this one.
    m_contactSearchHits.clear();
    m_messageSearchHits.clear();
    emit contactSearchHitsChanged();
    emit messageSearchHitsChanged();
    if (query.trimmed().isEmpty()) {
        m_chatSearchHits.clear();
        emit chatSearchHitsChanged();
        return;
    }
    // The chats already loaded are matched here, so the Chats group appears on
    // the keystroke instead of after a round trip. The daemon's answer, which
    // also reaches the archived shelf, replaces this a moment later.
    const auto needle = query.trimmed().toCaseFolded();
    QVariantList local;
    for (const auto &entry : std::as_const(m_chats)) {
        const auto chat = entry.toMap();
        if (chat.value(QStringLiteral("title")).toString().toCaseFolded().contains(needle)
            || chat.value(QStringLiteral("jid")).toString().toCaseFolded().contains(needle))
            local.append(chat);
    }
    m_chatSearchHits = local;
    emit chatSearchHitsChanged();
    runSidebarSearch(query);
}

void RpcClient::runSidebarSearch(const QString &query)
{
    // Typing before the daemon is up is normal at startup. The local pass has
    // already answered from the chats in hand, and refreshChats() replays the
    // query once the connection lands, so this stays quiet rather than raising
    // three "not connected" errors per keystroke.
    if (!daemonConnected())
        return;

    // Each reply is discarded when the field has moved on: typing fires a
    // request per keystroke, and a slow early one must not win the race.
    // chats.search rather than chats.list: results span the archived shelf too,
    // which the list deliberately keeps separate.
    sendRequest(QStringLiteral("chats.search"),
                {{QStringLiteral("limit"), 50}, {QStringLiteral("query"), query}},
                [this, query](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || query != m_chatQuery)
                        return;
                    m_chatSearchHits = result.toArray().toVariantList();
                    emit chatSearchHitsChanged();
                });
    sendRequest(QStringLiteral("contacts.list"),
                {{QStringLiteral("limit"), 50}, {QStringLiteral("query"), query}},
                [this, query](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || query != m_chatQuery)
                        return;
                    m_contactSearchHits = result.toArray().toVariantList();
                    emit contactSearchHitsChanged();
                });
    // Kept apart from searchResults: the message-history dialog owns that list
    // and would otherwise overwrite the sidebar's hits, and be overwritten.
    sendRequest(QStringLiteral("messages.search"),
                {{QStringLiteral("limit"), 100}, {QStringLiteral("query"), query}},
                [this, query](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || query != m_chatQuery)
                        return;
                    m_messageSearchHits = result.toArray().toVariantList();
                    emit messageSearchHitsChanged();
                });
}

void RpcClient::setChatListFilter(const QString &filter)
{
    static const QSet<QString> allowed{
        QStringLiteral("all"), QStringLiteral("unread"),
        QStringLiteral("favorites"), QStringLiteral("groups"),
    };
    // A list filter names the list it shows, so it cannot be enumerated here.
    const bool isList = filter.startsWith(QStringLiteral("label:"));
    const auto normalized = isList || allowed.contains(filter) ? filter : QStringLiteral("all");
    if (m_chatListFilter == normalized)
        return;
    m_chatListFilter = normalized;
    syncChatListModel();
}

void RpcClient::syncChatListModel()
{
    QVariantList visible;
    visible.reserve(m_chats.size());
    for (const auto &entry : std::as_const(m_chats)) {
        const auto chat = entry.toMap();
        const bool accepted = m_chatListFilter == QStringLiteral("all")
            || (m_chatListFilter == QStringLiteral("unread")
                && chat.value(QStringLiteral("unread_count")).toInt() > 0)
            || (m_chatListFilter == QStringLiteral("favorites")
                && chat.value(QStringLiteral("favorite")).toBool())
            || (m_chatListFilter == QStringLiteral("groups")
                && chat.value(QStringLiteral("is_group")).toBool())
            || (m_chatListFilter.startsWith(QStringLiteral("label:"))
                && chat.value(QStringLiteral("label_ids")).toStringList()
                       .contains(m_chatListFilter.mid(6)));
        if (accepted)
            visible.append(chat);
    }
    m_chatList.sync(visible);
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
                    m_archivedChatList.sync(m_archivedChats);
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

void RpcClient::setChatMuted(const QString &jid, bool muted, int durationSeconds)
{
    // Zero is the menu's "Always": the daemon reads it as "until undone".
    sendRequest(QStringLiteral("chat.mute"),
                {{QStringLiteral("chat_jid"), jid},
                 {QStringLiteral("value"), muted},
                 {QStringLiteral("duration_seconds"), qMax(0, durationSeconds)}},
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

void RpcClient::rememberMessages(const QString &chatJid, const QVariantList &messages)
{
    if (chatJid.isEmpty())
        return;
    m_messageCache.insert(chatJid, messages);
    m_messageCacheOrder.removeAll(chatJid);
    m_messageCacheOrder.append(chatJid);
    while (m_messageCacheOrder.size() > 12)
        m_messageCache.remove(m_messageCacheOrder.takeFirst());
}

void RpcClient::upgradeSmallLinkPreviews(const QVariantList &messages)
{
    const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
    if (chatJid.isEmpty() || !daemonConnected())
        return;
    for (const auto &entry : messages) {
        const auto message = entry.toMap();
        const auto messageId = message.value(QStringLiteral("id")).toString();
        const auto linkURL = message.value(QStringLiteral("link_url")).toString();
        const auto thumbnail = message.value(QStringLiteral("link_thumbnail")).toString();
        if (messageId.isEmpty() || linkURL.isEmpty() || thumbnail.isEmpty())
            continue;
        QImageReader reader(thumbnail);
        const auto size = reader.size();
        if (!size.isValid() || size.width() >= 640)
            continue;
        const auto requestKey = chatJid + QStringLiteral(":") + messageId;
        if (m_requestedLinkPreviews.contains(requestKey))
            continue;
        m_requestedLinkPreviews.insert(requestKey);
        sendRequest(QStringLiteral("link.preview.refresh"),
                    {{QStringLiteral("chat_jid"), chatJid},
                     {QStringLiteral("message_id"), messageId}},
                    [this, chatJid](const QJsonValue &result, const QJsonObject &error) {
                        if (!error.isEmpty()
                            || m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
                            return;
                        const auto refreshed = result.toObject().toVariantMap();
                        if (!refreshed.isEmpty())
                            upsertMessage(refreshed);
                    },
                    OnFailure::StayQuiet);
    }
}

void RpcClient::openChat(const QString &jid, const QString &title)
{
	const auto previousJid = m_selectedChat.value(QStringLiteral("jid")).toString();
	if (!previousJid.isEmpty())
		rememberMessages(previousJid, m_messages.items());
	m_waitingRemoteHistory = false;
    m_loadingOlder = false;
    m_mediaQueue.clear();
    m_requestedMedia.clear();
    clearChatInfo();
    m_selectedChat = {{QStringLiteral("jid"), jid}, {QStringLiteral("title"), title}};
    m_selectedPresence.clear();
    if (const auto cached = m_messageCache.constFind(jid); cached != m_messageCache.cend())
        m_messages.reset(cached.value());
    else
        m_messages.clear();
    m_hasMore = false;
    m_nextBefore = 0;
    m_nextBeforeId.clear();
    emit selectedChatChanged();
    emit selectedPresenceChanged();
    if (!jid.endsWith(QStringLiteral("@g.us")) && !jid.endsWith(QStringLiteral("@broadcast")))
        sendRequest(QStringLiteral("contact.presence.subscribe"), {{QStringLiteral("chat_jid"), jid}},
                    {}, OnFailure::StayQuiet);
    refreshChatInfo();
    // The daemon serves requests sequentially per connection. Read its local
    // database before asking WhatsApp for a remote refresh so opening a chat
    // is never held behind network history synchronisation.
    sendRequest(QStringLiteral("messages.list"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("before"), 0}, {QStringLiteral("limit"), 50}},
                [this, jid](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != jid)
                        return;
                    const auto page = result.toObject();
                    const auto loadedMessages = page.value(QStringLiteral("messages")).toArray().toVariantList();
                    rememberMessages(jid, loadedMessages);
                    m_messages.reset(loadedMessages);
                    upgradeSmallLinkPreviews(loadedMessages);
                    m_hasMore = page.value(QStringLiteral("has_more")).toBool();
                    m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
                    m_nextBeforeId = page.value(QStringLiteral("next_before_id")).toString();
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
                [this, jid](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != jid)
                        return;
                    const auto path = result.toObject().value(QStringLiteral("path")).toString();
                    if (!path.isEmpty()) {
                        m_selectedChat.insert(QStringLiteral("avatar_path"), path);
                        emit selectedChatChanged();
                        refreshChats();
                    }
                },
                OnFailure::StayQuiet);
}

void RpcClient::createGroup(const QString &name, const QStringList &participants)
{
    QJsonArray members;
    for (const auto &participant : participants)
        members.append(participant);
    sendRequest(QStringLiteral("group.create"),
                {{QStringLiteral("name"), name}, {QStringLiteral("participants"), members}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChats();
                    // Open the group straight away: the user just named it and
                    // chose who is in it, so the conversation is what they want
                    // next.
                    const auto chat = result.toObject();
                    const auto jid = chat.value(QStringLiteral("jid")).toString();
                    if (!jid.isEmpty())
                        openChat(jid, chat.value(QStringLiteral("title")).toString());
                });
}

void RpcClient::markAllChatsRead()
{
    sendRequest(QStringLiteral("chats.mark_all_read"), {},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChats();
                    refreshArchived();
                });
}

void RpcClient::deleteChat(const QString &jid)
{
    sendRequest(QStringLiteral("chat.delete"), {{QStringLiteral("chat_jid"), jid}},
                [this, jid](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    // The conversation is gone, so a pane still showing it would
                    // be pointing at nothing.
                    if (m_selectedChat.value(QStringLiteral("jid")).toString() == jid)
                        closeChat();
                    refreshChats();
                    refreshArchived();
                });
}

void RpcClient::clearChat(const QString &jid)
{
    sendRequest(QStringLiteral("chat.clear"), {{QStringLiteral("chat_jid"), jid}},
                [this, jid](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    // The chat stays, so the open conversation is reloaded rather
                    // than closed: it is simply empty now.
                    if (m_selectedChat.value(QStringLiteral("jid")).toString() == jid) {
                        m_messageCache.remove(jid);
                        openChat(jid, m_selectedChat.value(QStringLiteral("title")).toString());
                    }
                    refreshChats();
                });
}

void RpcClient::setChatDisappearing(const QString &jid, int seconds)
{
    sendRequest(QStringLiteral("chat.disappearing"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("duration_seconds"), seconds}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChats();
                    refreshChatInfo();
                });
}

void RpcClient::exportChat(const QString &jid, const QString &destinationUrl)
{
    const auto path = QUrl(destinationUrl).isLocalFile()
        ? QUrl(destinationUrl).toLocalFile()
        : destinationUrl;
    if (jid.isEmpty() || path.isEmpty())
        return;
    sendRequest(QStringLiteral("chat.export"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("path"), path}},
                [this, path](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    emit noticeOccurred(tr("Chat exported to %1").arg(path));
                });
}

void RpcClient::setChatFavorite(const QString &jid, bool favorite)
{
    sendRequest(QStringLiteral("chat.favorite"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("value"), favorite}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChats();
                });
}

void RpcClient::closeChat()
{
	const auto previousJid = m_selectedChat.value(QStringLiteral("jid")).toString();
	if (!previousJid.isEmpty())
		rememberMessages(previousJid, m_messages.items());
	m_waitingRemoteHistory = false;
    m_loadingOlder = false;
    clearComposerLinkPreview();
    m_selectedChat.clear();
    m_selectedPresence.clear();
    clearChatInfo();
    m_searchResults.clear();
    m_messages.clear();
    emit selectedChatChanged();
    emit selectedPresenceChanged();
    emit searchResultsChanged();
}

// loadMoreChats asks for the conversations after the ones already listed. The
// sidebar used to stop at its first page, so an account with more conversations
// than that could not reach the rest by scrolling.
void RpcClient::loadMoreChats()
{
    if (m_loadingChats || !m_moreChats || m_chats.isEmpty())
        return;
    m_loadingChats = true;
    const int offset = static_cast<int>(m_chats.size());
    sendRequest(QStringLiteral("chats.list"),
                {{QStringLiteral("limit"), chatPageSize}, {QStringLiteral("offset"), offset}, {QStringLiteral("query"), QString()}},
                [this, offset](const QJsonValue &result, const QJsonObject &error) {
                    m_loadingChats = false;
                    if (!error.isEmpty())
                        return;
                    // Another refresh may have replaced the list while this page
                    // was on its way; appending it then would duplicate rows.
                    if (static_cast<int>(m_chats.size()) != offset)
                        return;
                    const auto page = result.toArray().toVariantList();
                    m_moreChats = page.size() >= chatPageSize;
                    if (page.isEmpty())
                        return;
                    m_chats.append(page);
                    syncChatListModel();
                    emit chatsChanged();
                });
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

// The media browser is the same content without a chat filter, so it keeps its
// own page state rather than fighting the contact drawer over one buffer.
void RpcClient::refreshChatLabels()
{
    sendRequest(QStringLiteral("labels.list"), {},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    const auto labels = result.toObject().value(QStringLiteral("labels")).toArray().toVariantList();
                    if (labels == m_chatLabels)
                        return;
                    m_chatLabels = labels;
                    emit chatLabelsChanged();
                });
}

void RpcClient::createChatLabel(const QString &name)
{
    sendRequest(QStringLiteral("label.create"), {{QStringLiteral("name"), name}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChatLabels();
                });
}

void RpcClient::setChatLabeled(const QString &jid, const QString &labelId, bool labeled)
{
    sendRequest(QStringLiteral("chat.label"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("label_id"), labelId},
                 {QStringLiteral("value"), labeled}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty())
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                });
}

void RpcClient::refreshBlockedContacts()
{
    sendRequest(QStringLiteral("contacts.blocked"), {},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    QStringList blocked;
                    const auto items = result.toObject().value(QStringLiteral("blocked")).toArray();
                    for (const auto &item : items)
                        blocked.append(item.toString());
                    if (blocked == m_blockedContacts)
                        return;
                    m_blockedContacts = blocked;
                    emit blockedContactsChanged();
                });
}

void RpcClient::refreshPrivacySettings()
{
    sendRequest(QStringLiteral("privacy.get"), {},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty())
                        return;
                    const auto settings = result.toObject().toVariantMap();
                    if (settings == m_privacySettings)
                        return;
                    m_privacySettings = settings;
                    emit privacySettingsChanged();
                });
}

void RpcClient::setPrivacySetting(const QString &name, const QString &value)
{
    sendRequest(QStringLiteral("privacy.set"),
                {{QStringLiteral("name"), name}, {QStringLiteral("value"), value}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        // WhatsApp owns these, so a rejected change is followed by
                        // a read rather than by leaving the old value on screen.
                        refreshPrivacySettings();
                        return;
                    }
                    m_privacySettings = result.toObject().toVariantMap();
                    emit privacySettingsChanged();
                });
}

void RpcClient::createChannel(const QString &name, const QString &description)
{
    if (name.trimmed().isEmpty())
        return;
    sendRequest(QStringLiteral("channel.create"),
                {{QStringLiteral("name"), name.trimmed()}, {QStringLiteral("description"), description}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    emit noticeOccurred(tr("Channel created."));
                    refreshChannels();
                });
}

void RpcClient::followChannelLink(const QString &link)
{
    if (link.trimmed().isEmpty())
        return;
    sendRequest(QStringLiteral("channel.follow_link"), {{QStringLiteral("link"), link.trimmed()}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    const auto name = result.toObject().value(QStringLiteral("name")).toString();
                    emit noticeOccurred(name.isEmpty() ? tr("Channel followed.")
                                                       : tr("Following %1.").arg(name));
                    refreshChannels();
                });
}

void RpcClient::createCommunity(const QString &name)
{
    if (name.trimmed().isEmpty())
        return;
    sendRequest(QStringLiteral("community.create"), {{QStringLiteral("name"), name.trimmed()}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    emit noticeOccurred(tr("Community created."));
                    refreshCommunities();
                });
}

void RpcClient::joinGroupLink(const QString &link)
{
    if (link.trimmed().isEmpty())
        return;
    sendRequest(QStringLiteral("group.join_link"), {{QStringLiteral("link"), link.trimmed()}},
                [this](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    const auto chat = result.toObject();
                    refreshChats();
                    // Landing in the conversation is the point of joining, so the
                    // group opens rather than only appearing in the list.
                    const auto jid = chat.value(QStringLiteral("jid")).toString();
                    if (!jid.isEmpty())
                        openChat(jid, chat.value(QStringLiteral("title")).toString());
                });
}

void RpcClient::setChannelFollowed(const QString &jid, bool followed)
{
    sendRequest(QStringLiteral("channel.follow"),
                {{QStringLiteral("jid"), jid}, {QStringLiteral("value"), followed}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChannels();
                });
}

void RpcClient::setChannelMuted(const QString &jid, bool muted)
{
    sendRequest(QStringLiteral("channel.mute"),
                {{QStringLiteral("jid"), jid}, {QStringLiteral("value"), muted}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    refreshChannels();
                });
}

void RpcClient::postTextStatus(const QString &text, int background)
{
    if (text.trimmed().isEmpty())
        return;
    sendRequest(QStringLiteral("status.post"),
                {{QStringLiteral("text"), text}, {QStringLiteral("background"), background}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    emit noticeOccurred(tr("Status posted."));
                    refreshStatuses();
                });
}

void RpcClient::postMediaStatus(const QString &localUrl, const QString &caption)
{
    const auto path = QUrl(localUrl).isLocalFile() ? QUrl(localUrl).toLocalFile() : localUrl;
    if (path.isEmpty())
        return;
    sendRequest(QStringLiteral("status.post"),
                {{QStringLiteral("path"), path}, {QStringLiteral("caption"), caption}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    emit noticeOccurred(tr("Status posted."));
                    refreshStatuses();
                });
}

void RpcClient::setAbout(const QString &text)
{
    sendRequest(QStringLiteral("profile.set_about"), {{QStringLiteral("text"), text}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    emit noticeOccurred(tr("About updated."));
                    refreshStatus();
                });
}

void RpcClient::setContactBlocked(const QString &jid, bool blocked)
{
    sendRequest(QStringLiteral("contact.block"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("value"), blocked}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        return;
                    }
                    // WhatsApp owns the list, so read it back rather than
                    // assuming the change landed the way it was asked for.
                    refreshBlockedContacts();
                });
}

void RpcClient::refreshMediaLibrary(const QString &category, bool append)
{
    const auto normalized = category.isEmpty() ? QStringLiteral("media") : category;
    const int offset = append && normalized == m_mediaLibraryCategory ? m_mediaLibrary.size() : 0;
    if (!append || normalized != m_mediaLibraryCategory) {
        m_mediaLibrary.clear();
        m_mediaLibraryHasMore = false;
    }
    m_mediaLibraryCategory = normalized;
    m_mediaLibraryLoading = true;
    emit mediaLibraryChanged();
    sendRequest(QStringLiteral("media.shared"),
                {{QStringLiteral("category"), normalized},
                 {QStringLiteral("offset"), offset}, {QStringLiteral("limit"), 60}},
                [this, normalized, offset](const QJsonValue &result, const QJsonObject &error) {
                    if (m_mediaLibraryCategory != normalized)
                        return;
                    m_mediaLibraryLoading = false;
                    if (!error.isEmpty()) {
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                        emit mediaLibraryChanged();
                        return;
                    }
                    const auto page = result.toObject();
                    const auto items = page.value(QStringLiteral("messages")).toArray().toVariantList();
                    if (offset > 0)
                        m_mediaLibrary += items;
                    else
                        m_mediaLibrary = items;
                    m_mediaLibraryHasMore = page.value(QStringLiteral("has_more")).toBool();
                    emit mediaLibraryChanged();
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
    const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
    sendRequest(QStringLiteral("messages.list"),
                {{QStringLiteral("chat_jid"), chatJid},
                 {QStringLiteral("before"), m_nextBefore}, {QStringLiteral("before_id"), m_nextBeforeId}, {QStringLiteral("limit"), 50}},
                [this, chatJid](const QJsonValue &result, const QJsonObject &error) {
                    if (m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
                        return;
                    m_loadingOlder = false;
                    if (!error.isEmpty())
                        return;
                    const auto page = result.toObject();
                    const auto loadedMessages = page.value(QStringLiteral("messages")).toArray().toVariantList();
                    m_messages.prepend(loadedMessages);
                    upgradeSmallLinkPreviews(loadedMessages);
                    m_hasMore = page.value(QStringLiteral("has_more")).toBool();
                    m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
                    m_nextBeforeId = page.value(QStringLiteral("next_before_id")).toString();
					if (!m_hasMore)
						requestRemoteHistory();
                    // Keep going until the reader has back what they had
                    // loaded before the conversation was refreshed.
                    if (m_restoreTarget > m_messages.rowCount() && m_hasMore)
                        loadOlderMessages();
                    else
                        m_restoreTarget = 0;
                });
}

bool RpcClient::canLoadOlderMessages() const
{
    return m_hasMore || m_loadingOlder || m_waitingRemoteHistory;
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
		{{QStringLiteral("chat_jid"), chatJid}, {QStringLiteral("before"), m_nextBefore}, {QStringLiteral("before_id"), m_nextBeforeId}, {QStringLiteral("limit"), 50}},
		[this, chatJid](const QJsonValue &result, const QJsonObject &error) {
			m_loadingOlder = false;
			m_waitingRemoteHistory = false;
			if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
				return;
			const auto page = result.toObject();
			const auto loadedMessages = page.value(QStringLiteral("messages")).toArray().toVariantList();
			m_messages.prepend(loadedMessages);
			upgradeSmallLinkPreviews(loadedMessages);
			m_hasMore = page.value(QStringLiteral("has_more")).toBool();
			m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
			m_nextBeforeId = page.value(QStringLiteral("next_before_id")).toString();
		});
}

void RpcClient::refreshOpenMessages()
{
	if (m_selectedChat.isEmpty())
		return;
	const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
	// The daemon serves at most 200 messages in one page. Asking for more used
	// to come back as 50, which threw away everything the reader had loaded.
	const int loaded = m_messages.rowCount();
	const auto limit = qBound(50, loaded, 200);
	m_restoreTarget = loaded;
	sendRequest(QStringLiteral("messages.list"),
		{{QStringLiteral("chat_jid"), chatJid}, {QStringLiteral("before"), 0}, {QStringLiteral("limit"), limit}},
		[this, chatJid](const QJsonValue &result, const QJsonObject &error) {
			if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != chatJid)
				return;
			const auto page = result.toObject();
			const auto loadedMessages = page.value(QStringLiteral("messages")).toArray().toVariantList();
			m_messages.reset(loadedMessages);
			upgradeSmallLinkPreviews(loadedMessages);
			m_hasMore = page.value(QStringLiteral("has_more")).toBool();
			m_nextBefore = page.value(QStringLiteral("next_before")).toVariant().toLongLong();
			m_nextBeforeId = page.value(QStringLiteral("next_before_id")).toString();
			// One page is all the daemon serves. A reader who had scrolled
			// further gets the rest back rather than being dropped at the
			// newest messages.
			if (m_restoreTarget > m_messages.rowCount() && m_hasMore)
				loadOlderMessages();
			else
				m_restoreTarget = 0;
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
                },
                OnFailure::StayQuiet);
}

void RpcClient::clearComposerLinkPreview()
{
    m_linkPreviewRequestText.clear();
    if (m_composerLinkPreview.isEmpty())
        return;
    m_composerLinkPreview.clear();
    emit composerLinkPreviewChanged();
}

void RpcClient::sendFile(const QString &localUrl, const QString &caption, const QString &replyTo, bool document)
{
    if (m_selectedChat.isEmpty())
        return;
    const auto path = QUrl(localUrl).toLocalFile();
    if (path.isEmpty())
        return;
    setBusy(true);
    // WhatsApp Web quotes a message whatever is attached to the reply, so the
    // reply the composer is showing travels with the file.
    sendRequest(QStringLiteral("message.send_media"),
                {{QStringLiteral("chat_jid"), m_selectedChat.value(QStringLiteral("jid")).toString()},
                 {QStringLiteral("path"), path}, {QStringLiteral("caption"), caption},
                 {QStringLiteral("reply_to"), replyTo}, {QStringLiteral("document"), document}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    setBusy(false);
                    if (error.isEmpty())
                        emit messageSent();
                });
}

void RpcClient::sendVoice(const QString &localUrl, const QString &chatJid, const QString &recordingProfile, const QString &replyTo)
{
    // Completion can arrive after navigation. Never infer a recipient or an
    // account from the UI that happens to be open at that point.
    if (chatJid.isEmpty() || recordingProfile != m_profile)
        return;
    const auto path = QUrl(localUrl).toLocalFile();
    if (path.isEmpty())
        return;
    setBusy(true);
    sendRequest(QStringLiteral("message.send_media"),
                {{QStringLiteral("chat_jid"), chatJid},
                 {QStringLiteral("path"), path}, {QStringLiteral("voice"), true},
                 {QStringLiteral("reply_to"), replyTo}},
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

void RpcClient::starMessage(const QString &messageId, const QString &senderJid, bool fromMe, bool starred)
{
    if (m_selectedChat.isEmpty())
        return;
    const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
    sendRequest(QStringLiteral("message.star"),
                {{QStringLiteral("chat_jid"), chatJid},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("sender_jid"), senderJid},
                 {QStringLiteral("from_me"), fromMe}, {QStringLiteral("starred"), starred}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    // The daemon reports the new state as a message.starred
                    // event, so the bubble is updated there and this callback
                    // only has to surface a refusal.
                    if (!error.isEmpty())
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
                });
}

void RpcClient::applyStarToOpenConversation(const QString &messageId, bool starred)
{
    // By identity, not by row: the model stores a conversation chronologically
    // and hands the view the reverse, so a row number from one is a different
    // message in the other. Starring the first message marked the last.
    auto message = m_messages.byId(messageId);
    if (message.isEmpty())
        return;
    message.insert(QStringLiteral("starred"), starred);
    m_messages.upsert(message);
}

void RpcClient::forwardMessage(const QString &messageId, const QString &toChatJid)
{
    if (m_selectedChat.isEmpty())
        return;
    forwardMessageFrom(m_selectedChat.value(QStringLiteral("jid")).toString(), messageId, toChatJid);
}

// The browser across every chat forwards items that are not in the open
// conversation, so the source chat travels with the message rather than being
// assumed to be the selected one.
void RpcClient::forwardMessageFrom(const QString &fromChatJid, const QString &messageId, const QString &toChatJid)
{
    if (fromChatJid.isEmpty() || messageId.isEmpty() || toChatJid.isEmpty())
        return;
    sendRequest(QStringLiteral("message.forward"),
                {{QStringLiteral("chat_jid"), fromChatJid},
                 {QStringLiteral("message_id"), messageId}, {QStringLiteral("to_chat_jid"), toChatJid}},
                [this](const QJsonValue &, const QJsonObject &error) {
                    if (!error.isEmpty())
                        emit errorOccurred(error.value(QStringLiteral("message")).toString());
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
    return m_messages.viewRowForId(messageId);
}

QVariantMap RpcClient::messageById(const QString &messageId) const
{
	for (const auto &entry : m_messages.items()) {
		const auto message = entry.toMap();
		if (message.value(QStringLiteral("id")).toString() == messageId)
			return message;
	}
	return {};
}

void RpcClient::markMediaPlayed(const QString &messageId)
{
	const auto message = messageById(messageId);
	const auto kind = message.value(QStringLiteral("kind")).toString();
	const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
	const auto key = chatJid + QLatin1Char(':') + messageId;
	if (message.isEmpty() || message.value(QStringLiteral("from_me")).toBool()
		|| (kind != QStringLiteral("audio") && kind != QStringLiteral("video"))
		|| chatJid.isEmpty() || m_playedMedia.contains(key)
		|| message.value(QStringLiteral("status")).toString() == QStringLiteral("played"))
		return;
	m_playedMedia.insert(key);
	sendRequest(QStringLiteral("message.played"), {
		{QStringLiteral("chat_jid"), chatJid},
		{QStringLiteral("sender_jid"), message.value(QStringLiteral("sender_jid")).toString()},
		{QStringLiteral("message_id"), messageId},
		{QStringLiteral("timestamp"), QDateTime::currentMSecsSinceEpoch()},
	}, [this, key](const QJsonValue &, const QJsonObject &error) {
		if (!error.isEmpty())
			m_playedMedia.remove(key);
	});
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
    abandonPendingRequests(tr("The account was changed."), false);
    m_profile = profile;
    m_status.clear();
    m_chats.clear();
    m_chatQuery.clear();
    m_chatSearchHits.clear();
    m_contactSearchHits.clear();
    m_messageSearchHits.clear();
    emit chatQueryChanged();
    emit chatSearchHitsChanged();
    emit contactSearchHitsChanged();
    emit messageSearchHitsChanged();
    m_chatList.clear();
    m_archivedChats.clear();
    m_archivedChatList.clear();
    m_archivedCount = 0;
    m_messages.clear();
    m_messageCache.clear();
    m_messageCacheOrder.clear();
    m_selectedChat.clear();
    m_searchResults.clear();
    ++m_starredRequestGeneration;
    m_starredMessages.clear();
    m_starredMessagesLoading = false;
    emit starredMessagesChanged();
    m_statusUpdates.clear();
    m_requestedLinkPreviews.clear();
    m_requestedStatusAvatars.clear();
    m_requestedStatusMedia.clear();
    m_callLogs.clear();
    m_channels.clear();
    m_communities.clear();
    m_pairingQr.clear();
    m_pairingCode.clear();
	clearComposerLinkPreview();
    QSettings().setValue(QStringLiteral("accounts/current"), m_profile);
    emit profileChanged();
    emit statusChanged();
    emit chatsChanged();
    emit archivedChatsChanged();
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
    auto displayName = name.simplified();
    if (displayName.size() > 64)
        displayName.truncate(64);
    if (!displayName.isEmpty()) {
        m_profileDisplayNames.insert(slug, displayName);
        settings.beginGroup(QStringLiteral("accounts/displayNames"));
        settings.setValue(slug, displayName);
        settings.endGroup();
        emit profileDisplayNamesChanged();
    }
    ensureProfileMonitor(slug);
    emit profilesChanged();
    switchProfile(slug);
}

bool RpcClient::profileRemovable(const QString &profile) const
{
    return profile != QStringLiteral("default")
        && m_profiles.contains(profile)
        && m_profiles.size() > 1;
}

// removeProfile deletes an account and everything stored for it.
//
// The name is validated against the same pattern the daemon accepts before it
// is ever turned into a path, so a crafted profile name cannot reach outside
// the application's own data and cache directories.
void RpcClient::removeProfile(const QString &profile)
{
    static const QRegularExpression validProfile(QStringLiteral("^[a-z0-9][a-z0-9_-]{0,31}$"));
    if (!validProfile.match(profile).hasMatch() || !profileRemovable(profile)) {
        emit errorOccurred(tr("That account cannot be removed."));
        return;
    }

    // Leave the account before deleting it: the open socket and the pane in
    // front of the reader both belong to data that is about to go.
    if (profile == m_profile) {
        QString replacement;
        for (const auto &candidate : std::as_const(m_profiles)) {
            if (candidate != profile) {
                replacement = candidate;
                break;
            }
        }
        if (replacement.isEmpty()) {
            emit errorOccurred(tr("That account cannot be removed."));
            return;
        }
        switchProfile(replacement);
    }

    // The account leaves the list before anything is torn down. Stopping its
    // daemon and its monitor lets their handlers run again inside this call,
    // and every one of them checks the list before acting.
    m_profiles.removeAll(profile);
    m_profileDisplayNames.remove(profile);
    m_profileUnreadCounts.remove(profile);
    QSettings settings;
    settings.setValue(QStringLiteral("accounts/profiles"), m_profiles);
    settings.beginGroup(QStringLiteral("accounts/displayNames"));
    settings.remove(profile);
    settings.endGroup();
    emit profilesChanged();
    emit profileDisplayNamesChanged();
    emit profileUnreadCountsChanged();

    if (auto *monitor = m_profileMonitors.take(profile)) {
        // deleteLater keeps the monitor alive until the event loop returns, and
        // its socket reports the vanishing server in the meantime. Cutting the
        // signals first stops it asking for the account's daemon back.
        QObject::disconnect(monitor, nullptr, this, nullptr);
        monitor->shutdown();
        monitor->deleteLater();
    }

    auto *process = m_ownedBackends.take(profile);
    if (process != nullptr) {
        QObject::disconnect(process, nullptr, this, nullptr);
        // The process owns its own deletion from here. waitForFinished would
        // block this call inside a nested wait while the account's own socket
        // handlers are still live, which is how the teardown used to re-enter
        // itself and corrupt the heap.
        connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished),
                process, &QObject::deleteLater);
    }

    // The files go once the daemon that holds them open has actually gone, so
    // it cannot write the directory back after it is deleted.
    const auto deleteFiles = [this, profile] {
        QLocalServer::removeServer(socketPathForProfile(profile));
        bool removed = true;
        for (const auto &directory : {profileDataDir(profile), profileCacheDir(profile)}) {
            QDir folder(directory);
            if (folder.exists() && !folder.removeRecursively())
                removed = false;
        }
        if (!removed) {
            emit errorOccurred(tr("The account was removed, but some of its files could not be deleted."));
            return;
        }
        emit noticeOccurred(tr("Account removed."));
    };

    if (process == nullptr || process->state() == QProcess::NotRunning) {
        deleteFiles();
        return;
    }
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [deleteFiles](int, QProcess::ExitStatus) { deleteFiles(); });
    // A daemon that ignores the polite request still has to let go of the files.
    auto *forceStop = new QTimer(process);
    forceStop->setSingleShot(true);
    forceStop->setInterval(1500);
    connect(forceStop, &QTimer::timeout, process, [process] {
        if (process->state() != QProcess::NotRunning)
            process->kill();
    });
    forceStop->start();
    process->terminate();
}

void RpcClient::refreshUpdateStatus()
{
    sendRequest(QStringLiteral("update.status"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        m_updateStatus = result.toObject().toVariantMap();
        emit updateStatusChanged();
        // update.available is published once, when the version the daemon
        // found changes. A window that opened after that - the usual case,
        // since the daemon starts first - would never hear it, so the status
        // answer offers it too. The window remembers what it has already
        // offered and does not ask twice.
        if (m_updateStatus.value(QStringLiteral("available")).toBool()) {
            emit updateAvailable(m_updateStatus.value(QStringLiteral("latest")).toString());
            return;
        }
        // Opening the application is when somebody would want to know, and a
        // daemon that has been running for days may have last looked long ago.
        // A check a day is enough to notice a release without asking GitHub
        // about it on every window.
        const auto checkedAt = m_updateStatus.value(QStringLiteral("checked_at")).toLongLong();
        const auto dayMs = 24LL * 60LL * 60LL * 1000LL;
        if (checkedAt <= 0 || QDateTime::currentMSecsSinceEpoch() - checkedAt > dayMs) {
            startUpdateCheck(false);
        }
    });
}

void RpcClient::checkForUpdates()
{
    // Asking now rather than waiting for the next few-hourly look: somebody
    // pressed a button and is waiting for an answer.
    startUpdateCheck(true);
}

// announce says whether anybody is waiting to be told what came of this. A
// check the window starts by itself, on the first connection of the day, says
// nothing unless there is actually a new version.
void RpcClient::startUpdateCheck(bool announce)
{
    if (m_checkingForUpdates) {
        return;
    }
    m_checkingForUpdates = true;
    emit checkingForUpdatesChanged();
    sendRequest(QStringLiteral("update.check"), {}, [this, announce](const QJsonValue &result, const QJsonObject &error) {
        m_checkingForUpdates = false;
        emit checkingForUpdatesChanged();
        if (!error.isEmpty()) {
            const auto message = error.value(QStringLiteral("message")).toString();
            if (announce) {
                emit updateCheckFinished(false, QString(), message);
                emit updateFailed(message);
            }
            return;
        }
        m_updateStatus = result.toObject().toVariantMap();
        emit updateStatusChanged();
        const auto available = m_updateStatus.value(QStringLiteral("available")).toBool();
        const auto latest = m_updateStatus.value(QStringLiteral("latest")).toString();
        const auto failure = m_updateStatus.value(QStringLiteral("error")).toString();
        if (announce) {
            emit updateCheckFinished(available, latest, failure);
        }
        if (available) {
            emit updateAvailable(latest);
        }
    });
}

void RpcClient::downloadUpdate()
{
    m_updateStatus.insert(QStringLiteral("downloading"), true);
    m_updateStatus.insert(QStringLiteral("error"), QString());
    emit updateStatusChanged();
    sendRequest(QStringLiteral("update.download"), {}, [this](const QJsonValue &, const QJsonObject &error) {
        if (error.isEmpty())
            return;
        m_updateStatus.insert(QStringLiteral("downloading"), false);
        emit updateStatusChanged();
        emit updateFailed(error.value(QStringLiteral("message")).toString());
    });
}

void RpcClient::openReleasePage()
{
    const auto releases = QStringLiteral("https://github.com/shukiv/whatsappgo/releases/latest");
    const QUrl page(m_updateStatus.value(QStringLiteral("url")).toString());
    // The address arrives over the socket and is handed to whatever program
    // the desktop has registered for its scheme, so only the release page
    // this application publishes is opened; anything else falls back to it.
    const bool ours = page.scheme() == QStringLiteral("https")
        && page.host() == QStringLiteral("github.com")
        && page.path().startsWith(QStringLiteral("/shukiv/whatsappgo/"));
    QDesktopServices::openUrl(ours ? page : QUrl(releases));
}

bool RpcClient::updateInstallable() const
{
    return m_updateStatus.value(QStringLiteral("installable")).toBool() && updateinstaller::installable();
}

bool RpcClient::installUpdate()
{
    const auto path = m_updateStatus.value(QStringLiteral("downloaded")).toString();
    if (path.isEmpty()) {
        emit updateFailed(tr("There is nothing downloaded to install."));
        return false;
    }
    const auto outcome = updateinstaller::install(path);
    if (!outcome.ok) {
        emit updateFailed(outcome.message);
        return false;
    }
    if (!outcome.message.isEmpty())
        emit noticeOccurred(outcome.message);
    if (outcome.restart) {
        // Started as this process goes away rather than beside it: the new
        // version brings up its own daemons, and two clients sharing one
        // socket would fight over them.
        const auto relaunch = QString::fromLocal8Bit(qgetenv("APPIMAGE"));
        connect(qApp, &QCoreApplication::aboutToQuit, qApp, [relaunch] {
            QProcess::startDetached(relaunch, {});
        });
    }
    if (outcome.restart || outcome.quit) {
        stopOwnedBackends();
        QTimer::singleShot(0, qApp, &QCoreApplication::quit);
    }
    return true;
}

void RpcClient::refreshBugReportEnvironment()
{
    sendRequest(QStringLiteral("bugreport.environment"), {}, [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty())
            return;
        const auto rendered = result.toObject().value(QStringLiteral("rendered")).toString();
        if (rendered == m_bugReportEnvironment)
            return;
        m_bugReportEnvironment = rendered;
        emit bugReportEnvironmentChanged();
    });
}

void RpcClient::submitBugReport(const QString &subject, const QString &body)
{
    // The daemon validates and bounds this text as well; checking here only
    // keeps the dialog from sending an obviously empty report.
    if (subject.trimmed().isEmpty() || body.trimmed().isEmpty()) {
        emit bugReportFinished(false, tr("A report needs a subject and a description."), QString());
        return;
    }
    sendRequest(QStringLiteral("bugreport.submit"),
                {{QStringLiteral("subject"), subject}, {QStringLiteral("body"), body}},
                [this](const QJsonValue &result, const QJsonObject &error) {
        if (!error.isEmpty()) {
            emit bugReportFinished(false, error.value(QStringLiteral("message")).toString(), QString());
            return;
        }
        emit bugReportFinished(true, tr("Report sent."),
                               result.toObject().value(QStringLiteral("url")).toString());
    });
}

void RpcClient::renameProfile(const QString &profile, const QString &displayName)
{
    if (!m_profiles.contains(profile))
        return;
    auto cleaned = displayName.simplified();
    if (cleaned.isEmpty())
        return;
    if (cleaned.size() > 64)
        cleaned.truncate(64);
    if (m_profileDisplayNames.value(profile).toString() == cleaned)
        return;

    m_profileDisplayNames.insert(profile, cleaned);
    QSettings settings;
    settings.beginGroup(QStringLiteral("accounts/displayNames"));
    settings.setValue(profile, cleaned);
    settings.endGroup();
    emit profileDisplayNamesChanged();
}

void RpcClient::searchMessages(const QString &query)
{
    m_conversationQuery = query;
    const auto chatJid = m_selectedChat.value(QStringLiteral("jid")).toString();
    if (query.trimmed().isEmpty() || chatJid.isEmpty()) {
        m_searchResults.clear();
        emit searchResultsChanged();
        return;
    }
    sendRequest(QStringLiteral("messages.search"),
                {{QStringLiteral("query"), query},
                 {QStringLiteral("limit"), 100},
                 {QStringLiteral("chat_jid"), chatJid}},
                [this, query, chatJid](const QJsonValue &result, const QJsonObject &error) {
                    // The reader may have typed on, or opened another chat,
                    // before this answer arrived.
                    if (!error.isEmpty() || query != m_conversationQuery
                        || chatJid != m_selectedChat.value(QStringLiteral("jid")).toString())
                        return;
                    m_searchResults = result.toArray().toVariantList();
                    emit searchResultsChanged();
                });
}

void RpcClient::loadStarredMessages(const QString &chatJid)
{
    const auto generation = ++m_starredRequestGeneration;
    m_starredMessages.clear();
    m_starredMessagesLoading = true;
    emit starredMessagesChanged();
    sendRequest(QStringLiteral("messages.starred"),
                {{QStringLiteral("limit"), 100}, {QStringLiteral("chat_jid"), chatJid}},
                [this, generation](const QJsonValue &result, const QJsonObject &error) {
                    if (generation != m_starredRequestGeneration)
                        return;
                    m_starredMessagesLoading = false;
                    if (error.isEmpty())
                        m_starredMessages = result.toObject().value(QStringLiteral("items")).toArray().toVariantList();
                    emit starredMessagesChanged();
                });
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
                    if (belongsToOpenChat(message)) {
                        upsertMessage(message);
                        const auto path = message.value(QStringLiteral("media_path")).toString();
                        if (!path.isEmpty())
                            emit mediaReady(message.value(QStringLiteral("id")).toString(), path);
                    }
                }
                pumpMediaQueue();
            },
            OnFailure::StayQuiet);
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
			// A download outlives the conversation it was started in. Its
			// answer used to be inserted wherever the reader had moved to,
			// which put one person's picture in somebody else's conversation.
			if (!belongsToOpenChat(message))
				return;
			upsertMessage(message);
			// Downloading only makes the file available. What happens next is
			// the caller's decision: audio and video play inside the window,
			// documents are opened with their desktop application.
			const auto path = message.value(QStringLiteral("media_path")).toString();
			if (!path.isEmpty())
				emit mediaReady(message.value(QStringLiteral("id")).toString(), path);
		});
}

void RpcClient::sweepClipboardDirectory(const QString &directory, qint64 maxAgeSeconds)
{
	// A pasted image is written to disk, sent, and deleted. Anything still
	// here at startup belongs to a session that is over: a client that was
	// killed between the two, or - until the separator was fixed above - every
	// paste ever made on Windows.
	const auto now = QDateTime::currentDateTimeUtc();
	const QDir folder(directory);
	const auto entries = folder.entryInfoList({QStringLiteral("paste-*")}, QDir::Files);
	for (const auto &entry : entries) {
		if (entry.lastModified().toUTC().secsTo(now) >= maxAgeSeconds)
			QFile::remove(entry.absoluteFilePath());
	}
}

QString RpcClient::clipboardDirectory() const
{
	return QDir(QStandardPaths::writableLocation(QStandardPaths::CacheLocation)).filePath(QStringLiteral("clipboard"));
}

bool RpcClient::isClipboardFile(const QString &path) const
{
	// Qt spells every path with a forward slash, on Windows as much as
	// anywhere else, so QDir::separator - a backslash there - never matched
	// what absoluteFilePath returns. Nothing pasted on Windows was recognised
	// as this application's own file, so the temporary copies were never
	// deleted and collected in the cache directory.
	const auto directory = QDir(clipboardDirectory()).absolutePath() + QLatin1Char('/');
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

// rotatedImage writes a turned copy of a picture. The preview turns what is on
// screen, and the file was sent untouched: the picture arrived the way the
// camera wrote it, with the turning the reader had asked for thrown away.
QString RpcClient::rotatedImage(const QString &localUrl, int degrees)
{
	const int turn = ((degrees % 360) + 360) % 360;
	if (turn == 0)
		return localUrl;
	const auto path = QUrl(localUrl).isLocalFile() ? QUrl(localUrl).toLocalFile() : localUrl;
	QImage image(path);
	if (image.isNull()) {
		emit errorOccurred(tr("Could not turn the image."));
		return localUrl;
	}
	const auto directory = clipboardDirectory();
	if (!QDir().mkpath(directory)) {
		emit errorOccurred(tr("Could not turn the image."));
		return localUrl;
	}
	const auto turned = QDir(directory).filePath(
		QStringLiteral("turned-%1.png").arg(QUuid::createUuid().toString(QUuid::WithoutBraces)));
	QTransform rotation;
	rotation.rotate(turn);
	if (!image.transformed(rotation, Qt::SmoothTransformation).save(turned, "PNG")) {
		emit errorOccurred(tr("Could not turn the image."));
		return localUrl;
	}
	return QUrl::fromLocalFile(turned).toString();
}

void RpcClient::sendClipboardImage(const QString &localUrl, const QString &caption, const QString &replyTo)
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
		 {QStringLiteral("path"), path}, {QStringLiteral("caption"), caption},
		 {QStringLiteral("reply_to"), replyTo}},
		[this, path](const QJsonValue &, const QJsonObject &error) {
			setBusy(false);
			QFile::remove(path);
			if (error.isEmpty()) {
				emit noticeOccurred(tr("Image sent"));
				emit messageSent();
			}
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
	const auto localPath = localFilePath(path);
	if (localPath.isEmpty())
		return false;
	QImage image(localPath);
	if (image.isNull()) {
		emit errorOccurred(tr("Could not read this image."));
		return false;
	}
	QGuiApplication::clipboard()->setImage(image);
	emit noticeOccurred(tr("Image copied"));
	return true;
}

void RpcClient::saveImage(const QString &path, const QString &destination)
{
	const auto sourcePath = localFilePath(path);
	const auto destinationPath = localFilePath(destination);
	if (sourcePath.isEmpty() || destinationPath.isEmpty()) {
		emit errorOccurred(tr("Could not save this image."));
		return;
	}
	if (QFileInfo(sourcePath).absoluteFilePath() == QFileInfo(destinationPath).absoluteFilePath()) {
		emit noticeOccurred(tr("Image saved"));
		return;
	}
	QFile source(sourcePath);
	QSaveFile output(destinationPath);
	if (!source.open(QIODevice::ReadOnly) || !output.open(QIODevice::WriteOnly)) {
		emit errorOccurred(tr("Could not save this image."));
		return;
	}
	while (!source.atEnd()) {
		const auto block = source.read(256 * 1024);
		if (block.isEmpty() && source.error() != QFile::NoError) {
			output.cancelWriting();
			emit errorOccurred(tr("Could not save this image."));
			return;
		}
		if (output.write(block) != block.size()) {
			output.cancelWriting();
			emit errorOccurred(tr("Could not save this image."));
			return;
		}
	}
	if (!output.commit()) {
		emit errorOccurred(tr("Could not save this image."));
		return;
	}
	emit noticeOccurred(tr("Image saved"));
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
                [this, jid](const QJsonValue &result, const QJsonObject &error) {
                    m_pendingChatAvatars.remove(jid);
                    if (!error.isEmpty())
                        return;
                    applyChatAvatar(jid, result.toObject().value(QStringLiteral("path")).toString());
                },
                OnFailure::StayQuiet);
}

void RpcClient::applyChatAvatar(const QString &jid, const QString &path)
{
    if (jid.isEmpty() || path.isEmpty())
        return;
    const auto update = [&jid, &path](QVariantList &chats) {
        bool changed = false;
        for (auto &entry : chats) {
            auto chat = entry.toMap();
            if (chat.value(QStringLiteral("jid")).toString() != jid
                || chat.value(QStringLiteral("avatar_path")).toString() == path)
                continue;
            chat.insert(QStringLiteral("avatar_path"), path);
            entry = chat;
            changed = true;
        }
        return changed;
    };

    if (update(m_chats)) {
        syncChatListModel();
        emit chatsChanged();
    }
    if (update(m_archivedChats)) {
        m_archivedChatList.sync(m_archivedChats);
        emit archivedChatsChanged();
    }
    if (m_selectedChat.value(QStringLiteral("jid")).toString() == jid
        && m_selectedChat.value(QStringLiteral("avatar_path")).toString() != path) {
        m_selectedChat.insert(QStringLiteral("avatar_path"), path);
        emit selectedChatChanged();
    }
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

// belongsToOpenChat answers whether a message is part of the conversation on
// screen. Anything that arrives through a callback has to ask: the reader can
// open another chat between the request and its answer.
// refreshOneMessage re-reads a single message from the daemon and puts it back
// into the open conversation, leaving every loaded page and the reader's place
// where they were.
void RpcClient::refreshOneMessage(const QString &messageId)
{
    const auto jid = m_selectedChat.value(QStringLiteral("jid")).toString();
    if (jid.isEmpty() || messageId.isEmpty())
        return;
    if (m_messages.byId(messageId).isEmpty()) {
        // Not a message on screen: the change belongs to a page that was never
        // loaded, and there is nothing to update.
        refreshChats();
        return;
    }
    sendRequest(QStringLiteral("message.get"),
                {{QStringLiteral("chat_jid"), jid}, {QStringLiteral("message_id"), messageId}},
                [this, jid](const QJsonValue &result, const QJsonObject &error) {
                    if (!error.isEmpty() || m_selectedChat.value(QStringLiteral("jid")).toString() != jid)
                        return;
                    const auto message = result.toObject().toVariantMap();
                    if (!message.value(QStringLiteral("id")).toString().isEmpty())
                        upsertMessage(message);
                    refreshChats();
                },
                OnFailure::StayQuiet);
}

// acknowledgeIncoming sends the read receipt for one message the reader can see.
void RpcClient::acknowledgeIncoming(const QVariantMap &message)
{
    if (message.value(QStringLiteral("from_me")).toBool())
        return;
    const auto chat = message.value(QStringLiteral("chat_jid")).toString();
    const auto id = message.value(QStringLiteral("id")).toString();
    if (chat.isEmpty() || id.isEmpty())
        return;
    sendRequest(QStringLiteral("chat.read"),
                {{QStringLiteral("chat_jid"), chat},
                 {QStringLiteral("sender_jid"), message.value(QStringLiteral("sender_jid")).toString()},
                 {QStringLiteral("message_ids"), QJsonArray{id}},
                 {QStringLiteral("timestamp"), message.value(QStringLiteral("timestamp")).toLongLong()}},
                {}, OnFailure::StayQuiet);
}

bool RpcClient::belongsToOpenChat(const QVariantMap &message) const
{
    const auto open = m_selectedChat.value(QStringLiteral("jid")).toString();
    if (open.isEmpty())
        return false;
    const auto chat = message.value(QStringLiteral("chat_jid")).toString();
    return chat.isEmpty() || chat == open;
}
