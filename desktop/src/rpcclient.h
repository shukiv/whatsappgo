#pragma once

#include "messagemodel.h"

#include <QAbstractItemModel>
#include <QHash>
#include <QJsonObject>
#include <QLocalSocket>
#include <QObject>
#include <QTimer>
#include <QVariantList>
#include <QVariantMap>
#include <QStringList>
#include <QSet>

#include <functional>

class QProcess;
class ProfileMonitor;

class RpcClient final : public QObject
{
    Q_OBJECT
    Q_PROPERTY(bool daemonConnected READ daemonConnected NOTIFY daemonConnectedChanged)
    Q_PROPERTY(bool loggedIn READ loggedIn NOTIFY statusChanged)
    Q_PROPERTY(QVariantMap status READ status NOTIFY statusChanged)
    Q_PROPERTY(QVariantList chats READ chats NOTIFY chatsChanged)
    Q_PROPERTY(QVariantList archivedChats READ archivedChats NOTIFY archivedChatsChanged)
    Q_PROPERTY(int archivedCount READ archivedCount NOTIFY archivedChatsChanged)
    Q_PROPERTY(QAbstractItemModel *messages READ messages CONSTANT)
    Q_PROPERTY(QVariantMap selectedChat READ selectedChat NOTIFY selectedChatChanged)
    Q_PROPERTY(QVariantMap chatInfo READ chatInfo NOTIFY chatInfoChanged)
    Q_PROPERTY(QVariantList sharedContent READ sharedContent NOTIFY sharedContentChanged)
    Q_PROPERTY(bool sharedContentHasMore READ sharedContentHasMore NOTIFY sharedContentChanged)
    Q_PROPERTY(QString sharedContentCategory READ sharedContentCategory NOTIFY sharedContentChanged)
    Q_PROPERTY(bool sharedContentLoading READ sharedContentLoading NOTIFY sharedContentChanged)
    Q_PROPERTY(QString pairingQr READ pairingQr NOTIFY pairingQrChanged)
    Q_PROPERTY(QString pairingCode READ pairingCode NOTIFY pairingCodeChanged)
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(QString profile READ profile NOTIFY profileChanged)
    Q_PROPERTY(QStringList profiles READ profiles NOTIFY profilesChanged)
    Q_PROPERTY(QVariantMap profileDisplayNames READ profileDisplayNames NOTIFY profileDisplayNamesChanged)
    Q_PROPERTY(QVariantMap profileUnreadCounts READ profileUnreadCounts NOTIFY profileUnreadCountsChanged)
    Q_PROPERTY(QVariantList searchResults READ searchResults NOTIFY searchResultsChanged)
    Q_PROPERTY(QVariantList statusUpdates READ statusUpdates NOTIFY statusUpdatesChanged)
    Q_PROPERTY(QVariantList callLogs READ callLogs NOTIFY callLogsChanged)
    Q_PROPERTY(QVariantList channels READ channels NOTIFY channelsChanged)
    Q_PROPERTY(QVariantList communities READ communities NOTIFY communitiesChanged)
    Q_PROPERTY(bool clipboardHasImage READ clipboardHasImage NOTIFY clipboardChanged)
    Q_PROPERTY(QVariantMap composerLinkPreview READ composerLinkPreview NOTIFY composerLinkPreviewChanged)

public:
    explicit RpcClient(const QString &initialProfile = QString(), const QString &initialChat = QString(), QObject *parent = nullptr);
    ~RpcClient() override;

    bool daemonConnected() const;
    bool loggedIn() const;
    QVariantMap status() const { return m_status; }
    QVariantList chats() const { return m_chats; }
    QVariantList archivedChats() const { return m_archivedChats; }
    int archivedCount() const { return m_archivedCount; }
    QAbstractItemModel *messages() { return &m_messages; }
    QVariantMap selectedChat() const { return m_selectedChat; }
    QVariantMap chatInfo() const { return m_chatInfo; }
    QVariantList sharedContent() const { return m_sharedContent; }
    bool sharedContentHasMore() const { return m_sharedContentHasMore; }
    QString sharedContentCategory() const { return m_sharedContentCategory; }
    bool sharedContentLoading() const { return m_sharedContentLoading; }
    QString pairingQr() const { return m_pairingQr; }
    QString pairingCode() const { return m_pairingCode; }
    bool busy() const { return m_busy; }
    QString profile() const { return m_profile; }
    QStringList profiles() const { return m_profiles; }
    QVariantMap profileDisplayNames() const { return m_profileDisplayNames; }
    QVariantMap profileUnreadCounts() const { return m_profileUnreadCounts; }
    QVariantList searchResults() const { return m_searchResults; }
    QVariantList statusUpdates() const { return m_statusUpdates; }
    QVariantList callLogs() const { return m_callLogs; }
    QVariantList channels() const { return m_channels; }
    QVariantList communities() const { return m_communities; }
    bool clipboardHasImage() const;
    QVariantMap composerLinkPreview() const { return m_composerLinkPreview; }

