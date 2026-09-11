#pragma once

#include <QObject>
#include <QPointer>
#include <QQuickItem>
#include <QTextDocument>
#include <QVariantList>
#include <QtQml/qqmlregistration.h>

// Mention identity lives in the document's undoable character properties,
// never inferred from a name the user happened to type or paste.
class ComposerMentions : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(QQuickItem *editor READ editor WRITE setEditor NOTIFY editorChanged)
    Q_PROPERTY(QVariantList ranges READ ranges WRITE restore NOTIFY rangesChanged)
public:
    explicit ComposerMentions(QObject *parent = nullptr) : QObject(parent) {}
    QQuickItem *editor() const { return m_editor; }
    void setEditor(QQuickItem *editor);
    QVariantList ranges() const;
    void restore(const QVariantList &ranges);
    Q_INVOKABLE bool insert(int start, int end, const QString &jid, const QString &name);
    Q_INVOKABLE QVariantMap outgoing() const;
signals:
    void editorChanged();
    void rangesChanged();
private:
    QPointer<QQuickItem> m_editor;
    QPointer<QTextDocument> m_document;
};
