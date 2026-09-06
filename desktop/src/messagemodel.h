#pragma once

#include <QAbstractListModel>
#include <QHash>
#include <QStringList>
#include <QVariantList>
#include <QVariantMap>

// MessageListModel holds one conversation's messages.
//
// The conversation used to be a plain list property that was replaced whenever
// anything changed. Replacing it rebuilt every delegate, so the view lost its
// position, reloaded images, and re-triggered the "load older messages" edge.
// Storage remains chronological for playback and history paging, while the
// rows exposed to QML are newest-first. A BottomToTop ListView can therefore
// keep row zero at the composer and grow older history away from the reader.
// This avoids Qt's variable-height content estimate moving the visible rows.

// nextAudioAfter returns the recording that follows messageId. Deleted and
// system messages are stepped over because they are not part of what the
// reader is listening to; anything else ends the run, so playback stops where
// the talking stops.
QVariantMap nextAudioAfter(const QVariantList &messages, const QString &messageId);

// ChatListModel keeps the sidebar's model object stable while chat metadata
// changes. A reset is visible in ListView as a jump to its origin, so sync()
// reports inserts, moves, removals, and data changes individually.
class ChatListModel final : public QAbstractListModel
{
    Q_OBJECT
    Q_PROPERTY(int count READ count NOTIFY countChanged)

public:
    explicit ChatListModel(QObject *parent = nullptr);

    enum Role { ChatRole = Qt::UserRole + 1 };

    int rowCount(const QModelIndex &parent = QModelIndex()) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

    void sync(const QVariantList &chats);
    void clear();
    int count() const { return static_cast<int>(m_chats.size()); }
    QVariantMap at(int row) const;

signals:
    void countChanged();

private:
    QVariantList m_chats;
};

class MessageListModel final : public QAbstractListModel
{
    Q_OBJECT
    Q_PROPERTY(int count READ count NOTIFY countChanged)

public:
    explicit MessageListModel(QObject *parent = nullptr);

    enum Role { MessageRole = Qt::UserRole + 1 };

    int rowCount(const QModelIndex &parent = QModelIndex()) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

    // Replaces the conversation, for example when another chat is opened.
    void reset(const QVariantList &messages);
    // Adds an older page before the chronological storage and after the
    // existing newest-first view rows.
    void prepend(const QVariantList &older);
    // Updates a message in place, or appends it when it is new.
    void upsert(const QVariantMap &message);
    // Applies a delivery receipt to messages already on screen. A receipt
    // never moves a message backwards: a read message that is reported as
    // merely delivered stays read.
    void applyReceipt(const QStringList &messageIds, const QString &status, qint64 timestamp = 0);
    void clear();

    bool isEmpty() const { return m_messages.isEmpty(); }
    int count() const { return static_cast<int>(m_messages.size()); }
    QVariantMap at(int row) const;
    QVariantMap oldest() const;
    // lastOwnEditableMessage is the message the composer's Up arrow edits: the
    // newest one this account sent that an edit can still reach. Anything
    // received, deleted, or not text is stepped over, the way the reader's own
    // Edit item is only offered on those messages.
    Q_INVOKABLE QVariantMap lastOwnEditableMessage() const;
    QVariantList items() const { return m_messages; }
    int viewRowForId(const QString &messageId) const;
    // The message with this id, or an empty map. Callers that hold an id must
    // use this rather than mixing the view's row numbers with the storage's.
    QVariantMap byId(const QString &messageId) const;

signals:
    void countChanged();
    // Emitted when a message is added at the chronological end / view row zero,
    // so a view that is already at the composer can follow the conversation.
    void appended();

private:
    void rebuildIndex();
    // Marks the message that opens each calendar day, so the conversation can
    // put a date between one day and the next the way WhatsApp Web does. A
    // page of older history changes which message opens its day, so this runs
    // again after every change.
    void refreshDayStarts();

    QVariantList m_messages;
    QHash<QString, int> m_rowById;
};