    Q_INVOKABLE void reconnect();
    Q_INVOKABLE void refreshStatus();
    Q_INVOKABLE void refreshChats(const QString &query = {});
    Q_INVOKABLE void refreshArchived();
    Q_INVOKABLE void setChatPinned(const QString &jid, bool pinned);
    Q_INVOKABLE void setChatMuted(const QString &jid, bool muted);
    Q_INVOKABLE void setChatArchived(const QString &jid, bool archived);
    Q_INVOKABLE void setChatRead(const QString &jid, bool read);
    Q_INVOKABLE void openChat(const QString &jid, const QString &title);
    Q_INVOKABLE void closeChat();
    Q_INVOKABLE void refreshChatInfo();
    Q_INVOKABLE void refreshSharedContent(const QString &category, bool append = false);
    Q_INVOKABLE void clearChatInfo();
    Q_INVOKABLE void loadOlderMessages();
    Q_INVOKABLE bool canLoadOlderMessages() const;
    Q_INVOKABLE void sendMessage(const QString &text, const QString &replyTo = {});
    Q_INVOKABLE void sendStatusReply(const QString &recipientJid, const QString &statusMessageId, const QString &text);
    Q_INVOKABLE void requestLinkPreview(const QString &text);
    Q_INVOKABLE void clearComposerLinkPreview();
    Q_INVOKABLE void sendFile(const QString &localUrl, const QString &caption = {});
    Q_INVOKABLE void sendVoice(const QString &localUrl);
    Q_INVOKABLE void editMessage(const QString &messageId, const QString &text);
    Q_INVOKABLE void deleteMessage(const QString &messageId, const QString &senderJid = {});
    Q_INVOKABLE void reactMessage(const QString &messageId, const QString &senderJid, const QString &reaction);
    Q_INVOKABLE void pinMessage(const QString &messageId, const QString &senderJid, int durationSeconds);
    Q_INVOKABLE void unpinMessage(const QString &messageId, const QString &senderJid = {});
    Q_INVOKABLE int messageIndex(const QString &messageId) const;
    Q_INVOKABLE void startChat(const QString &phone);
    Q_INVOKABLE void setTyping(bool typing);
    Q_INVOKABLE void startPairing();
    Q_INVOKABLE void pairPhone(const QString &phone);
    Q_INVOKABLE void logout();
    Q_INVOKABLE void switchProfile(const QString &profile);
    Q_INVOKABLE void addProfile(const QString &name);
    Q_INVOKABLE void renameProfile(const QString &profile, const QString &displayName);
    Q_INVOKABLE void searchMessages(const QString &query);
    Q_INVOKABLE void openFile(const QString &path);
    Q_INVOKABLE void downloadMedia(const QString &messageId);
    // Fetches a picture that has no preview, a few at a time.
    Q_INVOKABLE void ensureMedia(const QString &messageId);
    // The voice note that follows the given message, when the conversation
    // continues with one.
    Q_INVOKABLE QVariantMap nextAudioAfter(const QString &messageId) const;
    Q_INVOKABLE QString prepareClipboardImage();
    Q_INVOKABLE void sendClipboardImage(const QString &localUrl, const QString &caption = {});
    Q_INVOKABLE void discardClipboardImage(const QString &localUrl);
    Q_INVOKABLE void copyImage(const QString &messageId, const QString &path = {});
    Q_INVOKABLE void copyText(const QString &text);
    Q_INVOKABLE void refreshStatuses();
    Q_INVOKABLE void refreshChatAvatar(const QString &jid);
    Q_INVOKABLE void fetchStatusAvatar(const QString &jid);
    Q_INVOKABLE void ensureStatusMedia(const QString &messageId);
    Q_INVOKABLE void refreshCalls();
    Q_INVOKABLE void refreshChannels();
    Q_INVOKABLE void refreshCommunities();

signals:
    void daemonConnectedChanged();
    void statusChanged();
    void chatsChanged();
    void archivedChatsChanged();
    void selectedChatChanged();
    void chatInfoChanged();
    void sharedContentChanged();
    void pairingQrChanged();
    void pairingCodeChanged();
    void busyChanged();
    void errorOccurred(const QString &message);
    void messageSent();
    void statusReplyFinished(const QString &recipientJid, const QString &statusMessageId, bool success, const QString &message);
    void profileChanged();
    void profilesChanged();
    void profileDisplayNamesChanged();
    void profileUnreadCountsChanged();
    void searchResultsChanged();
    void statusUpdatesChanged();
    void callLogsChanged();
    void channelsChanged();
    void communitiesChanged();
    void clipboardChanged();
    void composerLinkPreviewChanged();
    void noticeOccurred(const QString &message);
    void notificationRequested(const QString &chatJid, const QString &title, const QString &body);
    // A message's file is cached and can be played or opened.
    void mediaReady(const QString &messageId, const QString &path);

private:
    using Callback = std::function<void(const QJsonValue &, const QJsonObject &)>;

