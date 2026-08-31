#include "messagemodel.h"

#include <QCoreApplication>
#include <QDebug>
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

    bool passed = true;
    const auto require = [&passed](bool condition, const QString &description) {
        if (!condition) {
            passed = false;
            qInfo().noquote() << QStringLiteral("FAIL: ") + description;
        }
    };

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

    // An older page must be reported as an insertion at the top. A reset here
    // would scroll the reader away from what they were reading.
    model.prepend({message(QStringLiteral("x")), message(QStringLiteral("y"))});
    require(model.count() == 4, QStringLiteral("prepend adds the older page"));
    require(resets == 1, QStringLiteral("prepend does not reset the model"));
    require(insertions.size() == 1 && insertions.at(0).first == 0 && insertions.at(0).second == 1,
            QStringLiteral("prepend is reported as rows 0-1"));
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
    require(changedRows.size() == 1 && changedRows.at(0) == 3, QStringLiteral("the edited row is reported"));
    require(model.at(3).value(QStringLiteral("body")).toString() == QStringLiteral("edited"),
            QStringLiteral("the edit is applied in place"));

    // Receipts repeat the same message; re-emitting would reload its image.
    model.upsert(edited);
    require(changedRows.size() == 1, QStringLiteral("an identical update is ignored"));

    model.upsert(message(QStringLiteral("c")));
    require(model.count() == 5, QStringLiteral("a new message is appended"));
    require(appended == 1, QStringLiteral("appending is announced so the view can follow"));
    require(insertions.size() == 2 && insertions.at(1).first == 4, QStringLiteral("the append is reported at the end"));

    model.upsert({});
    require(model.count() == 5, QStringLiteral("a message without an id is rejected"));

    require(model.data(model.index(4, 0), MessageListModel::MessageRole).toMap().value(QStringLiteral("id")).toString()
                == QStringLiteral("c"),
            QStringLiteral("data() returns the message map"));
    require(!model.data(model.index(99, 0), MessageListModel::MessageRole).isValid(),
            QStringLiteral("an out-of-range row has no data"));
    require(model.rowCount(model.index(0, 0)) == 0, QStringLiteral("the model is flat"));

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
    require(changedRows.size() == 1 && changedRows.at(0) == 0,
            QStringLiteral("only the message that changed should be reported"));

    model.applyReceipt({QStringLiteral("r1")}, QStringLiteral("read"));
    require(model.at(0).value(QStringLiteral("status")).toString() == QStringLiteral("read"),
            QStringLiteral("a delivered message was not marked read"));
    changedRows.clear();
    model.applyReceipt({QStringLiteral("r1")}, QStringLiteral("nonsense"));
    require(changedRows.isEmpty() && model.at(0).value(QStringLiteral("status")).toString() == QStringLiteral("read"),
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

    return passed ? EXIT_SUCCESS : EXIT_FAILURE;
}
