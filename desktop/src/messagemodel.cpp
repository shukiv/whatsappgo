#include "messagemodel.h"

#include <QDateTime>
#include <QImageReader>

namespace {
QString messageId(const QVariantMap &message)
{
    return message.value(QStringLiteral("id")).toString();
}

QVariantMap withPreviewDimensions(QVariantMap message)
{
    if (message.value(QStringLiteral("preview_width")).toInt() > 0
        && message.value(QStringLiteral("preview_height")).toInt() > 0)
        return message;

    const auto kind = message.value(QStringLiteral("kind")).toString();
    QString previewPath;
    if (kind == QStringLiteral("image") || kind == QStringLiteral("sticker"))
        previewPath = message.value(QStringLiteral("media_path")).toString();
    if (previewPath.isEmpty()
        && (kind == QStringLiteral("image") || kind == QStringLiteral("video")
            || kind == QStringLiteral("sticker")))
        previewPath = message.value(QStringLiteral("media_thumbnail")).toString();
    if (previewPath.isEmpty())
        return message;

    QImageReader reader(previewPath);
    const auto size = reader.size();
    if (size.isValid() && !size.isEmpty()) {
        message.insert(QStringLiteral("preview_width"), size.width());
        message.insert(QStringLiteral("preview_height"), size.height());
    }
    return message;
}
}

QVariantMap nextAudioAfter(const QVariantList &messages, const QString &messageId)
{
    if (messageId.isEmpty())
        return {};
    int index = -1;
    for (int row = 0; row < messages.size(); ++row) {
        if (messages.at(row).toMap().value(QStringLiteral("id")).toString() == messageId) {
            index = row;
            break;
        }
    }
    if (index < 0)
        return {};
    for (int row = index + 1; row < messages.size(); ++row) {
        const auto candidate = messages.at(row).toMap();
        const auto kind = candidate.value(QStringLiteral("kind")).toString();
        if (candidate.value(QStringLiteral("revoked")).toBool() || kind == QStringLiteral("system"))
            continue;
        if (kind == QStringLiteral("audio"))
            return candidate;
        return {};
    }
    return {};
}

ChatListModel::ChatListModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

int ChatListModel::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : count();
}

QVariant ChatListModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() < 0 || index.row() >= count() || role != ChatRole)
        return QVariant();
    return m_chats.at(index.row());
}

QHash<int, QByteArray> ChatListModel::roleNames() const
{
    return {{ChatRole, QByteArrayLiteral("modelData")}};
}

void ChatListModel::sync(const QVariantList &chats)
{
    const int previousCount = count();
    const auto jidFor = [](const QVariant &entry) {
        return entry.toMap().value(QStringLiteral("jid")).toString();
    };

    for (int targetRow = 0; targetRow < chats.size(); ++targetRow) {
        const auto target = chats.at(targetRow).toMap();
        const auto targetJid = target.value(QStringLiteral("jid")).toString();
        int existingRow = -1;
        for (int candidate = targetRow; candidate < m_chats.size(); ++candidate) {
            if (!targetJid.isEmpty() && jidFor(m_chats.at(candidate)) == targetJid) {
                existingRow = candidate;
                break;
            }
        }

        if (existingRow < 0) {
            beginInsertRows(QModelIndex(), targetRow, targetRow);
            m_chats.insert(targetRow, target);
            endInsertRows();
        } else if (existingRow != targetRow) {
            beginMoveRows(QModelIndex(), existingRow, existingRow, QModelIndex(), targetRow);
            m_chats.move(existingRow, targetRow);
            endMoveRows();
        }

        if (m_chats.at(targetRow).toMap() != target) {
            m_chats[targetRow] = target;
            const auto changed = index(targetRow, 0);
            emit dataChanged(changed, changed, {ChatRole});
        }
    }

    if (m_chats.size() > chats.size()) {
        const int firstRemoved = static_cast<int>(chats.size());
        const int lastRemoved = count() - 1;
        beginRemoveRows(QModelIndex(), firstRemoved, lastRemoved);
        while (m_chats.size() > chats.size())
            m_chats.removeLast();
        endRemoveRows();
    }
    if (count() != previousCount)
        emit countChanged();
}

