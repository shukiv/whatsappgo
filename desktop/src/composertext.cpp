#include "composertext.h"

#include <QCoreApplication>
#include <QHash>
#include <QInputMethodEvent>
#include <QQuickTextDocument>
#include <QRegularExpression>
#include <QTextCharFormat>
#include <QStandardPaths>
#include <QSet>
#include <QTextBlock>
#include <QTextDocument>
#include <utility>

namespace {
// Exclude URLs, addresses, paths and code spans before extracting words. The
// same tokenization is used for checking and display, including mixed RTL.
QList<QRegularExpressionMatch> spellWords(const QString &text)
{
    static const QRegularExpression tokens(QStringLiteral("`[^`]*`|\\S+"));
    static const QRegularExpression words(QStringLiteral("[\\p{L}\\p{M}]+(?:['’][\\p{L}\\p{M}]+)*"));
    QList<QRegularExpressionMatch> result;
    auto tokenMatches = tokens.globalMatch(text);
    while (tokenMatches.hasNext()) {
        const auto token = tokenMatches.next();
        const auto value = token.captured();
        if (value.startsWith(QLatin1Char('`')) || value.contains(QLatin1Char('@')) || value.contains(QLatin1Char('/')) || value.contains(QLatin1Char('\\')) || value.contains(QLatin1Char('_'))) continue;
        auto matches = words.globalMatch(text, token.capturedStart());
        while (matches.hasNext()) {
            const auto word = matches.next();
            if (word.capturedStart() >= token.capturedEnd()) break;
            if (word.capturedLength() >= 2 && word.capturedLength() <= 64) result.append(word);
        }
    }
    return result;
}

class EmojiHighlighter : public QSyntaxHighlighter
{
public:
    explicit EmojiHighlighter(QObject *parent) : QSyntaxHighlighter(parent) {}
    QSet<QString> misspelled;
    QSet<QString> ignored;
    QVariantList mentions;
    QColor mentionColor;

protected:
    void highlightBlock(const QString &text) override
    {
        // Keep flags, keycaps, skin tones and ZWJ families as complete runs.
        static const QRegularExpression emoji(QStringLiteral(
            "(?:[0-9#*]\\x{FE0F}?\\x{20E3}|"
            "[\\x{1F000}-\\x{1FAFF}\\x{2600}-\\x{27BF}\\x{00A9}\\x{00AE}\\x{203C}\\x{2049}\\x{2122}\\x{2139}]"
            "[\\x{FE0E}\\x{FE0F}]?[\\x{1F3FB}-\\x{1F3FF}]?"
            "(?:\\x{200D}[\\x{1F000}-\\x{1FAFF}\\x{2600}-\\x{27BF}]"
            "\\x{FE0F}?[\\x{1F3FB}-\\x{1F3FF}]?)*"
            "[\\x{E0020}-\\x{E007F}]*)+"));
        QTextCharFormat format;
        format.setFontFamilies({QStringLiteral("Noto Color Emoji"),
                                QStringLiteral("Apple Color Emoji"),
                                QStringLiteral("Segoe UI Emoji")});
        auto matches = emoji.globalMatch(text);
        while (matches.hasNext()) {
            const auto match = matches.next();
            // Explicit text presentation is intentional, not a fallback error.
            if (!match.captured().contains(QChar(0xFE0E)))
                setFormat(match.capturedStart(), match.capturedLength(), format);
        }
        for (const auto &value : mentions) {
            const auto span = value.toMap();
            const int start = span.value(QStringLiteral("start")).toInt() - currentBlock().position();
            const int length = span.value(QStringLiteral("length")).toInt();
            for (int i = qMax(0, start); i < qMin(int(text.size()), start + length); ++i) {
                auto mark = this->format(i);
                mark.setForeground(mentionColor);
                mark.setFontWeight(QFont::DemiBold);
                setFormat(i, 1, mark);
            }
        }
        if (misspelled.isEmpty()) return;
        for (const auto &word : spellWords(text)) {
            if (!misspelled.contains(word.captured()) || ignored.contains(word.captured())) continue;
            if (this->format(word.capturedStart()).fontWeight() == QFont::DemiBold) continue;
            QTextCharFormat spelling;
            spelling.setUnderlineStyle(QTextCharFormat::SpellCheckUnderline);
            spelling.setUnderlineColor(QColor(QStringLiteral("#e05b65")));
            setFormat(word.capturedStart(), word.capturedLength(), spelling);
        }
    }
};

struct Replacement {
    int start;
    int length;
    QString text;
};

QList<Replacement> emoticons(const QString &text, int completedEnd)
{
    static const QHash<QString, QString> faces = {
        {QStringLiteral(":-)"), QStringLiteral("🙂")}, {QStringLiteral(":)"), QStringLiteral("🙂")},
        {QStringLiteral(";-)"), QStringLiteral("😉")}, {QStringLiteral(";)"), QStringLiteral("😉")},
        {QStringLiteral(":-D"), QStringLiteral("😃")}, {QStringLiteral(":D"), QStringLiteral("😃")},
        {QStringLiteral(":-("), QStringLiteral("🙁")}, {QStringLiteral(":("), QStringLiteral("🙁")},
        {QStringLiteral(":-P"), QStringLiteral("😛")}, {QStringLiteral(":P"), QStringLiteral("😛")},
        {QStringLiteral(":-p"), QStringLiteral("😛")}, {QStringLiteral(":p"), QStringLiteral("😛")},
        {QStringLiteral(":-O"), QStringLiteral("😮")}, {QStringLiteral(":O"), QStringLiteral("😮")},
        {QStringLiteral(":-o"), QStringLiteral("😮")}, {QStringLiteral(":o"), QStringLiteral("😮")},
        {QStringLiteral(":'("), QStringLiteral("😢")}, {QStringLiteral(":'-("), QStringLiteral("😢")},
        {QStringLiteral("<3"), QStringLiteral("❤️")}
    };
    static const QRegularExpression tokens(QStringLiteral("`+|[^\\s`]+"),
                                           QRegularExpression::UseUnicodePropertiesOption);
    QList<Replacement> result;
    int codeDelimiter = 0;
    auto matches = tokens.globalMatch(text);
    while (matches.hasNext()) {
        const auto match = matches.next();
        QString token = match.captured();
        if (token.startsWith(QLatin1Char('`'))) {
            if (codeDelimiter == 0)
                codeDelimiter = token.size();
            else if (codeDelimiter == token.size())
                codeDelimiter = 0;
            continue;
        }
        const int start = match.capturedStart();
        const int end = match.capturedEnd();
        // Only standalone tokens, never URL/path fragments or backtick code.
        if (codeDelimiter != 0 || (completedEnd >= 0 && end != completedEnd)
                || (start > 0 && !text.at(start - 1).isSpace())
                || (end < text.size() && !text.at(end).isSpace()))
            continue;
        while (!token.isEmpty() && QStringLiteral(".,!?").contains(token.back()))
            token.chop(1);
        const auto face = faces.constFind(token);
        if (face != faces.cend())
            result.append({start, int(token.size()), *face});
    }
    return result;
}
} // namespace

