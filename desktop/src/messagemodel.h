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
// Reporting insertions precisely lets the view keep the rows it already shows:
// prepending an older page leaves the reader where they were.

// nextAudioAfter returns the recording that follows messageId. Deleted and
// system messages are stepped over because they are not part of what the
// reader is listening to; anything else ends the run, so playback stops where
// the talking stops.
QVariantMap nextAudioAfter(const QVariantList &messages, const QString &messageId);

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
    // Adds an older page above the current contents.
    void prepend(const QVariantList &older);
    // Updates a message in place, or appends it when it is new.
    void upsert(const QVariantMap &message);
    // Applies a delivery receipt to messages already on screen. A receipt
    // never moves a message backwards: a read message that is reported as
    // merely delivered stays read.
    void applyReceipt(const QStringList &messageIds, const QString &status);
    void clear();

    bool isEmpty() const { return m_messages.isEmpty(); }
    int count() const { return static_cast<int>(m_messages.size()); }
    QVariantMap at(int row) const;
    QVariantMap oldest() const;
    QVariantList items() const { return m_messages; }

signals:
    void countChanged();
    // Emitted when a message is added at the end, so a view that is already at
    // the bottom can follow the conversation.
    void appended();

private:
    void rebuildIndex();

    QVariantList m_messages;
    QHash<QString, int> m_rowById;
};
