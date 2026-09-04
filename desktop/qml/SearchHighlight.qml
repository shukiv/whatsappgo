pragma Singleton
import QtQuick

// WhatsApp Web tints the matched run inside a search result rather than
// leaving the reader to find it. Styled text is the only way to colour part of
// a label, so the surrounding text has to be escaped before the tags go in.
QtObject {
    function escapeHtml(text) {
        return String(text === undefined || text === null ? "" : text)
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
    }

    // Embedded newlines survive elide and maximumLineCount in rich text, so a
    // two-line body would push the row past its height; they collapse here.
    function oneLine(text) {
        return String(text === undefined || text === null ? "" : text).replace(/\s+/g, " ").trim()
    }

    // Matching is case-insensitive on a lowercased copy, and the slices come
    // from the original so the result keeps the sender's own capitalisation.
    function markup(text, query, color) {
        const source = oneLine(text)
        const needle = oneLine(query)
        if (needle.length === 0)
            return escapeHtml(source)
        const haystack = source.toLowerCase()
        const at = haystack.indexOf(needle.toLowerCase())
        if (at < 0)
            return escapeHtml(source)
        return escapeHtml(source.slice(0, at))
            + "<font color=\"" + color + "\"><b>" + escapeHtml(source.slice(at, at + needle.length)) + "</b></font>"
            + escapeHtml(source.slice(at + needle.length))
    }
}