void ChatListModel::clear()
{
    if (m_chats.isEmpty())
        return;
    beginRemoveRows(QModelIndex(), 0, count() - 1);
    m_chats.clear();
    endRemoveRows();
    emit countChanged();
}

QVariantMap ChatListModel::at(int row) const
{
    if (row < 0 || row >= count())
        return {};
    return m_chats.at(row).toMap();
}

MessageListModel::MessageListModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

int MessageListModel::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : count();
}

QVariant MessageListModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() < 0 || index.row() >= count() || role != MessageRole)
        return QVariant();
    return m_messages.at(count() - 1 - index.row());
}

QHash<int, QByteArray> MessageListModel::roleNames() const
{
    // The single role is named so that delegates keep receiving the message as
    // "modelData", exactly as they did with the previous list property.
    return {{MessageRole, QByteArrayLiteral("modelData")}};
}

void MessageListModel::rebuildIndex()
{
    m_rowById.clear();
    m_rowById.reserve(count());
    for (int row = 0; row < count(); ++row)
        m_rowById.insert(messageId(m_messages.at(row).toMap()), row);
}

namespace {
// dayOf is the local calendar day a message belongs to. The separator names a
// day in the reader's own timezone, which is where "today" is decided.
QDate dayOf(const QVariantMap &message)
{
    const auto timestamp = message.value(QStringLiteral("timestamp")).toLongLong();
    if (timestamp <= 0)
        return {};
    return QDateTime::fromMSecsSinceEpoch(timestamp).date();
}
}

void MessageListModel::refreshDayStarts()
{
    QDate previous;
    for (int row = 0; row < count(); ++row) {
        auto message = m_messages.at(row).toMap();
        const auto day = dayOf(message);
        const bool starts = day.isValid() && day != previous;
        if (day.isValid())
            previous = day;
        if (message.value(QStringLiteral("starts_day")).toBool() == starts)
            continue;
        message.insert(QStringLiteral("starts_day"), starts);
        m_messages[row] = message;
        const auto changed = index(count() - 1 - row, 0);
        emit dataChanged(changed, changed, {MessageRole});
    }
}

void MessageListModel::reset(const QVariantList &messages)
{
    beginResetModel();
    m_messages.clear();
    m_messages.reserve(messages.size());
    for (const auto &message : messages)
        m_messages.append(withPreviewDimensions(message.toMap()));
    rebuildIndex();
    refreshDayStarts();
    endResetModel();
    emit countChanged();
}

void MessageListModel::prepend(const QVariantList &older)
{
    if (older.isEmpty())
        return;
    const int firstViewRow = count();
    beginInsertRows(QModelIndex(), firstViewRow,
                    firstViewRow + static_cast<int>(older.size()) - 1);
    QVariantList combined;
    combined.reserve(older.size() + m_messages.size());
    for (const auto &message : older)
        combined.append(withPreviewDimensions(message.toMap()));
    combined.append(m_messages);
    m_messages = combined;
    rebuildIndex();
    endInsertRows();
    // The message that used to open the conversation may now be in the middle
    // of a day, so this runs after the insert rather than inside it.
    refreshDayStarts();
    emit countChanged();
}

void MessageListModel::upsert(const QVariantMap &message)
{
    const auto prepared = withPreviewDimensions(message);
    const auto id = messageId(prepared);
    if (id.isEmpty())
        return;
    const auto existing = m_rowById.constFind(id);
    if (existing != m_rowById.constEnd()) {
        const int row = existing.value();
        if (row < 0 || row >= count())
            return;
        // The day mark is the model's own working, not part of the message the
        // daemon sends. Replacing the row with what arrived would drop it, and
        // the date pill above that message with it.
        const auto stored = m_messages.at(row).toMap();
        auto merged = prepared;
        if (stored.contains(QStringLiteral("starts_day")))
            merged.insert(QStringLiteral("starts_day"), stored.value(QStringLiteral("starts_day")));
        if (stored == merged)
            return;
        m_messages[row] = merged;
        const auto changed = index(count() - 1 - row, 0);
        emit dataChanged(changed, changed, {MessageRole});
        // A replacement can carry a different time, which moves where one day
        // ends and the next begins.
        refreshDayStarts();
        return;
    }
    const int row = count();
    beginInsertRows(QModelIndex(), 0, 0);
    m_messages.append(prepared);
    m_rowById.insert(id, row);
    endInsertRows();
    refreshDayStarts();
    emit countChanged();
    emit appended();
}