ComposerText::ComposerText(QObject *parent)
    : QObject(parent), m_highlighter(new EmojiHighlighter(this))
{
    m_aspell = QStandardPaths::findExecutable(QStringLiteral("aspell"));
    m_spellDebounce.setSingleShot(true);
    m_spellDebounce.setInterval(400);
    m_spellTimeout.setSingleShot(true);
    m_spellTimeout.setInterval(3000);
    connect(&m_spellDebounce, &QTimer::timeout, this, &ComposerText::checkSpelling);
    connect(&m_spellTimeout, &QTimer::timeout, this, [this] {
        m_spellError = tr("Spell checking timed out. Try another installed dictionary.");
        m_spellProcess.kill();
        emit spellStateChanged();
    });
    connect(&m_spellProcess, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this](int code, QProcess::ExitStatus) { finishSpellJob(code); });
    connect(&m_spellProcess, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) finishSpellJob(-1);
    });
    connect(&m_spellProcess, &QProcess::readyReadStandardOutput, this, [this] {
        if (m_spellProcess.bytesAvailable() > 256 * 1024) m_spellProcess.kill();
    });
    QTimer::singleShot(0, this, [this] {
        if (m_aspell.isEmpty()) {
            m_spellError = tr("Install Aspell and a language dictionary to enable offline spell checking.");
            emit spellStateChanged();
        } else startSpellJob(QStringLiteral("dicts"), {QStringLiteral("dump"), QStringLiteral("dicts")});
    });
}

