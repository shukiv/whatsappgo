#include "composermentions.h"
#include <QQuickTextDocument>
#include <QRegularExpression>
#include <QTextBlock>
#include <QTextCursor>
#include <QTextFragment>

namespace {
constexpr int MentionJid = QTextFormat::UserProperty + 501;
constexpr int MentionLabel = QTextFormat::UserProperty + 502;
bool validJid(const QString &jid)
{
    static const QRegularExpression pattern(QStringLiteral("^[0-9]+@(s\\.whatsapp\\.net|lid)$"));
    return pattern.match(jid).hasMatch();
}
}

void ComposerMentions::setEditor(QQuickItem *editor)
{
    if (m_editor == editor) return;
    if (m_document) disconnect(m_document, nullptr, this, nullptr);
    m_editor = editor;
    auto *quick = editor ? editor->property("textDocument").value<QQuickTextDocument *>() : nullptr;
    m_document = quick ? quick->textDocument() : nullptr;
    if (m_document) connect(m_document, &QTextDocument::contentsChanged, this, &ComposerMentions::rangesChanged, Qt::QueuedConnection);
    emit editorChanged();
    emit rangesChanged();
}

QVariantList ComposerMentions::ranges() const
{
    QVariantList result;
    if (!m_document) return result;
    for (auto block = m_document->begin(); block.isValid(); block = block.next()) {
        for (auto it = block.begin(); !it.atEnd(); ++it) {
            const auto fragment = it.fragment();
            const auto format = fragment.charFormat();
            const auto jid = format.stringProperty(MentionJid);
            const auto label = format.stringProperty(MentionLabel);
            // Any edit to the marked text invalidates its identity. Native
            // undo restores both the original text and its document properties.
            if (validJid(jid) && label.startsWith(QLatin1Char('@')) && fragment.text() == label)
                result.append(QVariantMap{{QStringLiteral("start"), fragment.position()},
                    {QStringLiteral("length"), fragment.length()}, {QStringLiteral("jid"), jid},
                    {QStringLiteral("name"), label.mid(1)}});
        }
    }
    return result;
}

bool ComposerMentions::insert(int start, int end, const QString &jid, const QString &name)
{
    if (!m_document || !m_editor || !validJid(jid) || name.trimmed().isEmpty()
        || start < 0 || end < start || end > m_document->toPlainText().size()) return false;
    const auto codepoints = name.simplified().toUcs4();
    const auto label = QLatin1Char('@') + QString::fromUcs4(codepoints.constData(), qMin(qsizetype(128), codepoints.size()));
    QTextCursor cursor(m_document);
    cursor.setPosition(start);
    cursor.setPosition(end, QTextCursor::KeepAnchor);
    QTextCharFormat format;
    format.setProperty(MentionJid, jid);
    format.setProperty(MentionLabel, label);
    cursor.beginEditBlock();
    cursor.insertText(label, format);
    cursor.insertText(QStringLiteral(" "), QTextCharFormat{});
    cursor.endEditBlock();
    m_editor->setProperty("cursorPosition", cursor.position());
    return true;
}

void ComposerMentions::restore(const QVariantList &saved)
{
    if (!m_document) return;
    const auto text = m_document->toPlainText();
    QTextCursor cursor(m_document);
    cursor.beginEditBlock();
    cursor.select(QTextCursor::Document);
    cursor.setCharFormat(QTextCharFormat{});
    int previousEnd = 0;
    for (const auto &value : saved) {
        const auto range = value.toMap();
        const int start = range.value(QStringLiteral("start")).toInt();
        const auto jid = range.value(QStringLiteral("jid")).toString();
        const auto label = QLatin1Char('@') + range.value(QStringLiteral("name")).toString();
        if (start < previousEnd || label.size() <= 1 || !validJid(jid)
            || text.mid(start, label.size()) != label) continue;
        cursor.setPosition(start);
        cursor.setPosition(start + label.size(), QTextCursor::KeepAnchor);
        QTextCharFormat format;
        format.setProperty(MentionJid, jid);
        format.setProperty(MentionLabel, label);
        cursor.setCharFormat(format);
        previousEnd = start + label.size();
    }
    cursor.endEditBlock();
    // Restored drafts are a fresh editor revision, not an undo path into a
    // different conversation's names or text.
    m_document->clearUndoRedoStacks();
}

QVariantMap ComposerMentions::outgoing() const
{
    QString text = m_document ? m_document->toPlainText() : QString();
    const auto spans = ranges();
    QVariantList mentions;
    QStringList seen;
    for (qsizetype i = spans.size(); i-- > 0;) {
        const auto span = spans.at(i).toMap();
        const auto jid = span.value(QStringLiteral("jid")).toString();
        text.replace(span.value(QStringLiteral("start")).toInt(), span.value(QStringLiteral("length")).toInt(),
                     QLatin1Char('@') + jid.section(QLatin1Char('@'), 0, 0));
        if (!seen.contains(jid)) {
            mentions.prepend(QVariantMap{{QStringLiteral("jid"), jid}, {QStringLiteral("name"), span.value(QStringLiteral("name"))}});
            seen.append(jid);
        }
    }
    return {{QStringLiteral("text"), text}, {QStringLiteral("mentions"), mentions}};
}
