#include "messagemodel.h"

#include <QCoreApplication>
#include <QDebug>
#include <QImage>
#include <QTemporaryDir>
#include <QVariantMap>

namespace {
QVariantMap message(const QString &id, const QString &body = QStringLiteral("hello"))
{
    return {{QStringLiteral("id"), id}, {QStringLiteral("body"), body}};
}
}

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    MessageListModel model;
    ChatListModel chats;

    bool passed = true;
    const auto require = [&passed](bool condition, const QString &description) {
        if (!condition) {
            passed = false;
            qInfo().noquote() << QStringLiteral("FAIL: ") + description;
        }
    };

    int chatResets = 0;
    int chatMoves = 0;
    int chatChanges = 0;
    QObject::connect(&chats, &QAbstractItemModel::modelReset, &app, [&chatResets] { ++chatResets; });
    QObject::connect(&chats, &QAbstractItemModel::rowsMoved, &app,
                     [&chatMoves](const QModelIndex &, int, int, const QModelIndex &, int) { ++chatMoves; });
    QObject::connect(&chats, &QAbstractItemModel::dataChanged, &app,
                     [&chatChanges](const QModelIndex &, const QModelIndex &, const QList<int> &) { ++chatChanges; });
    const auto chat = [](const QString &jid, const QString &preview) {
        return QVariantMap{{QStringLiteral("jid"), jid},
                           {QStringLiteral("last_message_preview"), preview}};
    };
    chats.sync({chat(QStringLiteral("a@lid"), QStringLiteral("A")),
                chat(QStringLiteral("b@lid"), QStringLiteral("B")),
                chat(QStringLiteral("c@lid"), QStringLiteral("C"))});
    chats.sync({chat(QStringLiteral("c@lid"), QStringLiteral("C updated")),
                chat(QStringLiteral("a@lid"), QStringLiteral("A")),
                chat(QStringLiteral("b@lid"), QStringLiteral("B"))});
    require(chatResets == 0, QStringLiteral("refreshing sidebar chats must not reset the model"));
    require(chatMoves == 1, QStringLiteral("a reordered chat is moved instead of rebuilding every row"));
    require(chatChanges == 1, QStringLiteral("changed chat metadata updates only its row"));
    require(chats.at(0).value(QStringLiteral("jid")).toString() == QStringLiteral("c@lid"),
            QStringLiteral("chat synchronization preserves server ordering"));

    int appended = 0;
    int resets = 0;
    QList<QPair<int, int>> insertions;
    QList<int> changedRows;
    QObject::connect(&model, &MessageListModel::appended, &app, [&appended] { ++appended; });
    QObject::connect(&model, &QAbstractItemModel::modelReset, &app, [&resets] { ++resets; });
    QObject::connect(&model, &QAbstractItemModel::rowsInserted, &app,
                     [&insertions](const QModelIndex &, int first, int last) { insertions.append({first, last}); });
    QObject::connect(&model, &QAbstractItemModel::dataChanged, &app,
                     [&changedRows](const QModelIndex &topLeft, const QModelIndex &, const QList<int> &) {
                         changedRows.append(topLeft.row());
                     });

    // Delegates receive the message as "modelData", the same name the previous
    // list property provided.
    require(model.roleNames().value(MessageListModel::MessageRole) == QByteArrayLiteral("modelData"),
            QStringLiteral("the single role is named modelData"));

    model.reset({message(QStringLiteral("a")), message(QStringLiteral("b"))});
    require(model.count() == 2, QStringLiteral("reset stores the conversation"));
    require(resets == 1, QStringLiteral("reset is reported once"));

    // QML sees newest-first rows for its BottomToTop list. Older history is
    // inserted after the existing view rows, away from the composer edge.
    model.prepend({message(QStringLiteral("x")), message(QStringLiteral("y"))});
    require(model.count() == 4, QStringLiteral("prepend adds the older page"));
    require(resets == 1, QStringLiteral("prepend does not reset the model"));
    require(insertions.size() == 1 && insertions.at(0).first == 2 && insertions.at(0).second == 3,
            QStringLiteral("older history is reported after the visible view rows"));
    require(model.at(0).value(QStringLiteral("id")).toString() == QStringLiteral("x"),
            QStringLiteral("the older page is above the newer messages"));
    require(model.oldest().value(QStringLiteral("id")).toString() == QStringLiteral("x"),
            QStringLiteral("oldest() returns the first message"));
    require(appended == 0, QStringLiteral("an older page is not an arriving message"));

    model.prepend({});
    require(insertions.size() == 1, QStringLiteral("an empty page changes nothing"));

    auto edited = message(QStringLiteral("b"), QStringLiteral("edited"));
    model.upsert(edited);
    require(insertions.size() == 1, QStringLiteral("editing a message inserts no row"));
    require(changedRows.size() == 1 && changedRows.at(0) == 0, QStringLiteral("the edited view row is reported"));
    require(model.at(3).value(QStringLiteral("body")).toString() == QStringLiteral("edited"),
            QStringLiteral("the edit is applied in place"));

    // Receipts repeat the same message; re-emitting would reload its image.
    model.upsert(edited);
    require(changedRows.size() == 1, QStringLiteral("an identical update is ignored"));

    model.upsert(message(QStringLiteral("c")));
    require(model.count() == 5, QStringLiteral("a new message is appended"));
    require(appended == 1, QStringLiteral("appending is announced so the view can follow"));
    require(insertions.size() == 2 && insertions.at(1).first == 0,
            QStringLiteral("the newest message is inserted at the composer edge"));

    model.upsert({});
    require(model.count() == 5, QStringLiteral("a message without an id is rejected"));

    require(model.data(model.index(0, 0), MessageListModel::MessageRole).toMap().value(QStringLiteral("id")).toString()
                == QStringLiteral("c"),
            QStringLiteral("view row zero is the newest message"));
    require(model.viewRowForId(QStringLiteral("x")) == 4
                && model.viewRowForId(QStringLiteral("c")) == 0,
            QStringLiteral("message ids map to newest-first view rows"));
    require(!model.data(model.index(99, 0), MessageListModel::MessageRole).isValid(),
            QStringLiteral("an out-of-range row has no data"));
    require(model.rowCount(model.index(0, 0)) == 0, QStringLiteral("the model is flat"));

    // Media dimensions must be known before QML creates a recycled delegate.
    // Otherwise every thumbnail starts at a guessed height and resizes after
    // decoding, making the conversation jump while it is being scrolled.
    QTemporaryDir mediaDirectory;
    const auto thumbnailPath = mediaDirectory.filePath(QStringLiteral("video-poster.jpg"));
    QImage thumbnail(640, 360, QImage::Format_RGB32);
    thumbnail.fill(Qt::green);
    require(thumbnail.save(thumbnailPath), QStringLiteral("the media-dimension fixture could not be written"));
    model.reset({QVariantMap{
        {QStringLiteral("id"), QStringLiteral("video-dimensions")},
        {QStringLiteral("kind"), QStringLiteral("video")},
        {QStringLiteral("media_thumbnail"), thumbnailPath},
    }});
    require(model.at(0).value(QStringLiteral("preview_width")).toInt() == 640
                && model.at(0).value(QStringLiteral("preview_height")).toInt() == 360,
            QStringLiteral("cached video-poster dimensions were not attached before delegate creation"));

    model.clear();
    require(model.count() == 0 && model.isEmpty(), QStringLiteral("clear empties the conversation"));
    require(model.oldest().isEmpty(), QStringLiteral("an empty conversation has no oldest message"));

    // Receipts land on the messages already on screen and never go backwards.
    model.reset({
        QVariantMap{{QStringLiteral("id"), QStringLiteral("r1")}, {QStringLiteral("status"), QStringLiteral("sent")}},
        QVariantMap{{QStringLiteral("id"), QStringLiteral("r2")}, {QStringLiteral("status"), QStringLiteral("read")}},
    });
    changedRows.clear();
    model.applyReceipt({QStringLiteral("r1"), QStringLiteral("r2"), QStringLiteral("absent")}, QStringLiteral("delivered"));
    require(model.at(0).value(QStringLiteral("status")).toString() == QStringLiteral("delivered"),
            QStringLiteral("a sent message was not marked delivered"));
    require(model.at(1).value(QStringLiteral("status")).toString() == QStringLiteral("read"),
            QStringLiteral("a read message was pushed back to delivered"));
    require(changedRows.size() == 1 && changedRows.at(0) == 1,
            QStringLiteral("only the message that changed should be reported"));

    model.applyReceipt({QStringLiteral("r1")}, QStringLiteral("read"), 4000);
    require(model.at(0).value(QStringLiteral("status")).toString() == QStringLiteral("read"),
            QStringLiteral("a delivered message was not marked read"));
	require(model.at(0).value(QStringLiteral("delivered_at")).toLongLong() == 4000
				&& model.at(0).value(QStringLiteral("read_at")).toLongLong() == 4000,
			QStringLiteral("a read receipt did not retain its milestone timestamps"));
	model.applyReceipt({QStringLiteral("r1")}, QStringLiteral("played"), 5000);
	require(model.at(0).value(QStringLiteral("played_at")).toLongLong() == 5000,
			QStringLiteral("a played receipt did not retain its timestamp"));
    changedRows.clear();
    model.applyReceipt({QStringLiteral("r1")}, QStringLiteral("nonsense"));
    require(changedRows.isEmpty() && model.at(0).value(QStringLiteral("status")).toString() == QStringLiteral("played"),
            QStringLiteral("an unknown receipt should change nothing"));

    // A run of voice notes plays through; anything else ends it.
    const auto voice = [](const QString &id) {
        return QVariantMap{{QStringLiteral("id"), id}, {QStringLiteral("kind"), QStringLiteral("audio")}};
    };
    const auto text = [](const QString &id) {
        return QVariantMap{{QStringLiteral("id"), id}, {QStringLiteral("kind"), QStringLiteral("text")}};
    };
    const QVariantList conversation{
        voice(QStringLiteral("v1")),
        voice(QStringLiteral("v2")),
        QVariantMap{{QStringLiteral("id"), QStringLiteral("gone")}, {QStringLiteral("kind"), QStringLiteral("text")},
                    {QStringLiteral("revoked"), true}},
        voice(QStringLiteral("v3")),
        text(QStringLiteral("t1")),
        voice(QStringLiteral("v4")),
    };
    require(nextAudioAfter(conversation, QStringLiteral("v1")).value(QStringLiteral("id")).toString() == QStringLiteral("v2"),
            QStringLiteral("the next recording does not follow the current one"));
    require(nextAudioAfter(conversation, QStringLiteral("v2")).value(QStringLiteral("id")).toString() == QStringLiteral("v3"),
            QStringLiteral("a deleted message should be stepped over"));
    require(nextAudioAfter(conversation, QStringLiteral("v3")).isEmpty(),
            QStringLiteral("playback should stop when the conversation returns to text"));
    require(nextAudioAfter(conversation, QStringLiteral("v4")).isEmpty(),
            QStringLiteral("the last recording has nothing after it"));
    require(nextAudioAfter(conversation, QStringLiteral("missing")).isEmpty(),
            QStringLiteral("an unknown message has no successor"));
    require(nextAudioAfter(conversation, QString()).isEmpty(),
            QStringLiteral("an empty id has no successor"));

    // Up on an empty composer edits the last thing this account said, so the
    // model has to walk back past everything an edit cannot reach.
    const auto sent = [](const QString &id, const QString &body) {
        return QVariantMap{{QStringLiteral("id"), id}, {QStringLiteral("kind"), QStringLiteral("text")},
                           {QStringLiteral("from_me"), true}, {QStringLiteral("body"), body}};
    };
    MessageListModel editable;
    require(editable.lastOwnEditableMessage().isEmpty(),
            QStringLiteral("an empty conversation offered something to edit"));
    editable.reset(QVariantList{
        sent(QStringLiteral("mine-old"), QStringLiteral("first")),
        sent(QStringLiteral("mine-new"), QStringLiteral("second")),
        QVariantMap{{QStringLiteral("id"), QStringLiteral("theirs")}, {QStringLiteral("kind"), QStringLiteral("text")},
                    {QStringLiteral("from_me"), false}, {QStringLiteral("body"), QStringLiteral("reply")}},
        QVariantMap{{QStringLiteral("id"), QStringLiteral("mine-photo")}, {QStringLiteral("kind"), QStringLiteral("image")},
                    {QStringLiteral("from_me"), true}},
        QVariantMap{{QStringLiteral("id"), QStringLiteral("mine-gone")}, {QStringLiteral("kind"), QStringLiteral("text")},
                    {QStringLiteral("from_me"), true}, {QStringLiteral("revoked"), true},
                    {QStringLiteral("body"), QStringLiteral("deleted")}},
    });
    const auto candidate = editable.lastOwnEditableMessage();
    require(candidate.value(QStringLiteral("id")).toString() == QStringLiteral("mine-new"),
            QStringLiteral("Up offered %1 rather than the newest message an edit can reach")
                .arg(candidate.value(QStringLiteral("id")).toString()));
    require(candidate.value(QStringLiteral("body")).toString() == QStringLiteral("second"),
            QStringLiteral("the message came back without the text to edit"));

    MessageListModel received;
    received.reset(QVariantList{
        QVariantMap{{QStringLiteral("id"), QStringLiteral("theirs")}, {QStringLiteral("kind"), QStringLiteral("text")},
                    {QStringLiteral("from_me"), false}},
    });
    require(received.lastOwnEditableMessage().isEmpty(),
            QStringLiteral("a conversation this account has not spoken in offered somebody else's message"));

    return passed ? EXIT_SUCCESS : EXIT_FAILURE;
}