ComposerText::~ComposerText()
{
    // QProcess can emit finished while its destructor reaps a child. Disconnect
    // before our later-declared state members are destroyed.
    disconnect(&m_spellProcess, nullptr, this, nullptr);
    m_spellTimeout.stop();
    m_spellDebounce.stop();
    if (m_spellProcess.state() != QProcess::NotRunning) {
        m_spellProcess.kill();
        m_spellProcess.waitForFinished(1000);
    }
    if (auto *document = m_highlighter->document())
        disconnect(document, nullptr, this, nullptr);
    m_highlighter->setDocument(nullptr);
}

void ComposerText::setEditor(QQuickItem *editor)
{
    if (m_editor == editor)
        return;
    if (auto *old = m_highlighter->document()) disconnect(old, &QTextDocument::contentsChanged, this, &ComposerText::scheduleSpelling);
    m_editor = editor;
    auto *document = editor ? editor->property("textDocument").value<QQuickTextDocument *>() : nullptr;
    m_highlighter->setDocument(document ? document->textDocument() : nullptr);
    if (document) connect(document->textDocument(), &QTextDocument::contentsChanged, this, &ComposerText::scheduleSpelling);
    scheduleSpelling();
    emit editorChanged();
}

QString ComposerText::editorText() const { return m_editor ? m_editor->property("text").toString() : QString(); }

void ComposerText::setMentionRanges(const QVariantList &ranges)
{
    if (m_mentionRanges == ranges) return;
    m_mentionRanges = ranges;
    auto *highlighter = static_cast<EmojiHighlighter *>(m_highlighter);
    highlighter->mentions = ranges;
    highlighter->rehighlight();
    emit mentionStyleChanged();
}

void ComposerText::setMentionColor(const QColor &color)
{
    if (m_mentionColor == color) return;
    m_mentionColor = color;
    auto *highlighter = static_cast<EmojiHighlighter *>(m_highlighter);
    highlighter->mentionColor = color;
    highlighter->rehighlight();
    emit mentionStyleChanged();
}

void ComposerText::setSpellChecking(bool enabled)
{
    if (enabled == m_spellChecking) return;
    m_spellChecking = enabled;
    scheduleSpelling();
    emit spellStateChanged();
}

void ComposerText::setSpellLanguage(const QString &language)
{
    if (language == m_spellLanguage) return;
    m_spellLanguage = language;
    scheduleSpelling();
    emit spellStateChanged();
}

void ComposerText::scheduleSpelling()
{
    m_suggestions.clear();
    m_word.clear();
    emit suggestionsChanged();
    auto *highlighter = static_cast<EmojiHighlighter *>(m_highlighter);
    if (!m_spellChecking || editorText().isEmpty()) {
        m_spellDebounce.stop();
        if (!highlighter->misspelled.isEmpty()) { highlighter->misspelled.clear(); highlighter->rehighlight(); }
        return;
    }
    m_spellDebounce.start();
}

