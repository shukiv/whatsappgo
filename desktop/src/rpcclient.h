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
    Q_PROPERTY(QAbstractItemModel *chatListModel READ chatListModel CONSTANT)
    Q_PROPERTY(QVariantList archivedChats READ archivedChats NOTIFY archivedChatsChanged)
    Q_PROPERTY(QAbstractItemModel *archivedChatListModel READ archivedChatListModel CONSTANT)
    Q_PROPERTY(int archivedCount READ archivedCount NOTIFY archivedChatsChanged)
    Q_PROPERTY(QAbstractItemModel *messages READ messages CONSTANT)
    Q_PROPERTY(QVariantMap selectedChat READ selectedChat NOTIFY selectedChatChanged)
    Q_PROPERTY(QVariantMap selectedPresence READ selectedPresence NOTIFY selectedPresenceChanged)
    Q_PROPERTY(QVariantMap chatInfo READ chatInfo NOTIFY chatInfoChanged)
    Q_PROPERTY(QStringList blockedContacts READ blockedContacts NOTIFY blockedContactsChanged)
    Q_PROPERTY(QVariantMap privacySettings READ privacySettings NOTIFY privacySettingsChanged)
    Q_PROPERTY(QVariantList chatLabels READ chatLabels NOTIFY chatLabelsChanged)
    Q_PROPERTY(QVariantList mediaLibrary READ mediaLibrary NOTIFY mediaLibraryChanged)
    Q_PROPERTY(bool mediaLibraryHasMore READ mediaLibraryHasMore NOTIFY mediaLibraryChanged)
    Q_PROPERTY(QString mediaLibraryCategory READ mediaLibraryCategory NOTIFY mediaLibraryChanged)
    Q_PROPERTY(bool mediaLibraryLoading READ mediaLibraryLoading NOTIFY mediaLibraryChanged)
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
    Q_PROPERTY(QString chatQuery READ chatQuery NOTIFY chatQueryChanged)
    Q_PROPERTY(QString conversationQuery READ conversationQuery NOTIFY searchResultsChanged)
    Q_PROPERTY(QVariantList chatSearchHits READ chatSearchHits NOTIFY chatSearchHitsChanged)
    Q_PROPERTY(QVariantList contactSearchHits READ contactSearchHits NOTIFY contactSearchHitsChanged)
    Q_PROPERTY(QVariantList messageSearchHits READ messageSearchHits NOTIFY messageSearchHitsChanged)
    Q_PROPERTY(QVariantList starredMessages READ starredMessages NOTIFY starredMessagesChanged)
    Q_PROPERTY(bool starredMessagesLoading READ starredMessagesLoading NOTIFY starredMessagesChanged)
    Q_PROPERTY(QVariantList statusUpdates READ statusUpdates NOTIFY statusUpdatesChanged)
    Q_PROPERTY(QVariantList callLogs READ callLogs NOTIFY callLogsChanged)
    Q_PROPERTY(QVariantList channels READ channels NOTIFY channelsChanged)
    Q_PROPERTY(QVariantList communities READ communities NOTIFY communitiesChanged)
    Q_PROPERTY(bool clipboardHasImage READ clipboardHasImage NOTIFY clipboardChanged)
    Q_PROPERTY(QVariantMap composerLinkPreview READ composerLinkPreview NOTIFY composerLinkPreviewChanged)
    Q_PROPERTY(QString bugReportEnvironment READ bugReportEnvironment NOTIFY bugReportEnvironmentChanged)
    Q_PROPERTY(QVariantMap updateStatus READ updateStatus NOTIFY updateStatusChanged)
    // True while a check the reader asked for is in flight, so the control
    // they pressed can show that something is happening.
    Q_PROPERTY(bool checkingForUpdates READ checkingForUpdates NOTIFY checkingForUpdatesChanged)