    void connectSocket();
    void sendRequest(const QString &method, const QJsonObject &params, Callback callback = {});
    void processLine(const QByteArray &line);
    void processEvent(const QString &name, const QJsonValue &data);
    void setBusy(bool value);
    void upsertMessage(const QVariantMap &message);
    void rememberMessages(const QString &chatJid, const QVariantList &messages);
    void requestRemoteHistory();
    void loadRemoteHistoryPage();
    void refreshOpenMessages();
    void pumpMediaQueue();
    bool copyImageFile(const QString &path);
    QString clipboardDirectory() const;
    bool isClipboardFile(const QString &path) const;
    QString socketPath() const;
    void startBackendForProfile(const QString &profile);
    void ensureProfileMonitor(const QString &profile);
    void stopOwnedBackends();

    QLocalSocket m_socket;
    QTimer m_reconnectTimer;
    QByteArray m_readBuffer;
    quint64 m_nextId = 0;
    QHash<QString, Callback> m_pending;
    QVariantMap m_status;
    QVariantList m_chats;
    QVariantList m_archivedChats;
    int m_archivedCount = 0;
    MessageListModel m_messages;
    QHash<QString, QVariantList> m_messageCache;
    QStringList m_messageCacheOrder;
    QVariantMap m_selectedChat;
    QVariantMap m_chatInfo;
    QVariantList m_sharedContent;
    bool m_sharedContentHasMore = false;
    QString m_sharedContentCategory;
    bool m_sharedContentLoading = false;
    QVariantList m_searchResults;
    QVariantList m_statusUpdates;
    QVariantList m_callLogs;
    QVariantList m_channels;
    QVariantList m_communities;
    QVariantMap m_composerLinkPreview;
    QString m_linkPreviewRequestText;
    QString m_pairingQr;
    QString m_pairingCode;
    QString m_profile = QStringLiteral("default");
    QStringList m_profiles{QStringLiteral("default")};
    QVariantMap m_profileDisplayNames;
    QVariantMap m_profileUnreadCounts;
    bool m_busy = false;
    QHash<QString, QProcess *> m_ownedBackends;
    QHash<QString, ProfileMonitor *> m_profileMonitors;
    bool m_shuttingDown = false;
    QString m_initialChat;
    bool m_loadingOlder = false;
    bool m_hasMore = false;
    qint64 m_nextBefore = 0;
    QSet<QString> m_requestedHistoryBoundaries;
    bool m_waitingRemoteHistory = false;
    QString m_pendingCopyImageId;
    QSet<QString> m_requestedMedia;
    QSet<QString> m_requestedStatusAvatars;
    QSet<QString> m_pendingChatAvatars;
    QHash<QString, qint64> m_chatAvatarRequestedAt;
    QSet<QString> m_requestedStatusMedia;
    QStringList m_mediaQueue;
    int m_mediaInFlight = 0;
};