namespace {
// How far along delivery a status is. Receipts can arrive out of order, and a
// later "delivered" must not undo an earlier "read".
int receiptRank(const QString &status)
{
    if (status == QStringLiteral("played"))
        return 4;
    if (status == QStringLiteral("read"))
        return 3;
    if (status == QStringLiteral("delivered"))
        return 2;
    if (status == QStringLiteral("sent"))
        return 1;
    return 0;
}
}

void MessageListModel::applyReceipt(const QStringList &messageIds, const QString &status, qint64 timestamp)
{
    const int rank = receiptRank(status);
    if (rank == 0)
        return;
    for (const auto &id : messageIds) {
        const auto found = m_rowById.constFind(id);
        if (found == m_rowById.constEnd())
            continue;
        const int row = found.value();
        if (row < 0 || row >= count())
            continue;
        auto message = m_messages.at(row).toMap();
		const auto original = message;
        const auto currentRank = receiptRank(message.value(QStringLiteral("status")).toString());
        if (currentRank < rank)
            message.insert(QStringLiteral("status"), status);
        if (timestamp > 0) {
            if (status == QStringLiteral("delivered")) {
                message.insert(QStringLiteral("delivered_at"), timestamp);
            } else if (status == QStringLiteral("read")) {
                if (message.value(QStringLiteral("delivered_at")).toLongLong() <= 0)
                    message.insert(QStringLiteral("delivered_at"), timestamp);
                message.insert(QStringLiteral("read_at"), timestamp);
            } else if (status == QStringLiteral("played")) {
                if (message.value(QStringLiteral("delivered_at")).toLongLong() <= 0)
                    message.insert(QStringLiteral("delivered_at"), timestamp);
                if (message.value(QStringLiteral("read_at")).toLongLong() <= 0)
                    message.insert(QStringLiteral("read_at"), timestamp);
                message.insert(QStringLiteral("played_at"), timestamp);
            }
        }
        if (message == original)
            continue;
        m_messages[row] = message;
        const auto changed = index(count() - 1 - row, 0);
        emit dataChanged(changed, changed, {MessageRole});
    }
}

void MessageListModel::clear()
{
    if (m_messages.isEmpty())
        return;
    beginResetModel();
    m_messages.clear();
    m_rowById.clear();
    endResetModel();
    emit countChanged();
}

QVariantMap MessageListModel::at(int row) const
{
    if (row < 0 || row >= count())
        return {};
    return m_messages.at(row).toMap();
}

QVariantMap MessageListModel::oldest() const
{
    return m_messages.isEmpty() ? QVariantMap{} : m_messages.constFirst().toMap();
}

QVariantMap MessageListModel::lastOwnEditableMessage() const
{
    for (auto row = m_messages.crbegin(); row != m_messages.crend(); ++row) {
        const auto message = row->toMap();
        if (!message.value(QStringLiteral("from_me")).toBool())
            continue;
        if (message.value(QStringLiteral("revoked")).toBool())
            continue;
        if (message.value(QStringLiteral("kind")).toString() != QStringLiteral("text"))
            continue;
        return message;
    }
    return {};
}

QVariantMap MessageListModel::byId(const QString &id) const
{
    const auto found = m_rowById.constFind(id);
    if (found == m_rowById.constEnd() || found.value() < 0 || found.value() >= count())
        return {};
    return m_messages.at(found.value()).toMap();
}

int MessageListModel::viewRowForId(const QString &id) const
{
    const auto found = m_rowById.constFind(id);
    if (found == m_rowById.constEnd())
        return -1;
    return count() - 1 - found.value();
}