public:
    explicit RpcClient(const QString &initialProfile = QString(), const QString &initialChat = QString(), QObject *parent = nullptr);
    ~RpcClient() override;

    // True when something is accepting connections on socketPath right now.
    // A refused connection is not proof that the daemon is gone - a full listen
    // backlog refuses too - so this answers the question the reconnect path
    // actually needs before it deletes a socket and starts a second daemon.
    // Deletes leftover pasted images. Static and parameterised so the test can
    // point it at a directory of its own.
    static void sweepClipboardDirectory(const QString &directory, qint64 maxAgeSeconds);
    static bool backendIsListening(const QString &socketPath);

    // The address the daemon for this account listens on: a socket path on
    // Unix, a named pipe name on Windows. Mirrors config.SocketAddress.
    static QString socketPathForProfile(const QString &profile);

    bool daemonConnected() const;
    bool loggedIn() const;
    QVariantMap status() const { return m_status; }
    QVariantList chats() const { return m_chats; }
    QAbstractItemModel *chatListModel() { return &m_chatList; }
    QVariantList archivedChats() const { return m_archivedChats; }
    QAbstractItemModel *archivedChatListModel() { return &m_archivedChatList; }
    int archivedCount() const { return m_archivedCount; }
    QAbstractItemModel *messages() { return &m_messages; }
    // The conversation as the model that holds it, for callers that need more
    // than a view: the tests, which read messages back by identity.
    MessageListModel *messageList() { return &m_messages; }
    QVariantMap selectedChat() const { return m_selectedChat; }
    QVariantMap selectedPresence() const { return m_selectedPresence; }
    QVariantMap chatInfo() const { return m_chatInfo; }
    QStringList blockedContacts() const { return m_blockedContacts; }
    QVariantMap privacySettings() const { return m_privacySettings; }
    QVariantList chatLabels() const { return m_chatLabels; }
    QVariantList mediaLibrary() const { return m_mediaLibrary; }
    bool mediaLibraryHasMore() const { return m_mediaLibraryHasMore; }
    QString mediaLibraryCategory() const { return m_mediaLibraryCategory; }
    bool mediaLibraryLoading() const { return m_mediaLibraryLoading; }
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
    QString chatQuery() const { return m_chatQuery; }
    QString conversationQuery() const { return m_conversationQuery; }
    QVariantList chatSearchHits() const { return m_chatSearchHits; }
    QVariantList contactSearchHits() const { return m_contactSearchHits; }
    QVariantList messageSearchHits() const { return m_messageSearchHits; }
    QVariantList starredMessages() const { return m_starredMessages; }
    bool starredMessagesLoading() const { return m_starredMessagesLoading; }
    QVariantList statusUpdates() const { return m_statusUpdates; }
    QVariantList callLogs() const { return m_callLogs; }
    QVariantList channels() const { return m_channels; }
    QVariantList communities() const { return m_communities; }
    bool clipboardHasImage() const;
    QVariantMap composerLinkPreview() const { return m_composerLinkPreview; }

    Q_INVOKABLE void reconnect();
    Q_INVOKABLE void refreshStatus();
    Q_INVOKABLE void refreshChats();
    // The sidebar query is remembered here rather than in QML: every event
    // that refreshes the chat list would otherwise drop it.
    Q_INVOKABLE void searchChats(const QString &query);
    Q_INVOKABLE void setChatListFilter(const QString &filter);
    Q_INVOKABLE void refreshArchived();
    Q_INVOKABLE void setChatPinned(const QString &jid, bool pinned);
    Q_INVOKABLE void setChatMuted(const QString &jid, bool muted, int durationSeconds = 0);
    Q_INVOKABLE void setChatArchived(const QString &jid, bool archived);
    Q_INVOKABLE void deleteChat(const QString &jid);
    Q_INVOKABLE void clearChat(const QString &jid);
    Q_INVOKABLE void setChatDisappearing(const QString &jid, int seconds);
    Q_INVOKABLE void exportChat(const QString &jid, const QString &destinationUrl);
    Q_INVOKABLE void setChatFavorite(const QString &jid, bool favorite);
    Q_INVOKABLE void markAllChatsRead();
    Q_INVOKABLE void createGroup(const QString &name, const QStringList &participants);
    Q_INVOKABLE void setChatRead(const QString &jid, bool read);
    Q_INVOKABLE void openChat(const QString &jid, const QString &title);
    Q_INVOKABLE void closeChat();
    Q_INVOKABLE void refreshChatInfo();
    Q_INVOKABLE void refreshSharedContent(const QString &category, bool append = false);
    Q_INVOKABLE void refreshMediaLibrary(const QString &category, bool append = false);
    Q_INVOKABLE void refreshBlockedContacts();
    Q_INVOKABLE void refreshPrivacySettings();
    Q_INVOKABLE void setPrivacySetting(const QString &name, const QString &value);
    Q_INVOKABLE void setAbout(const QString &text);
    Q_INVOKABLE void setChannelFollowed(const QString &jid, bool followed);
    Q_INVOKABLE void createChannel(const QString &name, const QString &description);
    Q_INVOKABLE void followChannelLink(const QString &link);
    Q_INVOKABLE void createCommunity(const QString &name);
    Q_INVOKABLE void joinGroupLink(const QString &link);

    // Bug reports. The environment is fetched separately so the dialog can
    // show the reader exactly what a report would disclose before they send it.
    // Updates. The daemon looks for them and downloads them; installing one
    // is this process's job because only it knows what it is running from.
    Q_INVOKABLE void refreshUpdateStatus();
    Q_INVOKABLE void checkForUpdates();
    Q_INVOKABLE void downloadUpdate();
    Q_INVOKABLE void openReleasePage();
    // Applies a file update.ready announced. The window restarts or closes
    // when it returns true; it stays where it is when it does not.
    Q_INVOKABLE bool installUpdate();
    Q_INVOKABLE bool updateInstallable() const;
    QVariantMap updateStatus() const { return m_updateStatus; }
    bool checkingForUpdates() const { return m_checkingForUpdates; }

    Q_INVOKABLE void refreshBugReportEnvironment();
    Q_INVOKABLE void submitBugReport(const QString &subject, const QString &body);
    QString bugReportEnvironment() const { return m_bugReportEnvironment; }
    Q_INVOKABLE void setChannelMuted(const QString &jid, bool muted);
    Q_INVOKABLE void postTextStatus(const QString &text, int background);
    Q_INVOKABLE void postMediaStatus(const QString &localUrl, const QString &caption);
    Q_INVOKABLE void refreshChatLabels();
    Q_INVOKABLE void createChatLabel(const QString &name);
    Q_INVOKABLE void setChatLabeled(const QString &jid, const QString &labelId, bool labeled);
    Q_INVOKABLE void setContactBlocked(const QString &jid, bool blocked);
    Q_INVOKABLE void clearChatInfo();
    Q_INVOKABLE void loadOlderMessages();
    // The next page of conversations for the sidebar.
    Q_INVOKABLE void loadMoreChats();
    // Re-reads the open conversation, keeping the pages already loaded. Used
    // after a reconnection, when anything that arrived meanwhile is missing.
    Q_INVOKABLE void refreshOpenMessages();
    Q_INVOKABLE bool canLoadOlderMessages() const;
    Q_INVOKABLE void sendMessage(const QString &text, const QString &replyTo = {});
    Q_INVOKABLE void sendStatusReply(const QString &recipientJid, const QString &statusMessageId, const QString &text);
    Q_INVOKABLE void requestLinkPreview(const QString &text);
    Q_INVOKABLE void clearComposerLinkPreview();
    Q_INVOKABLE void sendFile(const QString &localUrl, const QString &caption = {}, const QString &replyTo = {}, bool document = false);
    Q_INVOKABLE void sendVoice(const QString &localUrl, const QString &chatJid, const QString &recordingProfile, const QString &replyTo = {});
    Q_INVOKABLE void editMessage(const QString &messageId, const QString &text);
    Q_INVOKABLE void deleteMessage(const QString &messageId, const QString &senderJid = {});
    Q_INVOKABLE void reactMessage(const QString &messageId, const QString &senderJid, const QString &reaction);
    Q_INVOKABLE void pinMessage(const QString &messageId, const QString &senderJid, int durationSeconds);
    Q_INVOKABLE void starMessage(const QString &messageId, const QString &senderJid, bool fromMe, bool starred);
    Q_INVOKABLE void forwardMessage(const QString &messageId, const QString &toChatJid);
    Q_INVOKABLE void forwardMessageFrom(const QString &fromChatJid, const QString &messageId, const QString &toChatJid);
    Q_INVOKABLE void unpinMessage(const QString &messageId, const QString &senderJid = {});
    Q_INVOKABLE int messageIndex(const QString &messageId) const;
    Q_INVOKABLE QVariantMap messageById(const QString &messageId) const;
	Q_INVOKABLE void markMediaPlayed(const QString &messageId);
    Q_INVOKABLE void startChat(const QString &phone);
    Q_INVOKABLE void setTyping(bool typing);
    Q_INVOKABLE void startPairing();
    Q_INVOKABLE void pairPhone(const QString &phone);
    Q_INVOKABLE void logout();
    Q_INVOKABLE void switchProfile(const QString &profile);
    Q_INVOKABLE void addProfile(const QString &name);
    Q_INVOKABLE void renameProfile(const QString &profile, const QString &displayName);
    Q_INVOKABLE void removeProfile(const QString &profile);
    Q_INVOKABLE bool profileRemovable(const QString &profile) const;
    // Searches inside the open conversation, which is what the panel in the
    // chat header does; the sidebar keeps its own results elsewhere.
    Q_INVOKABLE void searchMessages(const QString &query);
    Q_INVOKABLE void loadStarredMessages(const QString &chatJid = {});
    Q_INVOKABLE void openFile(const QString &path);
    Q_INVOKABLE void downloadMedia(const QString &messageId);
    // Fetches a picture that has no preview, a few at a time.
    Q_INVOKABLE void ensureMedia(const QString &messageId);
    // The voice note that follows the given message, when the conversation
    // continues with one.
    Q_INVOKABLE QVariantMap nextAudioAfter(const QString &messageId) const;
    Q_INVOKABLE QString prepareClipboardImage();
    // Writes a turned copy of an image and returns its URL, so the picture that
    // is sent is the one the preview was showing. Returns the original URL when
    // there is nothing to turn.
    Q_INVOKABLE QString rotatedImage(const QString &localUrl, int degrees);
    Q_INVOKABLE void sendClipboardImage(const QString &localUrl, const QString &caption = {}, const QString &replyTo = {});
    Q_INVOKABLE void discardClipboardImage(const QString &localUrl);
    Q_INVOKABLE void copyImage(const QString &messageId, const QString &path = {});
    Q_INVOKABLE void saveImage(const QString &path, const QString &destination);
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
    void selectedPresenceChanged();
    void chatInfoChanged();
    void sharedContentChanged();
    void mediaLibraryChanged();
    void blockedContactsChanged();
    void privacySettingsChanged();
    void chatLabelsChanged();
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
    void chatQueryChanged();
    void chatSearchHitsChanged();
    void contactSearchHitsChanged();
    void messageSearchHitsChanged();
    void starredMessagesChanged();
    void statusUpdatesChanged();
    void callLogsChanged();
    void channelsChanged();
    void communitiesChanged();
    void clipboardChanged();
    void composerLinkPreviewChanged();
    void noticeOccurred(const QString &message);
    void notificationRequested(const QString &chatJid, const QString &title, const QString &body);
    void bugReportEnvironmentChanged();
    void updateStatusChanged();
    void checkingForUpdatesChanged();
    // The answer to a check somebody asked for, so the window can say what
    // came of it rather than leaving the button looking dead.
    void updateCheckFinished(bool available, const QString &version, const QString &error);
    // A version nobody has been offered yet.
    void updateAvailable(const QString &version);
    void updateProgress(qint64 received, qint64 total);
    void updateReady(const QString &path, const QString &version);
    void updateFailed(const QString &message);
    void bugReportFinished(bool success, const QString &message, const QString &url);
    // A message's file is cached and can be played or opened.
    void mediaReady(const QString &messageId, const QString &path);

