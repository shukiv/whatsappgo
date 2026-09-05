pragma Singleton
import QtQuick

QtObject {
    property string preferredMode: "system"

    // WhatsApp names the platform in its own empty state ("WhatsApp for
    // Windows"), so this application does the same rather than claiming Linux
    // everywhere.
    readonly property string platformName: {
        switch (Qt.platform.os) {
        case "windows": return qsTr("Windows")
        case "osx": return qsTr("Mac")
        default: return qsTr("Linux")
        }
    }

    readonly property string requestedMode: {
        const args = Qt.application.arguments
        for (let i = 0; i < args.length; ++i) {
            if (args[i] === "--theme" && i + 1 < args.length)
                return ["light", "dark", "system"].indexOf(args[i + 1]) >= 0 ? args[i + 1] : ""
            if (args[i].indexOf("--theme=") === 0) {
                const value = args[i].slice(8)
                return ["light", "dark", "system"].indexOf(value) >= 0 ? value : ""
            }
        }
        return ""
    }
    readonly property string mode: requestedMode || preferredMode
    readonly property bool dark: mode === "dark" || (mode === "system" && Application.styleHints.colorScheme === Qt.Dark)

    readonly property color brand: "#25D366"
    // The light tokens follow the current WhatsApp Web surfaces rather than
    // the older blue-grey/teal palette. Dark mode retains the familiar
    // high-contrast linked-device palette until it is measured independently.
    readonly property color primary: dark ? "#00A884" : "#1DAA61"
    readonly property color primaryText: "#FFFFFF"
    readonly property color primaryContainer: dark ? "#005C4B" : "#D9FDD3"
    // QML reserves onXxx names for signal handlers.
    readonly property color primaryContainerText: dark ? "#E9EDEF" : "#0A0A0A"
    readonly property color navigation: dark ? "#111B21" : "#F7F5F3"
    readonly property color navigationSelected: dark ? "#2A3942" : "#E4E2DF"
    readonly property color navigationHover: dark ? "#26343C" : "#ECEAE7"
    readonly property color navigationPressed: dark ? "#31414A" : "#DCDAD7"
    readonly property color surface: dark ? "#111B21" : "#FFFFFF"
    readonly property color surfaceRaised: dark ? "#202C33" : "#FFFFFF"
    readonly property color surfaceMuted: dark ? "#2A3942" : "#F6F5F4"
    readonly property color chatBackground: dark ? "#0B141A" : "#F5F1EB"
    readonly property color emptyBackground: dark ? "#0B141A" : "#F7F5F3"
    readonly property color composer: dark ? "#202C33" : "#FFFFFF"
    readonly property color input: dark ? "#2A3942" : "#FFFFFF"
    readonly property color text: dark ? "#E9EDEF" : "#0A0A0A"
    readonly property color textMuted: dark ? "#8696A0" : "#54545A"
    // The small marks along a chat row - pinned, muted, and the rest. WhatsApp
    // Web draws these a shade lighter than its secondary text.
    readonly property color iconMuted: dark ? "#8696A0" : "#666666"
    // The date pill between one day of a conversation and the next. WhatsApp
    // Web floats it over the wallpaper: white in the light theme, and the same
    // panel colour it uses for received bubbles in the dark one.
    readonly property color daySeparator: dark ? "#202C33" : "#FFFFFF"
    readonly property color border: dark ? "#2A3942" : "#DDDAD6"
    readonly property color icon: dark ? "#AEBAC1" : "#54545A"
    readonly property color avatar: dark ? "#374248" : "#E7E5E2"
    readonly property color avatarText: dark ? "#DDE5EA" : "#54545A"
    readonly property color danger: dark ? "#F15C6D" : "#EA0038"
    readonly property color dangerText: "#FFFFFF"
    readonly property color attachmentDocument: dark ? "#A78BFA" : "#7C4DFF"
    readonly property color attachmentMedia: dark ? "#60A5FA" : "#1677FF"
    readonly property color attachmentCamera: dark ? "#FB7185" : "#F43F75"
    readonly property color attachmentAudio: dark ? "#FB923C" : "#F35A2C"
    readonly property color attachmentContact: dark ? "#38BDF8" : "#0284C7"
    readonly property color attachmentPoll: dark ? "#FBBF24" : "#D97706"
    readonly property color attachmentEvent: dark ? "#FB7185" : "#E11D48"
    readonly property color attachmentSticker: dark ? "#34D399" : "#059669"
    readonly property color incomingBubble: dark ? "#202C33" : "#FFFFFF"
    readonly property color outgoingBubble: dark ? "#005C4B" : "#D9FDD3"
    readonly property color bubbleBorder: dark ? "#00000000" : "#14000000"
    readonly property color replyBackground: dark ? "#111B21" : "#F0EFED"
    // WhatsApp Web's `--icon-ack`, measured from the live client. It carries the
    // same value in both themes, which is why this one is not a light/dark pair.
    readonly property color readReceipt: "#007BFC"
    // A link inside a bubble is drawn a shade deeper than the accent green so it
    // stays legible on the outgoing bubble's pale fill.
    readonly property color link: dark ? "#53BDEB" : "#1B8755"
    // Chat-filter pills. The live client draws one hairline for every state and
    // marks the selected filter with the fill and a darker green label.
    readonly property color filterChipBorder: dark ? "#3B4A54" : "#33000000"
    readonly property color filterChipSelectedText: dark ? "#E9EDEF" : "#15603E"
    readonly property color selectedRow: dark ? "#2A3942" : "#F6F5F4"
    readonly property color hoverRow: dark ? "#26343C" : "#F7F5F3"
    readonly property color pressedRow: dark ? "#31414A" : "#EEECE9"
    readonly property color scrollbarHandle: dark ? "#667781" : "#8D8B88"
    readonly property color scrollbarHandleHover: dark ? "#AEBAC1" : "#666461"
    // WhatsApp Web keeps the light-theme doodles faint so the canvas reads as
    // an airy background rather than a second foreground texture.
    readonly property real patternOpacity: dark ? 0.10 : 0.12
    readonly property string emojiFontFamily: "Noto Color Emoji"

    readonly property int radiusSmall: 6
    readonly property int radiusMedium: 10
    readonly property int radiusLarge: 8
    readonly property int spacingSmall: 4
    readonly property int spacing: 8
    readonly property int spacingLarge: 16

    // Qt spells a local file as file:///path: three slashes, forward slashes
    // throughout, and the leading slash kept in front of a Windows drive
    // letter. Concatenating "file://" with a path produced file://C:\Users\...
    // on Windows, where C reads as a host name, so every cached image, avatar
    // and video failed to open.
    function fileUrl(path) {
        const value = String(path || "")
        if (!value)
            return ""
        // Two characters at least, so a "C:" drive letter is not read as a
        // URL scheme.
        if (/^[a-z][a-z0-9+.-]+:/i.test(value))
            return value
        const slashed = value.replace(/\\/g, "/")
        return slashed.charAt(0) === "/" ? "file://" + slashed : "file:///" + slashed
    }

    // The inverse: a dropped or chosen file arrives as a URL and the daemon
    // wants a path.
    function localPath(url) {
        const value = String(url || "")
        if (value.indexOf("file://") !== 0)
            return decodeURIComponent(value)
        const path = decodeURIComponent(value.substring("file://".length))
        return /^\/[A-Za-z]:/.test(path) ? path.substring(1) : path
    }

    function escapeHtml(value) {
        return String(value || "")
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/\"/g, "&quot;")
            .replace(/'/g, "&#39;")
    }

    function emojiMarkup(safe) {
        const atom = "(?:[\\uD83C-\\uDBFF][\\uDC00-\\uDFFF]|[\\u2600-\\u27BF])(?:\\uFE0F|\\uFE0E)?(?:\\uD83C[\\uDFFB-\\uDFFF])?"
        const flags = "(?:\\uD83C[\\uDDE6-\\uDDFF]){2}"
        const cluster = new RegExp("(" + flags + "|" + atom + "(?:\\u200D" + atom + ")*)", "g")
        return safe.replace(cluster, "<span style=\"font-family:'Noto Color Emoji'\">$1</span>")
    }

    function richTextSegment(value) {
        return emojiMarkup(escapeHtml(value)).replace(/\n/g, "<br>")
    }

    function emojiRichText(value) {
        return richTextSegment(value)
    }

    // linkColor and underlined let a surface that is not the chat background
    // ask for readable links: a status is drawn on solid teal, where the chat
    // link green is close to invisible.
    function linkifiedLine(source, linkColor, underlined) {
        const urlPattern = /https?:\/\/[^\s<>\"']+/gi
        let rendered = ""
        let cursor = 0
        let match
        while ((match = urlPattern.exec(source)) !== null) {
            let url = match[0]
            let suffix = ""
            while (url.length > 0 && /[.,!?;:]/.test(url[url.length - 1])) {
                suffix = url[url.length - 1] + suffix
                url = url.slice(0, -1)
            }
            if (url.endsWith(")") && (url.match(/\)/g) || []).length > (url.match(/\(/g) || []).length) {
                suffix = ")" + suffix
                url = url.slice(0, -1)
            }
            rendered += richTextSegment(source.slice(cursor, match.index))
            const color = linkColor || link
            const decoration = underlined ? "underline" : "none"
            rendered += "<a href=\"" + escapeHtml(url) + "\" style=\"color:" + color + ";text-decoration:" + decoration + "\">" + richTextSegment(url) + "</a>"
            rendered += richTextSegment(suffix)
            cursor = match.index + match[0].length
        }
        return rendered + richTextSegment(source.slice(cursor))
    }

    function messageRichText(value, linkColor, underlined) {
        const lines = String(value || "").split("\n")
        const rendered = []
        for (let i = 0; i < lines.length; ++i) {
            const quoted = lines[i].startsWith("▎") || /^\s*>\s?/.test(lines[i])
            if (!quoted) {
                rendered.push(linkifiedLine(lines[i], linkColor, underlined) || "&nbsp;")
                continue
            }
            const content = lines[i].startsWith("▎")
                ? lines[i].slice(1).replace(/^\s/, "")
                : lines[i].replace(/^\s*>\s?/, "")
            rendered.push("<span style=\"color:" + primary + "\">▎</span>&nbsp;"
                + (linkifiedLine(content, linkColor, underlined) || "&nbsp;"))
        }
        return rendered.join("<br>")
    }

    function isEmojiOnly(value) {
        const compact = String(value || "").replace(/\s/g, "")
        if (!compact)
            return false
        const atom = "(?:[\\uD83C-\\uDBFF][\\uDC00-\\uDFFF]|[\\u2600-\\u27BF])(?:\\uFE0F|\\uFE0E)?(?:\\uD83C[\\uDFFB-\\uDFFF])?"
        const flags = "(?:\\uD83C[\\uDDE6-\\uDDFF]){2}"
        const cluster = new RegExp("(" + flags + "|" + atom + "(?:\\u200D" + atom + ")*)", "g")
        return compact.replace(cluster, "").length === 0
    }
}