void ComposerText::checkSpelling()
{
    if (!m_spellChecking || !m_editor || m_editor->property("inputMethodComposing").toBool()) return;
    if (m_spellProcess.state() != QProcess::NotRunning) { m_spellDebounce.start(); return; }
    if (!m_dictionaries.contains(m_spellLanguage)) {
        m_spellError = tr("Choose an installed spelling dictionary in Settings → Chats.");
        emit spellStateChanged();
        return;
    }
    m_spellInput = editorText();
    if (m_spellInput.size() > 20000) {
        m_spellError = tr("Spell checking is paused for drafts over 20,000 characters.");
        emit spellStateChanged();
        return;
    }
    QSet<QString> words;
    for (const auto &word : spellWords(m_spellInput)) words.insert(word.captured());
    startSpellJob(QStringLiteral("check"), {QStringLiteral("list"), QStringLiteral("--encoding=utf-8"), QStringLiteral("--lang=") + m_spellLanguage},
                  words.values().join(QLatin1Char('\n')).toUtf8() + '\n');
}

void ComposerText::startSpellJob(const QString &job, const QStringList &arguments, const QByteArray &input)
{
    m_spellJob = job;
    m_spellJobLanguage = m_spellLanguage;
    m_spellError.clear();
    m_spellProcess.start(m_aspell, arguments);
    if (!input.isEmpty()) m_spellProcess.write(input);
    m_spellProcess.closeWriteChannel();
    m_spellTimeout.start();
}

void ComposerText::finishSpellJob(int code)
{
    m_spellTimeout.stop();
    const auto output = QString::fromUtf8(m_spellProcess.readAllStandardOutput());
    m_spellProcess.readAllStandardError(); // Never log draft text or dictionary output.
    const auto job = std::exchange(m_spellJob, QString());
    if (job.isEmpty()) return;
    if (code != 0) {
        if (m_spellError.isEmpty()) m_spellError = tr("Spell checker could not run with this dictionary.");
    } else if (job == QStringLiteral("dicts")) {
        static const QRegularExpression valid(QStringLiteral("^[a-z]{2,3}(?:_[A-Z]{2})?$"));
        for (const auto &line : output.split(QLatin1Char('\n'))) if (valid.match(line.trimmed()).hasMatch()) m_dictionaries.append(line.trimmed());
        m_dictionaries.removeDuplicates();
        m_dictionaries.sort();
        if (m_dictionaries.isEmpty()) m_spellError = tr("No Aspell dictionaries are installed.");
        scheduleSpelling();
    } else if (job == QStringLiteral("check") && m_spellChecking && m_spellInput == editorText() && m_spellJobLanguage == m_spellLanguage) {
        auto *highlighter = static_cast<EmojiHighlighter *>(m_highlighter);
        QSet<QString> misspelled;
        for (const auto &line : output.split(QLatin1Char('\n'))) if (!line.trimmed().isEmpty()) misspelled.insert(line.trimmed());
        if (highlighter->misspelled != misspelled) { highlighter->misspelled = misspelled; highlighter->rehighlight(); }
    } else if (job == QStringLiteral("suggest") && m_suggestionText == editorText() && m_spellJobLanguage == m_spellLanguage) {
        for (const auto &line : output.split(QLatin1Char('\n'))) {
            if (!line.startsWith(QLatin1Char('&')) && !line.startsWith(QLatin1Char('?'))) continue;
            const auto colon = line.indexOf(QStringLiteral(": "));
            if (colon >= 0) m_suggestions = line.mid(colon + 2).split(QStringLiteral(", ")).mid(0, 6);
        }
        emit suggestionsChanged();
    }
    emit spellStateChanged();
}