private:
    using Callback = std::function<void(const QJsonValue &, const QJsonObject &)>;

    // Whether a failed request is worth interrupting the reader over. A
    // picture that did not download because the network blinked is not.
    enum class OnFailure {
        Report,
        StayQuiet,
    };

    void connectSocket();
    void sendRequest(const QString &method, const QJsonObject &params, Callback callback = {},
                     OnFailure onFailure = OnFailure::Report);
    void processLine(const QByteArray &line);
    void processEvent(const QString &name, const QJsonValue &data);
    void setBusy(bool value);
    void upsertMessage(const QVariantMap &message);
    bool belongsToOpenChat(const QVariantMap &message) const;
    void refreshOneMessage(const QString &messageId);
    void acknowledgeIncoming(const QVariantMap &message);
    void rememberMessages(const QString &chatJid, const QVariantList &messages);
    void upgradeSmallLinkPreviews(const QVariantList &messages);
    void requestRemoteHistory();
    void loadRemoteHistoryPage();
    void pumpMediaQueue();
    // Answers every request that will never come back, so a caller waiting on
    // one is not left waiting for ever, and forgets the media queue's places.
    void abandonPendingRequests(const QString &reason, bool announce);
    void syncChatListModel();
    void runSidebarSearch(const QString &query);
    void applyChatAvatar(const QString &jid, const QString &path);
    bool copyImageFile(const QString &path);
    QString clipboardDirectory() const;
    bool isClipboardFile(const QString &path) const;
    QString socketPath() const;
    void startBackendForProfile(const QString &profile);
    void ensureProfileMonitor(const QString &profile);
    void stopOwnedBackends();

    QLocalSocket m_socket;
    QTimer m_reconnectTimer;
    QTimer m_searchReplayTimer;
    QByteArray m_readBuffer;
    quint64 m_nextId = 0;
    QHash<QString, Callback> m_pending;
    QString m_bugReportEnvironment;
    // Requests whose failure is not shown to the reader.
    QSet<QString> m_quietRequests;
    QVariantMap m_updateStatus;
    bool m_checkingForUpdates = false;
    void startUpdateCheck(bool announce);
    QVariantMap m_status;
    QVariantList m_chats;
    ChatListModel m_chatList;
    QVariantList m_archivedChats;
    ChatListModel m_archivedChatList;
    QString m_chatListFilter = QStringLiteral("all");
    QString m_chatQuery;
    QString m_conversationQuery;
    QVariantList m_chatSearchHits;
    QVariantList m_contactSearchHits;
    QVariantList m_messageSearchHits;
    int m_archivedCount = 0;
    MessageListModel m_messages;
    QHash<QString, QVariantList> m_messageCache;
    QStringList m_messageCacheOrder;
    QVariantMap m_selectedChat;
    QVariantMap m_selectedPresence;
    QVariantMap m_chatInfo;
    QStringList m_blockedContacts;
    QVariantMap m_privacySettings;
    QVariantList m_chatLabels;
    QVariantList m_mediaLibrary;
    bool m_mediaLibraryHasMore = false;
    QString m_mediaLibraryCategory;
    bool m_mediaLibraryLoading = false;
    QVariantList m_sharedContent;
    bool m_sharedContentHasMore = false;
    QString m_sharedContentCategory;
    bool m_sharedContentLoading = false;
    // Applies a star to the row already on screen rather than reloading the
    // conversation, which would move it under the reader.
    void applyStarToOpenConversation(const QString &messageId, bool starred);
    QVariantList m_searchResults;
    QVariantList m_starredMessages;
    bool m_starredMessagesLoading = false;
    quint64 m_starredRequestGeneration = 0;
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
    // Paired with m_nextBefore: messages sharing a timestamp are only ordered
    // by id, so the cursor needs both halves to page past them.
    QString m_nextBeforeId;
    QSet<QString> m_requestedHistoryBoundaries;
    bool m_waitingRemoteHistory = false;
    QString m_pendingCopyImageId;
    QSet<QString> m_requestedMedia;
	QSet<QString> m_playedMedia;
    QSet<QString> m_requestedLinkPreviews;
    QSet<QString> m_requestedStatusAvatars;
    QSet<QString> m_pendingChatAvatars;
    QHash<QString, qint64> m_chatAvatarRequestedAt;
    QSet<QString> m_requestedStatusMedia;
    // How many conversations one sidebar page holds, and where that listing
    // has got to.
    static constexpr int chatPageSize = 200;
    // The most the daemon will return in one answer.
    static constexpr int chatListCeiling = 500;
    bool m_loadingChats = false;
    bool m_moreChats = false;
    // How many messages the open conversation had loaded before a refresh, so
    // the pages the reader had scrolled through can be restored.
    int m_restoreTarget = 0;
    QStringList m_mediaQueue;
    int m_mediaInFlight = 0;
};
