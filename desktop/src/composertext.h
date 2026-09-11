#pragma once

#include <QObject>
#include <QPointer>
#include <QQuickItem>
#include <QSyntaxHighlighter>
#include <QProcess>
#include <QTimer>
#include <QStringList>
#include <QVariantList>
#include <QColor>
#include <QtQml/qqmlregistration.h>

// Presentation-only emoji formatting plus native, undoable emoticon input.
// The editor stays PlainText: drafts, clipboard and RPC all contain Unicode.
class ComposerText : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    Q_PROPERTY(QQuickItem *editor READ editor WRITE setEditor NOTIFY editorChanged)
    Q_PROPERTY(bool spellChecking READ spellChecking WRITE setSpellChecking NOTIFY spellStateChanged)
    Q_PROPERTY(QString spellLanguage READ spellLanguage WRITE setSpellLanguage NOTIFY spellStateChanged)
    Q_PROPERTY(QStringList dictionaries READ dictionaries NOTIFY spellStateChanged)
    Q_PROPERTY(QString spellError READ spellError NOTIFY spellStateChanged)
    Q_PROPERTY(QStringList suggestions READ suggestions NOTIFY suggestionsChanged)
    Q_PROPERTY(QString misspelledWord READ misspelledWord NOTIFY suggestionsChanged)
    Q_PROPERTY(QVariantList mentionRanges READ mentionRanges WRITE setMentionRanges NOTIFY mentionStyleChanged)
    Q_PROPERTY(QColor mentionColor READ mentionColor WRITE setMentionColor NOTIFY mentionStyleChanged)

public:
    explicit ComposerText(QObject *parent = nullptr);
    ~ComposerText() override;
    QQuickItem *editor() const { return m_editor; }
    void setEditor(QQuickItem *editor);
    bool spellChecking() const { return m_spellChecking; }
    void setSpellChecking(bool enabled);
    QString spellLanguage() const { return m_spellLanguage; }
    void setSpellLanguage(const QString &language);
    QStringList dictionaries() const { return m_dictionaries; }
    QString spellError() const { return m_spellError; }
    QStringList suggestions() const { return m_suggestions; }
    QString misspelledWord() const { return m_word; }
    Q_INVOKABLE void suggestAt(int position);
    Q_INVOKABLE void replaceSpelling(const QString &word);
    Q_INVOKABLE void ignoreSpelling();
    Q_INVOKABLE void convertEmoticons(bool all = false);
    QVariantList mentionRanges() const { return m_mentionRanges; }
    QColor mentionColor() const { return m_mentionColor; }
    void setMentionRanges(const QVariantList &ranges);
    void setMentionColor(const QColor &color);

signals:
    void editorChanged();
    void spellStateChanged();
    void suggestionsChanged();
    void mentionStyleChanged();

private:
    QPointer<QQuickItem> m_editor;
    QSyntaxHighlighter *m_highlighter;
    void scheduleSpelling();
    void checkSpelling();
    void startSpellJob(const QString &job, const QStringList &arguments, const QByteArray &input = {});
    void finishSpellJob(int code);
    QString editorText() const;
    QProcess m_spellProcess;
    QTimer m_spellDebounce;
    QTimer m_spellTimeout;
    QString m_aspell;
    QString m_spellJob;
    QString m_spellInput;
    QString m_spellJobLanguage;
    QString m_spellLanguage = QStringLiteral("en_US");
    QStringList m_dictionaries;
    QString m_spellError;
    QStringList m_suggestions;
    QString m_word;
    QString m_suggestionText;
    int m_wordStart = -1;
    bool m_spellChecking = false;
    QVariantList m_mentionRanges;
    QColor m_mentionColor;
};