void ComposerText::suggestAt(int position)
{
    m_suggestions.clear();
    m_word.clear();
    if (!m_spellChecking || m_spellProcess.state() != QProcess::NotRunning) { emit suggestionsChanged(); return; }
    const auto text = editorText();
    const auto *highlighter = static_cast<EmojiHighlighter *>(m_highlighter);
    for (const auto &word : spellWords(text)) {
        if (position < word.capturedStart() || position > word.capturedEnd() || !highlighter->misspelled.contains(word.captured()) || highlighter->ignored.contains(word.captured())) continue;
        m_word = word.captured();
        m_wordStart = word.capturedStart();
        m_suggestionText = text;
        // Prefixing ^ forces text mode, so draft text cannot issue pipe commands.
        startSpellJob(QStringLiteral("suggest"), {QStringLiteral("-a"), QStringLiteral("--encoding=utf-8"), QStringLiteral("--lang=") + m_spellLanguage},
                      '^' + m_word.toUtf8() + '\n');
        break;
    }
    emit suggestionsChanged();
}

void ComposerText::replaceSpelling(const QString &word)
{
    if (!m_editor || m_suggestionText != editorText() || !m_suggestions.contains(word) || m_editor->property("readOnly").toBool()) return;
    m_editor->forceActiveFocus();
    QMetaObject::invokeMethod(m_editor, "deselect");
    const int cursor = m_editor->property("cursorPosition").toInt();
    QInputMethodEvent event;
    event.setCommitString(word, m_wordStart - cursor, m_word.size());
    QCoreApplication::sendEvent(m_editor, &event);
}

void ComposerText::ignoreSpelling()
{
    if (m_word.isEmpty()) return;
    auto *highlighter = static_cast<EmojiHighlighter *>(m_highlighter);
    highlighter->ignored.insert(m_word);
    highlighter->rehighlight();
    m_word.clear();
    m_suggestions.clear();
    emit suggestionsChanged();
}

void ComposerText::convertEmoticons(bool all)
{
    if (!m_editor || m_editor->property("readOnly").toBool()
            || m_editor->property("inputMethodComposing").toBool())
        return;
    const QString text = m_editor->property("text").toString();
    const int cursor = m_editor->property("cursorPosition").toInt();
    const int selectionStart = m_editor->property("selectionStart").toInt();
    const int selectionEnd = m_editor->property("selectionEnd").toInt();
    int completedEnd = -1;
    if (!all) {
        if (selectionStart != selectionEnd || cursor <= 0 || !text.at(cursor - 1).isSpace())
            return;
        completedEnd = cursor;
        while (completedEnd > 0 && text.at(completedEnd - 1).isSpace())
            --completedEnd;
    }
    const auto replacements = emoticons(text, completedEnd);
    if (replacements.isEmpty())
        return;
    const int start = replacements.first().start;
    const int end = replacements.last().start + replacements.last().length;
    QString replacement = text.mid(start, end - start);
    for (auto it = replacements.crbegin(); it != replacements.crend(); ++it)
        replacement.replace(it->start - start, it->length, it->text);
    const auto mappedPosition = [&replacements](int position) {
        int delta = 0;
        for (const auto &edit : replacements) {
            if (position <= edit.start)
                break;
            if (position < edit.start + edit.length)
                return edit.start + delta + int(edit.text.size());
            delta += edit.text.size() - edit.length;
        }
        return position + delta;
    };
    // Go through TextEdit's native input path, not HTML or direct document
    // mutation (unsupported by QQuickTextDocument on Qt 6.5). One undo step.
    QMetaObject::invokeMethod(m_editor, "deselect");
    m_editor->setProperty("cursorPosition", cursor);
    QInputMethodEvent event;
    event.setCommitString(replacement, start - cursor, end - start);
    QCoreApplication::sendEvent(m_editor, &event);
    if (selectionStart == selectionEnd) {
        m_editor->setProperty("cursorPosition", mappedPosition(cursor));
    } else {
        const int anchor = cursor == selectionStart ? selectionEnd : selectionStart;
        QMetaObject::invokeMethod(m_editor, "select", Q_ARG(int, mappedPosition(anchor)),
                                  Q_ARG(int, mappedPosition(cursor)));
    }
}
