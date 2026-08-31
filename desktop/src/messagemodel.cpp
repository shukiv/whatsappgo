#include "messagemodel.h"

namespace {
QString messageId(const QVariantMap &message)
{
    return message.value(QStringLiteral("id")).toString();
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
    return m_messages.at(index.row());
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

void MessageListModel::reset(const QVariantList &messages)
{
    beginResetModel();
    m_messages = messages;
    rebuildIndex();
    endResetModel();
    emit countChanged();
}

void MessageListModel::prepend(const QVariantList &older)
{
    if (older.isEmpty())
        return;
    beginInsertRows(QModelIndex(), 0, static_cast<int>(older.size()) - 1);
    QVariantList combined = older;
    combined.append(m_messages);
    m_messages = combined;
    rebuildIndex();
    endInsertRows();
    emit countChanged();
}

void MessageListModel::upsert(const QVariantMap &message)
{
    const auto id = messageId(message);
    if (id.isEmpty())
        return;
    const auto existing = m_rowById.constFind(id);
    if (existing != m_rowById.constEnd()) {
        const int row = existing.value();
        if (row < 0 || row >= count())
            return;
        if (m_messages.at(row).toMap() == message)
            return;
        m_messages[row] = message;
        const auto changed = index(row, 0);
        emit dataChanged(changed, changed, {MessageRole});
        return;
    }
    const int row = count();
    beginInsertRows(QModelIndex(), row, row);
    m_messages.append(message);
    m_rowById.insert(id, row);
    endInsertRows();
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

void MessageListModel::applyReceipt(const QStringList &messageIds, const QString &status)
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
        if (receiptRank(message.value(QStringLiteral("status")).toString()) >= rank)
            continue;
        message.insert(QStringLiteral("status"), status);
        m_messages[row] = message;
        const auto changed = index(row, 0);
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
