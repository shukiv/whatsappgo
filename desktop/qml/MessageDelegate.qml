import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window
import org.whatsappgo

Item {
    id: root
    required property var modelData
    property string chatTitle: ""
    property url chatAvatarSource
    property string ownTitle: ""
    property bool navigationHighlighted: false
	property bool actionsEnabled: true
    // Selection mode turns the whole bubble into a checkbox: hover actions and
    // menus are suppressed so a tap cannot both select and act.
    property bool selectionActive: false
    property bool selected: false
    signal selectionToggled(var message)
    signal editRequested(string messageId, string body)
    signal deleteRequested(string messageId, string senderJid)
    signal replyRequested(string messageId, string body)
    signal pinRequested(string messageId, string senderJid, string body)
    signal starRequested(string messageId, string senderJid, bool fromMe, bool starred)
    signal forwardRequested(string messageId)
    signal imagePreviewRequested(var message)
    property bool animationSelected: false
    signal animationRequested(string messageId)
    signal animationFailed(string message)
    signal quotedMessageRequested(string messageId)
    signal viewOnceNoticeRequested()
    signal mentionRequested(string jid, string name)
    signal interactiveRequested(var message)
    signal infoRequested(var message)

    // Only Main dispatches shortcuts. A shared preview in Message info must
    // never compete with the conversation for a window-wide key sequence.
    readonly property bool shortcutHovered: bubbleHover.hovered
        || messageMenuButton.hovered || messageReactionButton.hovered

    function triggerKeyboardAction(action) {
        if (!actionsEnabled || selectionActive || modelData.revoked
                || !modelData.id || modelData.kind === "system")
            return false
        if (action === "reply") {
            replyRequested(modelData.id, viewOnceKind ? mediaLabel : modelData.body || mediaLabel)
        } else if (action === "forward" && !viewOnceKind) {
            forwardRequested(modelData.id)
        } else if (action === "star" && !viewOnceKind) {
            starRequested(modelData.id, modelData.sender_jid || "",
                          Boolean(modelData.from_me), !modelData.starred)
        } else {
            return false
        }
        return true
    }

    readonly property bool hasReply: !viewOnceKind && Boolean(modelData.reply_to)
    readonly property bool viewOnceKind: modelData.kind === "view_once"
    readonly property bool hasMedia: !viewOnceKind && Boolean(modelData.media_path)
    readonly property bool mediaKind: ["image", "video", "audio", "document", "sticker"].indexOf(modelData.kind) >= 0
    // WhatsApp Web puts a sticker on the conversation itself. It is drawn to
    // be seen against the wallpaper, so a panel behind it is wrong twice: it
    // fills in the transparency the sticker was drawn around, and it frames a
    // picture that is meant to be loose on the page.
    readonly property bool stickerKind: modelData.kind === "sticker"
    // A persisted cache path is not proof that its file survived a restore or
    // cache eviction. Reload a recovered file even when its path is unchanged.
    property int stickerRevision: 0
    readonly property bool stickerPreviewFailed: stickerKind && hasPreview && mediaImage.status === Image.Error
    readonly property bool gifKind: modelData.kind === "video" && Boolean(modelData.gif_playback)
    readonly property bool animatedSticker: stickerKind && Boolean(modelData.sticker_animated) && Boolean(modelData.sticker_source)
    readonly property bool animatedKind: gifKind || animatedSticker
    // ListView retains offscreen delegates; visibility alone is not a viewport
    // check. The ID is owned by Main, so at most one chat animation is selected.
    readonly property bool inViewport: {
        const view = root.ListView.view
        // Validate the attached QObject before reading its geometry. A plain
        // JS null guard can still warn in compiled QML during view teardown.
        if (!Qt.isQtObject(view)) return false
        return root.y + root.height > view.contentY && root.y < view.contentY + view.height
    }
    readonly property bool animationRunning: animationSelected && animatedKind && inViewport && visible && actionsEnabled && !selectionActive && !modelData.revoked
    readonly property bool visualKind: ["image", "video", "sticker"].indexOf(modelData.kind) >= 0
    readonly property bool framedVisualKind: modelData.kind === "image" || modelData.kind === "video"
    readonly property bool audioKind: modelData.kind === "audio"
    readonly property bool documentKind: modelData.kind === "document"
    // WhatsApp Web marks the reaction the reader themselves left, so it can be
    // replaced or taken back knowingly. The self reaction is the one whose
    // sender is this account; comparison is on the user part because the same
    // person arrives as a phone JID or a LID depending on the chat.
    // Not readonly: a test stands in for the account this is running as, which
    // is otherwise only known once a daemon has answered.
    property string selfUserPart: {
        const jid = String(backend.status.user_jid || "")
        return jid === "" ? "" : jid.split("@")[0].split(":")[0]
    }
    // WhatsApp Web counts a thumb as a thumb whoever left it: the skin tone
    // somebody chose is theirs, not a reaction of its own. The tone modifiers
    // and the emoji presentation selector are dropped for grouping, so 👍 and
    // 👍🏻 land under one heading, the way the group panel shows them.
    function reactionKey(emoji) {
        return String(emoji).replace(/[\u{1F3FB}-\u{1F3FF}\uFE0F]/gu, "")
    }
    readonly property var reactionSummary: {
        const summary = []
        const indexes = ({})
        const reactions = modelData.reactions || []
        for (let i = 0; i < reactions.length; ++i) {
            const emoji = String(reactions[i].emoji || "")
            if (!emoji)
                continue
            const key = root.reactionKey(emoji) || emoji
            const sender = String(reactions[i].sender_jid || "").split("@")[0].split(":")[0]
            const mine = root.selfUserPart !== "" && sender === root.selfUserPart
            if (indexes[key] === undefined) {
                indexes[key] = summary.length
                summary.push({ emoji: key, count: 1, mine: mine })
            } else {
                summary[indexes[key]].count += 1
                summary[indexes[key]].mine = summary[indexes[key]].mine || mine
            }
        }
        return summary
    }
    // One row per person for the panel: who they are, what they left, and
    // whether it is the reader's own, which is the one that can be taken back.
    readonly property var reactionPeople: {
        const people = []
        const reactions = modelData.reactions || []
        for (let i = 0; i < reactions.length; ++i) {
            const emoji = String(reactions[i].emoji || "")
            if (!emoji)
                continue
            const jid = String(reactions[i].sender_jid || "")
            const user = jid.split("@")[0].split(":")[0]
            const name = String(reactions[i].sender_name || "")
            people.push({
                emoji: emoji,
                key: root.reactionKey(emoji) || emoji,
                jid: jid,
                name: name,
                // Only a phone JID carries a number anybody can read; the
                // identifier of a group-only member is not one.
                number: jid.indexOf("@s.whatsapp.net") > 0 ? "+" + user : "",
                avatar: String(reactions[i].sender_avatar_path || ""),
                mine: root.selfUserPart !== "" && user === root.selfUserPart
            })
        }
        return people
    }
    readonly property int reactionTotal: {
        let total = 0
        for (let i = 0; i < root.reactionSummary.length; ++i)
            total += root.reactionSummary[i].count
        return total
    }
    readonly property bool reactedByMe: {
        for (let i = 0; i < root.reactionSummary.length; ++i) {
            if (root.reactionSummary[i].mine)
                return true
        }
        return false
    }
    // The badge is the emoji themselves and, once more than one person has
    // reacted, how many there were in total. WhatsApp Web shows at most three
    // faces before it lets the number speak for the rest.
    readonly property string reactionSummaryText: reactionSummary.slice(0, 3).map(function(reaction) {
        return reaction.emoji
    }).join("")

    // The local file when it has been downloaded, otherwise the inline
    // thumbnail WhatsApp delivers with the message itself. Showing the
    // thumbnail means a photo or video is visible before it is fetched.
    readonly property string previewPath: {
        if ((modelData.kind === "image" || modelData.kind === "sticker") && modelData.media_path)
            return String(modelData.media_path)
        return String(modelData.media_thumbnail || "")
    }
    readonly property bool hasPreview: visualKind && previewPath !== ""
    readonly property bool previewIsThumbnail: hasPreview && previewPath !== String(modelData.media_path || "")

    // Every width limit inside the bubble derives from the conversation width.
    // Nothing may size itself from the bubble, because the bubble sizes itself
    // from its content and the two together would form a binding loop.
    readonly property real horizontalPadding: 11
    // A photo/video and its caption share a compact width. Letting the caption
    // use the text-message limit left a large blank panel beside the preview.
    // 336 logical px matches the ~420 px reference at 125% desktop scaling.
    readonly property real maxBubbleWidth: Math.max(180,
        Math.min(root.width * 0.68, framedVisualKind || documentKind ? 336 : 620))
    readonly property real contentMaxWidth: maxBubbleWidth - 2 * horizontalPadding
    readonly property real linkCardWidth: Math.min(contentMaxWidth,
        Math.max(320, Math.min(root.width * 0.46, 420)))
    // Images sit closer to the bubble edge; captions retain the text gutter.
    readonly property real mediaHorizontalPadding: framedVisualKind || documentKind ? 4 : horizontalPadding
    readonly property real mediaOutset: horizontalPadding - mediaHorizontalPadding
    readonly property real mediaWidth: Math.min(contentMaxWidth + 2 * mediaOutset,
        modelData.kind === "sticker" ? 160 : 328)
    readonly property real mediaDisplayHeight: {
        const previewWidth = Number(modelData.preview_width || 0)
        const previewHeight = Number(modelData.preview_height || 0)
        if (previewWidth <= 0 || previewHeight <= 0)
            return modelData.kind === "sticker" ? 160 : 170
        const natural = mediaWidth * (previewHeight / previewWidth)
        return Math.max(110, Math.min(modelData.kind === "sticker" ? 160 : 320, natural))
    }

    // Keep compact chat density while giving short bubbles a little more
    // presence and a comfortably readable baseline.
    readonly property int bodyFontSize: 15
    readonly property int captionFontSize: 14
    readonly property int metaFontSize: 11

    readonly property bool playingThis: Playback.isCurrent(modelData.id)
    readonly property bool contactKind: modelData.kind === "contact"
    readonly property bool locationKind: modelData.kind === "location"
    readonly property string contactName: String(modelData.contact_name || modelData.body || "")
    readonly property string contactPhone: String(modelData.contact_phone || "")
    readonly property bool hasCoordinates: locationKind
        && (Number(modelData.latitude || 0) !== 0 || Number(modelData.longitude || 0) !== 0)

    // A short message keeps its timestamp on the same line, the way WhatsApp
    // does, instead of spending a whole line on it.
    readonly property bool metaBeside: bodyText.visible && !bodyText.wrapped
        && !mediaFrame.visible && !voiceRow.visible && !fileRow.visible
        && !documentCard.visible
        && !linkPreview.visible && !contactCard.visible && !locationCard.visible
        && (bodyText.implicitWidth + metaRow.implicitWidth + 12) <= contentMaxWidth

    // The bubble is exactly as wide as its widest part, capped by the
    // conversation. Every term below is a natural size that does not depend on
    // the width the bubble ends up with, so the two cannot chase each other:
    // a ColumnLayout with filling children could inflate itself instead of
    // hugging the text.
    readonly property real naturalContentWidth: Math.max(
        senderLabel.visible ? Math.min(senderLabel.implicitWidth, contentMaxWidth) : 0,
        forwardedMark.visible ? Math.min(forwardedMark.implicitWidth, contentMaxWidth) : 0,
        replyBox.visible ? Math.min(Math.max(replyBox.naturalWidth, 96), contentMaxWidth) : 0,
        mediaFrame.visible ? mediaWidth - 2 * mediaOutset : 0,
        voiceRow.visible ? 284 : 0,
        documentCard.visible ? 328 - 2 * mediaOutset : 0,
        fileRow.visible ? Math.min(fileRow.implicitWidth, contentMaxWidth) : 0,
        linkPreview.visible ? linkCardWidth : 0,
        contactCard.visible ? Math.min(Math.max(contactCard.naturalWidth, 220), contentMaxWidth) : 0,
        locationCard.visible ? Math.min(260, contentMaxWidth) : 0,
        viewOnceNotice.visible ? Math.min(480, contentMaxWidth) : 0,
        bodyText.visible ? (linkPreview.visible ? Math.min(bodyText.implicitWidth, linkCardWidth)
            : (metaBeside ? bodyText.implicitWidth + metaRow.implicitWidth + 12 : bodyText.implicitWidth)) : 0,
        unsupportedLabel.visible ? unsupportedLabel.implicitWidth : 0,
        metaRow.implicitWidth)
    readonly property real contentWidth: Math.min(Math.max(naturalContentWidth, 54), contentMaxWidth)

    readonly property string mediaLabel: {
        switch (modelData.kind) {
        case "image": return qsTr("Photo")
        case "video": return gifKind ? qsTr("GIF") : qsTr("Video")
        case "audio": return qsTr("Voice message")
        case "document": return modelData.media_name || qsTr("Document")
        case "sticker": return qsTr("Sticker")
        case "contact": return qsTr("Contact")
        case "location": return qsTr("Location")
        case "poll": return qsTr("Poll")
        case "event": return qsTr("Event")
        case "view_once": return qsTr("View once message")
        default: return ""
        }
    }

    // The site a link points at, shown under its preview the way a browser
    // shows the address of a page.
    function linkHost(url) {
        const withoutScheme = String(url || "").replace(/^[a-z]+:\/\//i, "")
        const host = withoutScheme.split("/")[0].split("?")[0]
        return host.replace(/^www\./i, "")
    }

    function openVisualMedia() {
        if (animatedKind) {
            animationRequested(String(modelData.id || ""))
            if (!hasMedia) backend.downloadMedia(modelData.id)
            return
        }
        if (modelData.kind === "video") {
            Playback.start(modelData.id, modelData.media_path || "", true)
            return
        }
        if ((modelData.kind === "image" || modelData.kind === "sticker") && hasPreview) {
            if (previewIsThumbnail)
                backend.ensureMedia(modelData.id)
            imagePreviewRequested(modelData)
        }
    }

    readonly property real playProgress: playingThis && Playback.duration > 0
        ? Playback.position / Playback.duration : 0

    // The amplitude bars the sender recorded. Messages that reached this
    // device without them still read as a voice note rather than a flat line:
    // the shape is derived from the message id, so it is stable while
    // scrolling instead of changing on every redraw.
    readonly property var waveformBars: {
        if (!audioKind)
            return []
        const stored = modelData.audio_waveform
        const source = (stored && stored.length > 0) ? stored : syntheticWaveform()
        const target = 44
        if (source.length <= target)
            return source
        const sampled = []
        for (let i = 0; i < target; ++i)
            sampled.push(source[Math.floor(i * source.length / target)])
        return sampled
    }

    function syntheticWaveform() {
        const id = String(modelData.id || "")
        let seed = 7
        for (let i = 0; i < id.length; ++i)
            seed = (seed * 31 + id.charCodeAt(i)) % 2147483647
        const bars = []
        for (let i = 0; i < 40; ++i) {
            seed = (seed * 48271) % 2147483647
            bars.push(22 + (seed % 68))
        }
        return bars
    }

    function formatClock(milliseconds) {
        const total = Math.max(0, Math.round(milliseconds / 1000))
        const minutes = Math.floor(total / 60)
        const seconds = total % 60
        return minutes + ":" + (seconds < 10 ? "0" : "") + seconds
    }

    function clampPopupX(popup, value) {
        return Math.max(8, Math.min(popup.parent.width - popup.width - 8, value))
    }

    function clampPopupY(popup, value) {
        const shownHeight = Math.max(popup.height, popup.implicitHeight)
        return Math.max(8, Math.min(popup.parent.height - shownHeight - 8, value))
    }

    // Menus contain hundreds of objects. Keep them out of the scrolling hot
    // path and construct them synchronously only for the message being used.
    readonly property var contextMenu: actionsLoader.item ? actionsLoader.item.contextMenu : null
    readonly property var quickReactionPopup: actionsLoader.item ? actionsLoader.item.quickReactionPopup : null
    readonly property var fullReactionPicker: actionsLoader.item ? actionsLoader.item.fullReactionPicker : null
    readonly property var reactionDetailsPopup: actionsLoader.item ? actionsLoader.item.reactionDetailsPopup : null
    property string actionMessageId: ""

    function ensureActions() {
        actionMessageId = String(modelData.id || "")
        actionsLoader.active = true
    }

    function releaseActions() {
        closeActionPopups()
        actionsLoader.active = false
        actionMessageId = ""
    }

    ListView.onPooled: releaseActions()

    function openMessageMenuAt(x, y) {
        ensureActions()
        contextMenu.capturedSelection = bodyText.selectedText
        const messageId = actionMessageId
        // Visibility-dependent rows change the popup's implicit height. Let
        // those bindings settle before positioning so the bottom clamp uses
        // the complete menu rather than an earlier, shorter action set.
        Qt.callLater(function() {
            if (!contextMenu || actionMessageId !== messageId)
                return
            try {
                openMessagePopups(x, y)
            } catch (error) {
                // Both popups open from one callback, so an exception halfway
                // through leaves the menu up and the reaction row missing -
                // which is what the interface looks like on CI.
                console.warn("could not open the message popups:", error)
            }
        })
    }

    function openMessagePopups(x, y) {
        ensureActions()
        {
            contextMenu.x = clampPopupX(contextMenu, x)
            contextMenu.y = clampPopupY(contextMenu, y)
            contextMenu.open()

            quickReactionPopup.pairedWithMenu = true
            positionMessagePopups()
            quickReactionPopup.open()
            Qt.callLater(root.positionMessagePopups)
        }
    }

    function positionMessagePopups() {
        if (!contextMenu || !contextMenu.visible || !quickReactionPopup.pairedWithMenu)
            return
        // The menu's visibility-dependent rows are polished during open(), and
        // WhatsAppMenuPopup clamps it again afterwards. Follow those final bounds
        // rather than leaving the tray at the menu's original click position.
        const gap = 6
        const margin = 8
        const menuHeight = Math.max(contextMenu.height, contextMenu.implicitHeight)
        const trayHeight = quickReactionPopup.height
        const bottom = contextMenu.parent.height - margin
        contextMenu.x = clampPopupX(contextMenu, contextMenu.x)
        contextMenu.y = clampPopupY(contextMenu, contextMenu.y)
        let trayY = contextMenu.y - trayHeight - gap
        if (trayY < margin) {
            trayY = contextMenu.y + menuHeight + gap
            if (trayY + trayHeight > bottom) {
                // Neither side fits at the current anchor. Move the pair, not
                // just the tray: an independent clamp would cover menu actions.
                contextMenu.y = margin + trayHeight + gap
                trayY = margin
            }
        }
        quickReactionPopup.x = clampPopupX(
            quickReactionPopup, contextMenu.x + contextMenu.width - quickReactionPopup.width)
        quickReactionPopup.y = trayY
    }

    function openMessageMenuFromButton() {
        ensureActions()
        const mapped = messageMenuButton.mapToItem(
            contextMenu.parent, messageMenuButton.width, messageMenuButton.height)
        openMessageMenuAt(mapped.x - contextMenu.width, mapped.y + 2)
    }

    function openReactionTray(anchorItem) {
        ensureActions()
        contextMenu.close()
        fullReactionPicker.close()
        quickReactionPopup.pairedWithMenu = false
        const mapped = anchorItem.mapToItem(
            quickReactionPopup.parent, anchorItem.width / 2, anchorItem.height / 2)
        const preferredX = root.modelData.from_me
            ? mapped.x - quickReactionPopup.width : mapped.x
        let preferredY = mapped.y - quickReactionPopup.height / 2
        quickReactionPopup.x = clampPopupX(quickReactionPopup, preferredX)
        quickReactionPopup.y = clampPopupY(quickReactionPopup, preferredY)
        quickReactionPopup.open()
    }

    function openFullReactionPicker() {
        ensureActions()
        const preferredX = quickReactionPopup.x + quickReactionPopup.width - fullReactionPicker.width
        let preferredY = quickReactionPopup.y + quickReactionPopup.height + 6
        if (preferredY + fullReactionPicker.height > fullReactionPicker.parent.height - 8)
            preferredY = quickReactionPopup.y - fullReactionPicker.height - 6
        quickReactionPopup.pairedWithMenu = false
        contextMenu.close()
        quickReactionPopup.close()
        fullReactionPicker.x = clampPopupX(fullReactionPicker, preferredX)
        fullReactionPicker.y = clampPopupY(fullReactionPicker, preferredY)
        fullReactionPicker.open()
    }

    // WhatsApp Web opens the badge into a list of who reacted and with what,
    // grouped by emoji, with the reader's own reaction offered for removal.
    function openReactionDetails() {
        if (root.reactionSummary.length === 0)
            return
        closeActionPopups()
        ensureActions()
        reactionDetailsPopup.shownEmoji = ""
        const mapped = reactionsFlow.mapToItem(
            reactionDetailsPopup.parent, 0, reactionsFlow.height)
        const preferredX = root.modelData.from_me
            ? mapped.x + reactionsFlow.width - reactionDetailsPopup.width : mapped.x
        reactionDetailsPopup.x = clampPopupX(reactionDetailsPopup, preferredX)
        reactionDetailsPopup.y = clampPopupY(reactionDetailsPopup, mapped.y + 6)
        reactionDetailsPopup.open()
    }

    function reactWith(emoji) {
        backend.reactMessage(root.modelData.id, root.modelData.sender_jid || "", emoji)
        closeActionPopups()
    }

    function closeActionPopups() {
        if (contextMenu) contextMenu.close()
        if (quickReactionPopup) quickReactionPopup.close()
        if (fullReactionPicker) fullReactionPicker.close()
        if (reactionDetailsPopup) reactionDetailsPopup.close()
    }

    // The first message of a calendar day carries the date above it, which is
    // where WhatsApp Web puts one day's end and the next one's start.
    readonly property bool startsDay: Boolean(modelData.starts_day)
    readonly property real daySeparatorHeight: startsDay ? 42 : 0
    readonly property int unreadCount: actionsEnabled ? Number(modelData.unread_separator_count || 0) : 0
    readonly property real unreadSeparatorHeight: unreadCount > 0 ? 52 : 0

    width: ListView.view ? ListView.view.width : 640
    implicitHeight: root.daySeparatorHeight + root.unreadSeparatorHeight
        + bubble.implicitHeight + (reactionsFlow.visible ? 24 : 4)

    Rectangle {
        id: daySeparator
        objectName: "messageDaySeparator"
        visible: root.startsDay
        height: visible ? 26 : 0
        width: daySeparatorLabel.implicitWidth + 24
        x: Math.round((root.width - width) / 2)
        y: 8
        radius: 8
        color: Theme.daySeparator
        Label {
            id: daySeparatorLabel
            objectName: "messageDaySeparatorLabel"
            anchors.centerIn: parent
            text: RowTime.daySeparator(root.modelData.timestamp)
            color: Theme.textMuted
            font.pixelSize: 13
            Accessible.name: text
        }
    }

    Rectangle {
        id: unreadSeparator
        objectName: "messageUnreadSeparator"
        visible: root.unreadCount > 0
        x: 0
        y: root.daySeparatorHeight
        width: root.width
        height: 44
        color: Theme.unreadSeparatorBand

        Rectangle {
            anchors.centerIn: parent
            width: Math.min(parent.width - 16, unreadLabel.implicitWidth + 28)
            height: 32
            radius: height / 2
            color: Theme.unreadSeparator

            Label {
                id: unreadLabel
                objectName: "messageUnreadSeparatorLabel"
                anchors.centerIn: parent
                width: Math.min(implicitWidth, parent.width - 20)
                text: root.unreadCount === 1 ? qsTr("1 unread message")
                    : qsTr("%1 unread messages").arg(root.unreadCount)
                font.pixelSize: 12
                font.weight: Font.DemiBold
                color: Theme.text
                elide: Text.ElideRight
                Accessible.role: Accessible.StaticText
                Accessible.name: text
            }
        }
    }

    // Attachments that arrived through history synchronisation carry no file,
    // so the conversation fetches the ones it is showing. The view only builds
    // delegates near the viewport, which bounds how much is pulled at once.
    // Videos and documents are allowed to be larger than pictures, but not
    // unlimited: nobody wants a conversation to pull a gigabyte in the
    // background because it scrolled past.
    readonly property int autoDownloadCeiling: {
        switch (modelData.kind) {
        case "image":
        case "sticker": return 8 * 1024 * 1024
        case "video":
        case "document": return 25 * 1024 * 1024
        default: return 0
        }
    }
    readonly property bool needsAutoDownload: autoDownloadCeiling > 0 && !hasMedia
        && Number(modelData.media_size || 0) <= autoDownloadCeiling
    function fetchMediaIfNeeded() {
        if ((needsAutoDownload || (stickerPreviewFailed && actionsEnabled && !modelData.revoked
                && Number(modelData.media_size || 0) <= autoDownloadCeiling)) && modelData.id)
            backend.ensureMedia(modelData.id)
    }
    onStickerPreviewFailedChanged: if (stickerPreviewFailed) fetchMediaIfNeeded()
    Connections {
        target: backend
        function onMediaReady(messageId, path) {
            if (root.stickerKind && messageId === root.modelData.id)
                ++root.stickerRevision
        }
    }
    Component.onCompleted: fetchMediaIfNeeded()
    onModelDataChanged: {
        if (actionMessageId !== "" && actionMessageId !== String(modelData.id || ""))
            releaseActions()
        fetchMediaIfNeeded()
    }

    Rectangle {
        id: bubble
        objectName: "messageBubble"
        implicitWidth: root.contentWidth + 2 * root.horizontalPadding
        width: implicitWidth
        // The meta line is placed rather than stacked, so a short message can
        // keep its timestamp beside the text instead of below it.
        implicitHeight: messageContent.implicitHeight + 14 + (root.metaBeside ? 0 : metaRow.height + 2)
        // Positioned, never anchored. Anchoring one side or the other by
        // sender needs two bindings, and they do not change together: while a
        // reused delegate is rebound to a message from the other side, both
        // anchors are set for an instant. That stretches the bubble across the
        // conversation and discards its width binding for good.
        x: root.modelData.from_me ? Math.max(0, root.width - width - 40) : 40
        y: root.daySeparatorHeight + root.unreadSeparatorHeight
        color: root.stickerKind
            ? "transparent"
            : (root.modelData.from_me ? Theme.outgoingBubble : Theme.incomingBubble)
        radius: Theme.radiusLarge
        // The corner the tail grows out of is square in WhatsApp Web, and the
        // tail continues the bubble's own top edge. Rounding it left the tail
        // hanging beside the bubble with a notch between the two.
        topLeftRadius: (messageTail.visible && !root.modelData.from_me) ? 0 : radius
        topRightRadius: (messageTail.visible && root.modelData.from_me) ? 0 : radius
        border.color: root.navigationHighlighted ? Theme.primary : Theme.bubbleBorder
        border.width: root.navigationHighlighted ? 2 : ((root.modelData.from_me || root.stickerKind) ? 0 : 1)

        HoverHandler { id: bubbleHover }

        Canvas {
            id: messageTail
            objectName: "messageTail"
            // Measured off WhatsApp Web at a 1.25 device scale: the tail is 8
            // device pixels wide and 14 tall, and its flat edge is the bubble's
            // own top edge.
            width: 6.4
            height: 11.2
            y: 0
            x: root.modelData.from_me ? bubble.width : -width
            property color fillColor: bubble.color
            property color edgeColor: bubble.border.width > 0 ? bubble.border.color : bubble.color
            visible: root.modelData.kind !== "system" && !root.stickerKind

            renderStrategy: Canvas.Immediate
            antialiasing: true
            onFillColorChanged: requestPaint()
            onEdgeColorChanged: requestPaint()
            onPaint: {
                const ctx = getContext("2d")
                ctx.reset()
                ctx.clearRect(0, 0, width, height)
                ctx.fillStyle = fillColor
                ctx.beginPath()
                if (root.modelData.from_me) {
                    ctx.moveTo(0, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(0, height)
                } else {
                    ctx.moveTo(width, 0)
                    ctx.lineTo(0, 0)
                    ctx.lineTo(width, height)
                }
                ctx.closePath()
                ctx.fill()
                // A received bubble is outlined, so the tail has to carry that
                // line along its two outer edges or the outline stops dead at
                // the corner.
                if (bubble.border.width > 0) {
                    ctx.strokeStyle = edgeColor
                    ctx.lineWidth = bubble.border.width
                    ctx.beginPath()
                    if (root.modelData.from_me) {
                        ctx.moveTo(width, 0)
                        ctx.lineTo(0, height)
                    } else {
                        ctx.moveTo(0, 0)
                        ctx.lineTo(width, height)
                    }
                    ctx.stroke()
                }
            }
        }

        Label {
            objectName: "voiceDuration"
            // Recordings synced from history before lengths were stored report
            // nothing; showing "0:00" would just be wrong.
            visible: root.audioKind && (root.playingThis || Number(root.modelData.media_duration || 0) > 0)
            x: root.horizontalPadding
            y: bubble.height - height - 6
            text: root.playingThis && Playback.duration > 0
                ? root.formatClock(Playback.position)
                : root.formatClock(Number(root.modelData.media_duration || 0) * 1000)
            color: Theme.textMuted
            font.pixelSize: root.metaFontSize
        }

        Row {
            id: metaRow
            spacing: 4
            x: bubble.width - width - root.horizontalPadding
            y: bubble.height - height - 6

            // WhatsApp Web marks a starred message with a small star next to
            // the clock, which is the only place the state is visible once the
            // menu is closed.
            TintedIcon {
                objectName: "starredMark"
                visible: Boolean(root.modelData.starred)
                width: root.metaFontSize
                height: root.metaFontSize
                source: Qt.resolvedUrl("icons/star-filled.svg")
                tint: Theme.textMuted
                anchors.verticalCenter: parent.verticalCenter
            }
            Label {
                visible: Boolean(root.modelData.edited)
                text: qsTr("edited")
                color: Theme.textMuted
                font.pixelSize: root.metaFontSize
                anchors.verticalCenter: parent.verticalCenter
            }
            Label {
                text: Qt.formatTime(new Date(root.modelData.timestamp), "HH:mm")
                color: Theme.textMuted
                font.pixelSize: root.metaFontSize
                anchors.verticalCenter: parent.verticalCenter
            }
            ReadReceipt {
                objectName: "readReceipt"
                visible: Boolean(root.modelData.from_me)
                status: root.modelData.status || ""
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        Column {
            id: messageContent
            x: root.horizontalPadding
            y: 7
            width: root.contentWidth
            spacing: 3

            Label {
                id: senderLabel
                // Only a group needs to say who is talking. Repeating the other
                // person's name above every message in a one-to-one chat is
                // noise, and WhatsApp does not do it.
                // A group bubble is always labelled, as WhatsApp Web labels it.
                // Older synced messages can arrive with no push name at all, and
                // a blank line above the bubble reads as a rendering fault, so
                // the number stands in until the name is known.
                readonly property string senderNumber: {
                    const jid = String(root.modelData.sender_jid || "")
                    const user = jid.indexOf("@") > 0 ? jid.substring(0, jid.indexOf("@")) : jid
                    return /^[0-9]{6,}$/.test(user) ? "+" + user : ""
                }
                visible: !root.modelData.from_me && text.length > 0
                    && String(root.modelData.chat_jid || "").endsWith("@g.us")
                width: root.contentWidth
                text: root.modelData.sender_name || senderNumber
                color: Theme.primary
                font.pixelSize: 12
                font.weight: Font.DemiBold
                elide: Text.ElideRight
                maximumLineCount: 1
            }

            // A forward is labelled rather than shown as a quote: the reader
            // needs to know the words are not the sender's own. WhatsApp
            // switches to "Forwarded many times" once a chain gets long.
            Row {
                id: forwardedMark
                objectName: "forwardedMark"
                spacing: 4
                visible: Number(root.modelData.forwarding_score || 0) > 0
                height: visible ? forwardedLabel.implicitHeight : 0
                TintedIcon {
                    width: 13
                    height: 13
                    source: Qt.resolvedUrl("icons/forward.svg")
                    tint: Theme.textMuted
                    anchors.verticalCenter: parent.verticalCenter
                }
                Label {
                    id: forwardedLabel
                    objectName: "forwardedLabel"
                    text: Number(root.modelData.forwarding_score || 0) >= 5
                        ? qsTr("Forwarded many times")
                        : qsTr("Forwarded")
                    color: Theme.textMuted
                    font.pixelSize: 12
                    font.italic: true
                    anchors.verticalCenter: parent.verticalCenter
                }
            }

            Rectangle {
                id: replyBox
                visible: root.hasReply
                // What the quoted message would need if it were not elided.
                readonly property real naturalWidth: Math.max(replySender.implicitWidth, replyLabel.implicitWidth + (root.modelData.reply_view_once ? 21 : 0)) + 19
                width: root.contentWidth
                height: visible ? Math.max(44, replyPreviewColumn.implicitHeight + 10) : 0
                color: Theme.replyBackground
                radius: 5
                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    width: 4
                    radius: 2
                    color: Theme.primary
                }
                Column {
                    id: replyPreviewColumn
                    anchors.left: parent.left
                    anchors.leftMargin: 11
                    anchors.right: parent.right
                    anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1
                    Label {
                        id: replySender
                        width: parent.width
                        text: root.modelData.reply_from_me ? qsTr("You") : (root.modelData.reply_sender || qsTr("Message"))
                        color: Theme.primary
                        font.pixelSize: 11
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Row {
                        width: parent.width
                        spacing: root.modelData.reply_view_once ? 5 : 0
                        TintedIcon {
                            visible: Boolean(root.modelData.reply_view_once)
                            width: visible ? 16 : 0
                            height: 16
                            source: Qt.resolvedUrl("icons/view-once.svg")
                            tint: Theme.textMuted
                        }
                        Label {
                            id: replyLabel
                            width: parent.width - (root.modelData.reply_view_once ? 21 : 0)
                            text: root.modelData.reply_view_once ? qsTr("View once message")
                                : Theme.emojiRichText(root.modelData.reply_preview || qsTr("Replied message"))
                            color: Theme.textMuted
                            font.pixelSize: 12
                            elide: Text.ElideRight
                            maximumLineCount: 2
                            wrapMode: Text.Wrap
                            textFormat: Text.RichText
                        }
                    }
                }

                Button {
                    objectName: "quotedMessagePreview"
                    anchors.fill: parent
                    flat: true
                    focusPolicy: Qt.TabFocus
                    Accessible.name: root.modelData.reply_view_once ? qsTr("View once message. Open WhatsApp on your phone.") : qsTr("Go to quoted message")
                    background: Item {}
                    contentItem: Item {}
                    onClicked: {
                        if (root.modelData.reply_view_once)
                            root.viewOnceNoticeRequested()
                        else
                            root.quotedMessageRequested(String(root.modelData.reply_to || ""))
                    }

                    HoverHandler {
                        cursorShape: Qt.PointingHandCursor
                    }
                }
            }

            Item {
                id: mediaFrame
                // A cached preview file can be missing or unreadable. Falling
                // back to the descriptive row keeps the message usable instead
                // of leaving an empty frame in the conversation.
                readonly property bool previewReady: root.hasPreview && mediaImage.status !== Image.Error
                // History synchronisation strips the poster frame from videos.
                // A downloaded video still belongs on screen as a video rather
                // than as a line of text.
                readonly property bool videoPlaceholder: root.modelData.kind === "video" && !previewReady
                readonly property bool stickerPlaceholder: root.stickerKind && !previewReady
                visible: previewReady || videoPlaceholder || stickerPlaceholder
                x: -root.mediaOutset
                width: root.mediaWidth
                height: visible ? (root.stickerKind || previewReady ? root.mediaDisplayHeight : 150) : 0

                Column {
                    visible: mediaFrame.stickerPlaceholder
                    anchors.centerIn: parent
                    width: parent.width
                    spacing: 8
                    Label {
                        width: parent.width
                        text: qsTr("Sticker unavailable")
                        textFormat: Text.PlainText
                        wrapMode: Text.Wrap
                        horizontalAlignment: Text.AlignHCenter
                        color: Theme.textMuted
                    }
                    Button {
                        objectName: "stickerRetry"
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: qsTr("Retry")
                        enabled: root.actionsEnabled && !root.modelData.revoked
                        Accessible.name: qsTr("Retry loading sticker")
                        onClicked: backend.downloadMedia(root.modelData.id)
                    }
                }

                Rectangle {
                    anchors.fill: parent
                    visible: mediaFrame.videoPlaceholder
                    color: Theme.dark ? "#0C1418" : "#2A3942"
                }

                Image {
                    id: mediaImage
                    objectName: "messageMedia"
                    anchors.fill: parent
                    visible: mediaFrame.previewReady && (!animationLoader.item || !animationLoader.item.hasFrame)
                    source: root.hasPreview ? Theme.fileUrl(root.previewPath)
                        + (root.stickerKind ? "#sticker-" + root.stickerRevision : "") : ""
                    fillMode: root.modelData.kind === "sticker" ? Image.PreserveAspectFit : Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    smooth: true
                    // Limit decoding, not the original file. The viewer still
                    // opens the full-resolution image for zooming.
                    // Error collapses the frame; deriving the decode size
                    // from that frame would reload the failed source, change
                    // status, and reopen the frame in a binding loop. These
                    // metadata-based bounds do not depend on image status.
                    sourceSize: Qt.size(Math.ceil(root.mediaWidth * root.Screen.devicePixelRatio),
                                        Math.ceil(root.mediaDisplayHeight * root.Screen.devicePixelRatio))
                    mipmap: root.previewIsThumbnail
                    Accessible.name: root.modelData.body || root.mediaLabel
                }

                Loader {
                    id: animationLoader
                    anchors.fill: parent
                    active: root.animationRunning && root.hasMedia
                    sourceComponent: InlineAnimation {
                        source: Theme.fileUrl(root.animatedSticker ? root.modelData.sticker_source : root.modelData.media_path)
                        sticker: root.animatedSticker
                        onFailed: message => root.animationFailed(message)
                    }
                }

                Rectangle {
                    objectName: "mediaPlayBadge"
                    visible: (root.modelData.kind === "video" || root.animatedSticker)
                        && (!root.animationRunning || mediaOpenButton.hovered || mediaOpenButton.activeFocus)
                    anchors.centerIn: parent
                    width: root.animatedSticker ? 32 : 48
                    height: width
                    radius: width / 2
                    color: "#99000000"
                    TintedIcon {
                        anchors.centerIn: parent
                        width: root.animatedSticker ? 18 : 24
                        height: width
                        source: (root.animatedKind ? root.animationRunning : root.playingThis && Playback.playing)
                            ? Qt.resolvedUrl("icons/pause.svg") : Qt.resolvedUrl("icons/play.svg")
                        tint: "#FFFFFF"
                    }
                }

                Rectangle {
                    visible: root.gifKind
                    anchors.left: parent.left; anchors.bottom: parent.bottom; anchors.margins: 8
                    width: 32; height: 20; radius: 4
                    color: "#99000000"
                    Label { anchors.centerIn: parent; text: "GIF"; color: "white"; font.pixelSize: 11; font.bold: true }
                }

                Button {
                    objectName: "mediaOverlayAction"
                    z: 2
                    visible: root.visualKind && root.modelData.kind !== "video" && !root.hasMedia
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: 8
                    height: 26
                    padding: 9
                    text: qsTr("Download")
                    font.pixelSize: root.metaFontSize
                    Accessible.name: qsTr("Download %1").arg(root.mediaLabel)
                    contentItem: Label {
                        text: parent.text
                        color: "#FFFFFF"
                        font: parent.font
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    background: Rectangle {
                        radius: height / 2
                        color: parent.down ? "#CC000000" : "#99000000"
                    }
                    onClicked: backend.downloadMedia(root.modelData.id)
                }

                Button {
                    id: mediaOpenButton
                    objectName: "messageMediaOpenArea"
                    z: 1
                    anchors.fill: parent
                    enabled: root.modelData.kind === "video" || mediaFrame.previewReady
                    Accessible.name: root.animatedKind
                        ? (root.animationRunning ? qsTr("Pause %1").arg(root.mediaLabel) : qsTr("Play %1").arg(root.mediaLabel))
                        : qsTr("View %1").arg(root.mediaLabel)
                    background: Rectangle { color: "transparent"; border.width: parent.activeFocus ? 2 : 0; border.color: Theme.primary }
                    contentItem: Item {}
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    onClicked: root.openVisualMedia()
                }
            }

            // Voice notes and audio files play in place.
            Loader {
                id: voiceRow
                visible: root.audioKind
                active: root.audioKind
                width: root.contentWidth
                height: item ? item.implicitHeight : 0
                sourceComponent: RowLayout {
                    objectName: "voiceRow"
                    spacing: 8

                    ThemedToolButton {
                        objectName: "voicePlayButton"
                        Layout.preferredWidth: 30
                        Layout.preferredHeight: 30
                        iconSource: root.playingThis && Playback.playing
                            ? Qt.resolvedUrl("icons/pause.svg") : Qt.resolvedUrl("icons/play.svg")
                        iconSize: 18
                        padding: 0
                        Accessible.name: root.playingThis && Playback.playing
                            ? qsTr("Pause voice message")
                            : qsTr("Play voice message")
                        // WhatsApp Web draws the play control as a bare glyph on the
                        // bubble. A filled circle behind it read as a button pasted
                        // onto the message.
                        background: null
                        onClicked: Playback.start(root.modelData.id, root.modelData.media_path || "", false)
                    }

                    Item {
                        objectName: "voiceProgress"
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        Layout.preferredHeight: 26

                        Row {
                            id: waveform
                            objectName: "voiceWaveform"
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            height: 24
                            spacing: 2
                            Repeater {
                                model: root.waveformBars
                                Rectangle {
                                    required property int index
                                    required property var modelData
                                    width: Math.max(1, (waveform.width - waveform.spacing * (root.waveformBars.length - 1))
                                        / Math.max(1, root.waveformBars.length))
                                    height: Math.max(3, Math.min(24, modelData * 0.24))
                                    radius: width / 2
                                    anchors.verticalCenter: parent.verticalCenter
                                    // Everything up to the playing position is
                                    // filled in, the way a played recording reads.
                                    color: root.playingThis && (index / Math.max(1, root.waveformBars.length)) <= root.playProgress
                                        ? Theme.primary
                                        : (root.modelData.from_me ? Qt.darker(Theme.outgoingBubble, 1.35) : Theme.scrollbarHandle)
                                }
                            }
                        }

                        Rectangle {
                            visible: root.playingThis
                            width: 9
                            height: 9
                            radius: 4.5
                            color: Theme.primary
                            anchors.verticalCenter: parent.verticalCenter
                            x: Math.max(0, Math.min(parent.width - width, parent.width * root.playProgress - width / 2))
                        }

                        MouseArea {
                            anchors.fill: parent
                            enabled: root.playingThis && Playback.duration > 0
                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                            onClicked: mouse => Playback.seek(Playback.duration * (mouse.x / Math.max(1, width)))
                        }
                    }

                    Avatar {
                        id: voiceAvatar
                        objectName: "voiceAvatar"
                        Layout.preferredWidth: 44
                        Layout.preferredHeight: 44
                        diameter: 44
                        title: root.modelData.from_me ? root.ownTitle : root.chatTitle
                        source: root.modelData.from_me ? "" : root.chatAvatarSource
                        fallbackIdentity: Boolean(root.modelData.from_me)
                    }
                }
            }

            // A shared contact, shown as a card rather than a bare line.
            Rectangle {
                id: contactCard
                objectName: "contactCard"
                visible: root.contactKind
                readonly property real naturalWidth: Math.max(contactNameLabel.implicitWidth,
                    contactPhoneLabel.implicitWidth) + 74
                width: root.contentWidth
                height: visible ? 62 : 0
                radius: 6
                color: root.modelData.from_me ? Qt.darker(Theme.outgoingBubble, 1.06) : Theme.replyBackground

                Avatar {
                    id: contactAvatar
                    anchors.left: parent.left
                    anchors.leftMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    width: 42
                    height: 42
                    diameter: 42
                    title: root.contactName
                    fallbackIdentity: root.contactName === ""
                }

                Column {
                    anchors.left: contactAvatar.right
                    anchors.leftMargin: 10
                    anchors.right: contactAction.left
                    anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1
                    Label {
                        id: contactNameLabel
                        width: parent.width
                        text: root.contactName || qsTr("Contact")
                        color: Theme.text
                        font.pixelSize: root.captionFontSize
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Label {
                        id: contactPhoneLabel
                        width: parent.width
                        visible: root.contactPhone !== "" || Number(root.modelData.contact_count || 0) > 1
                        text: Number(root.modelData.contact_count || 0) > 1
                            ? qsTr("%1 contacts").arg(root.modelData.contact_count)
                            : root.contactPhone
                        color: Theme.textMuted
                        font.pixelSize: root.metaFontSize
                        elide: Text.ElideRight
                    }
                }

                ToolButton {
                    id: contactAction
                    objectName: "contactAction"
                    anchors.right: parent.right
                    anchors.rightMargin: 4
                    anchors.verticalCenter: parent.verticalCenter
                    visible: root.contactPhone !== ""
                    width: visible ? 66 : 0
                    height: 30
                    flat: true
                    text: qsTr("Message")
                    font.pixelSize: root.metaFontSize
                    Accessible.name: qsTr("Message %1").arg(root.contactName)
                    // The resolver wants digits, not the punctuation a vCard
                    // uses to make a number readable.
                    onClicked: backend.startChat(root.contactPhone.replace(/[^0-9]/g, ""))
                }
            }

            // A shared place, with the map picture its sender included.
            Rectangle {
                id: locationCard
                objectName: "locationCard"
                visible: root.locationKind
                width: root.contentWidth
                height: visible ? (locationMap.visible ? 150 : 0) + locationText.implicitHeight + 16 : 0
                radius: 6
                clip: true
                color: root.modelData.from_me ? Qt.darker(Theme.outgoingBubble, 1.06) : Theme.replyBackground

                Image {
                    id: locationMap
                    objectName: "locationMap"
                    visible: Boolean(root.modelData.media_thumbnail) && status !== Image.Error
                    source: visible ? Theme.fileUrl(root.modelData.media_thumbnail) : ""
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: visible ? 150 : 0
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    Accessible.name: qsTr("Map of the shared place")
                }

                Column {
                    id: locationText
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: locationMap.visible ? locationMap.bottom : parent.top
                    anchors.leftMargin: 10
                    anchors.rightMargin: 10
                    anchors.topMargin: 8
                    spacing: 1
                    Label {
                        width: parent.width
                        text: root.modelData.body || qsTr("Location")
                        color: Theme.text
                        font.pixelSize: root.captionFontSize
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                        maximumLineCount: 2
                        wrapMode: Text.Wrap
                    }
                    Label {
                        width: parent.width
                        visible: root.hasCoordinates
                        text: Number(root.modelData.latitude || 0).toFixed(5) + ", "
                            + Number(root.modelData.longitude || 0).toFixed(5)
                        color: Theme.textMuted
                        font.pixelSize: root.metaFontSize
                        elide: Text.ElideRight
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    enabled: root.hasCoordinates
                    cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                    // There is no map inside the application, so the place is
                    // handed to whatever the desktop uses to show maps.
                    onClicked: Qt.openUrlExternally("https://www.openstreetmap.org/?mlat="
                        + Number(root.modelData.latitude || 0) + "&mlon=" + Number(root.modelData.longitude || 0)
                        + "#map=17/" + Number(root.modelData.latitude || 0) + "/" + Number(root.modelData.longitude || 0))
                }
            }

            Loader {
                id: documentCard
                active: root.documentKind
                visible: active
                x: -root.mediaOutset
                width: root.contentWidth + 2 * root.mediaOutset
                sourceComponent: DocumentCard {
                    message: root.modelData
                    enabled: root.actionsEnabled && !root.selectionActive && !root.modelData.revoked
                }
            }

            RowLayout {
                id: fileRow
                // Files that have no picture of their own keep the descriptive
                // row; a photo or video only falls back to it without a preview.
                visible: (root.mediaKind && !root.audioKind && !root.documentKind && !mediaFrame.visible)
                    || root.modelData.kind === "poll" || root.modelData.kind === "event"
                width: root.contentWidth
                spacing: 8
                Rectangle {
                    Layout.preferredWidth: 30
                    Layout.preferredHeight: 30
                    radius: 15
                    color: Theme.surfaceMuted
                    TintedIcon {
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        source: root.modelData.kind === "video" ? Qt.resolvedUrl("icons/play.svg")
                            : root.modelData.kind === "document" ? Qt.resolvedUrl("icons/document.svg")
                            : root.modelData.kind === "poll" ? Qt.resolvedUrl("icons/poll.svg")
                            : root.modelData.kind === "event" ? Qt.resolvedUrl("icons/calendar.svg")
                            : Qt.resolvedUrl("icons/gallery.svg")
                        tint: Theme.icon
                    }
                }
                Label {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    text: root.mediaLabel
                    color: Theme.text
                    font.pixelSize: root.captionFontSize
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }
                ToolButton {
                    objectName: "interactiveMessageAction"
                    visible: root.modelData.kind === "poll" || root.modelData.kind === "event"
                    text: root.modelData.kind === "poll" ? qsTr("View poll") : qsTr("View event")
                    enabled: root.actionsEnabled && !root.selectionActive && !root.modelData.revoked
                    onClicked: root.interactiveRequested(root.modelData)
                }
                ToolButton {
                    objectName: root.documentKind ? "fallbackMediaAction" : "mediaAction"
                    visible: root.mediaKind
                    text: root.modelData.kind === "video"
                        ? qsTr("Play")
                        : root.hasMedia ? qsTr("Open") : qsTr("Download")
                    flat: true
                    font.pixelSize: root.metaFontSize
                    Accessible.name: text + " " + root.mediaLabel
                    onClicked: {
                        if (root.modelData.kind === "video")
                            Playback.start(root.modelData.id, root.modelData.media_path || "", true)
                        else if (root.visualKind && root.hasPreview)
                            root.imagePreviewRequested(root.modelData)
                        else if (root.hasMedia)
                            backend.openFile(root.modelData.media_path)
                        else
                            backend.downloadMedia(root.modelData.id)
                    }
                }
            }

            // The preview the sender's client resolved when the message was
            // written. It is part of the message, so showing it contacts nobody.
            Rectangle {
                id: linkPreview
                objectName: "linkPreview"
                visible: !root.viewOnceKind && Boolean(root.modelData.link_url)
                readonly property real naturalWidth: Math.max(
                    linkTitle.visible ? linkTitle.implicitWidth + 20 : 0,
                    linkDescription.visible ? linkDescription.implicitWidth + 20 : 0,
                    linkHostLabel.implicitWidth + 20)
                width: root.contentWidth
                height: visible ? linkColumn.y + linkColumn.implicitHeight + 8 : 0
                color: root.modelData.from_me ? Qt.darker(Theme.outgoingBubble, 1.06) : Theme.replyBackground
                radius: 6
                clip: true

                Image {
                    id: linkImage
                    objectName: "linkPreviewImage"
                    visible: Boolean(root.modelData.link_thumbnail) && status !== Image.Error
                    source: visible ? Theme.fileUrl(root.modelData.link_thumbnail) : ""
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: visible ? Math.min(240, Math.max(120, width * 0.5625)) : 0
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    sourceSize: Qt.size(Math.ceil(width * root.Screen.devicePixelRatio),
                                        Math.ceil(height * root.Screen.devicePixelRatio))
                    Accessible.name: root.modelData.link_title || root.linkHost(root.modelData.link_url)
                }

                Column {
                    id: linkColumn
                    y: linkImage.visible ? linkImage.height + 6 : 8
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.leftMargin: 10
                    anchors.rightMargin: 10
                    spacing: 2

                    Label {
                        id: linkTitle
                        visible: Boolean(root.modelData.link_title)
                        width: parent.width
                        text: root.modelData.link_title || ""
                        color: Theme.text
                        font.pixelSize: root.captionFontSize
                        font.weight: Font.DemiBold
                        elide: Text.ElideRight
                        maximumLineCount: 2
                        wrapMode: Text.Wrap
                    }
                    Label {
                        id: linkDescription
                        visible: Boolean(root.modelData.link_description)
                        width: parent.width
                        text: root.modelData.link_description || ""
                        color: Theme.textMuted
                        font.pixelSize: root.metaFontSize + 1
                        elide: Text.ElideRight
                        maximumLineCount: 2
                        wrapMode: Text.Wrap
                    }
                    Label {
                        id: linkHostLabel
                        width: parent.width
                        text: root.linkHost(root.modelData.link_url)
                        color: Theme.textMuted
                        font.pixelSize: root.metaFontSize
                        elide: Text.ElideRight
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: Qt.openUrlExternally(root.modelData.link_url)
                }
            }

            Row {
                id: viewOnceNotice
                objectName: "viewOnceNotice"
                visible: root.viewOnceKind && !root.modelData.revoked
                width: root.contentWidth
                spacing: 9
                TintedIcon {
                    width: 22
                    height: 22
                    source: Qt.resolvedUrl("icons/view-once.svg")
                    tint: Theme.textMuted
                }
                Label {
                    objectName: "viewOnceNoticeText"
                    width: parent.width - 31
                    text: qsTr("This message was set to view once. For added privacy, you can only open it on your phone.")
                    textFormat: Text.PlainText
                    wrapMode: Text.Wrap
                    color: Theme.textMuted
                    font.pixelSize: root.captionFontSize
                    font.italic: true
                    Accessible.name: text
                }
            }

            SelectableMessageText {
                id: bodyText
                visible: (Boolean(root.modelData.body) || (root.viewOnceKind && Boolean(root.modelData.revoked)))
                    && !root.contactKind && !root.locationKind && (!root.viewOnceKind || Boolean(root.modelData.revoked))
                width: root.metaBeside ? implicitWidth : root.contentWidth
                maximumWidth: root.contentMaxWidth
                plainText: root.modelData.revoked ? qsTr("This message was deleted")
                    : root.viewOnceKind ? "" : root.modelData.body || ""
                mentions: root.modelData.revoked || root.viewOnceKind ? [] : root.modelData.mentions || []
                mentionNavigationEnabled: root.actionsEnabled && !root.selectionActive
                onMentionActivated: (jid, name) => root.mentionRequested(jid, name)
                color: root.modelData.revoked ? Theme.textMuted : Theme.text
                font.pixelSize: root.bodyFontSize
                font.italic: Boolean(root.modelData.revoked)
            }

            Label {
                id: unsupportedLabel
                visible: !Boolean(root.modelData.body) && !root.mediaKind && !root.viewOnceKind && root.modelData.kind !== "system"
                text: qsTr("Unsupported message")
                color: Theme.textMuted
                font.pixelSize: root.captionFontSize
                font.italic: true
            }

        }

        ToolButton {
            id: messageMenuButton
            objectName: "messageMenuButton"
            width: 32
            height: 32
            x: bubble.width - width - 1
            y: 1
            z: 12
            focusPolicy: Qt.TabFocus
            opacity: bubbleHover.hovered || hovered || activeFocus || (root.contextMenu && root.contextMenu.opened) ? 1 : 0
			visible: root.actionsEnabled && !root.selectionActive
			enabled: visible && root.modelData.kind !== "system"
            Accessible.name: qsTr("Message actions")
            Accessible.description: root.contextMenu && root.contextMenu.opened ? qsTr("Menu open") : qsTr("Menu closed")
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            onClicked: root.openMessageMenuFromButton()
            Behavior on opacity { NumberAnimation { duration: 90 } }
            contentItem: Item {
                TintedIcon {
                    objectName: "messageMenuIcon"
                    anchors.centerIn: parent
                    width: 16
                    height: 16
                    source: Qt.resolvedUrl("icons/chevron-right.svg")
                    tint: Theme.icon
                    rotation: 90
                }
            }
            background: Rectangle {
                objectName: "messageMenuBackground"
                radius: 8
                color: parent.down ? Theme.pressedRow : "transparent"
                border.width: parent.activeFocus ? 2 : 0
                border.color: Theme.primary
            }
        }
    }

    ToolButton {
        id: messageReactionButton
        objectName: "messageReactionButton"
        width: 36
        height: 36
        x: root.modelData.from_me ? bubble.x - width - 8 : bubble.x + bubble.width + 8
        y: bubble.y + Math.max(0, (bubble.height - height) / 2)
        z: 8
        focusPolicy: Qt.TabFocus
        opacity: bubbleHover.hovered || hovered || activeFocus
            || (root.quickReactionPopup && root.quickReactionPopup.opened && !root.quickReactionPopup.pairedWithMenu) ? 1 : 0
		visible: root.actionsEnabled && !root.selectionActive
		enabled: visible && root.modelData.kind !== "system" && !root.modelData.revoked
        Accessible.name: qsTr("React to message")
        ToolTip.visible: hovered
        ToolTip.text: Accessible.name
        onClicked: root.openReactionTray(messageReactionButton)
        Behavior on opacity { NumberAnimation { duration: 90 } }
        contentItem: Item {
            TintedIcon {
                objectName: "messageReactionIcon"
                anchors.centerIn: parent
                width: 16
                height: 16
                source: Qt.resolvedUrl("icons/smile.svg")
                tint: Theme.icon
            }
        }
        background: Item {
            Rectangle {
                width: 26
                height: 26
                anchors.centerIn: parent
                anchors.horizontalCenterOffset: -1
                anchors.verticalCenterOffset: 1
                radius: 13
                color: Theme.dark ? "#48000000" : "#22000000"
            }
            Rectangle {
                width: 26
                height: 26
                anchors.centerIn: parent
                radius: 13
                color: Theme.surfaceRaised
                border.color: messageReactionButton.activeFocus ? Theme.primary : Theme.border
                border.width: messageReactionButton.activeFocus ? 2 : 1
            }
        }
    }

    Rectangle {
        id: reactionsFlow
        objectName: "messageReactionBadge"
        visible: root.reactionSummary.length > 0
        // Measured against WhatsApp Web: the badge is 30 device pixels tall
        // with the emoji filling 20 of them, and it sits below the bubble with
        // only its top edge under the corner.
        implicitWidth: reactionBadgeRow.implicitWidth + 12
        width: implicitWidth
        height: 24
        x: root.modelData.from_me
            ? bubble.x + bubble.width - width - 7 : bubble.x + 7
        y: bubble.y + bubble.height - 2
        z: 4
        radius: height / 2
        // The reader's own reaction looks like anybody else's here: WhatsApp
        // Web draws the same white pill whoever left it, and says which one is
        // yours in the panel the badge opens. A green pill on the bubble read
        // as a different kind of reaction altogether.
        color: Theme.surfaceRaised
        // WhatsApp Web separates the pill from what is behind it with a soft
        // shadow. Nothing in this application uses an effect, and a hairline
        // does the same work where the bubble under the badge is white.
        border.color: Theme.border
        border.width: 1

        Row {
            id: reactionBadgeRow
            anchors.centerIn: parent
            spacing: 3

            Label {
                id: reactionSummaryLabel
                objectName: "messageReactionSummary"
                anchors.verticalCenter: parent.verticalCenter
                text: root.reactionSummaryText
                font.family: Theme.emojiFontFamily
                font.pixelSize: 16
                Accessible.name: qsTr("Reactions: %1").arg(text)
            }
            Label {
                objectName: "messageReactionCount"
                anchors.verticalCenter: parent.verticalCenter
                visible: root.reactionTotal > 1
                text: root.reactionTotal
                color: Theme.textMuted
                font.pixelSize: 12
            }
        }

        TapHandler {
            // WhatsApp Web opens the list of who reacted with what. Adding a
            // reaction of one's own is the hover control and the menu, not
            // this badge.
            onTapped: root.openReactionDetails()
        }
    }

    TapHandler {
		enabled: root.actionsEnabled && !root.selectionActive
        acceptedButtons: Qt.RightButton
        onTapped: (eventPoint, button) => {
            root.ensureActions()
            const mapped = root.mapToItem(contextMenu.parent, eventPoint.position.x, eventPoint.position.y)
            root.openMessageMenuAt(mapped.x, mapped.y)
        }
    }

    Loader {
        id: actionsLoader
        active: false
        sourceComponent: Item {
            property alias contextMenu: contextMenu
            property alias quickReactionPopup: quickReactionPopup
            property alias fullReactionPicker: fullReactionPicker
            property alias reactionDetailsPopup: reactionDetailsPopup

            Connections {
                target: contextMenu
                function onXChanged() { Qt.callLater(root.positionMessagePopups) }
                function onYChanged() { Qt.callLater(root.positionMessagePopups) }
                function onHeightChanged() { Qt.callLater(root.positionMessagePopups) }
                function onImplicitHeightChanged() { Qt.callLater(root.positionMessagePopups) }
            }
            Connections {
                target: contextMenu.parent
                function onWidthChanged() { Qt.callLater(root.positionMessagePopups) }
                function onHeightChanged() { Qt.callLater(root.positionMessagePopups) }
            }

            Popup {
                id: reactionDetailsPopup
                objectName: "reactionDetailsPopup"
                parent: Overlay.overlay
                // Empty means everybody; otherwise only the people who left this
                // emoji, which is what the headings along the top switch between.
                property string shownEmoji: ""
                readonly property var shownPeople: root.reactionPeople.filter(function(person) {
                    return reactionDetailsPopup.shownEmoji === "" || person.key === reactionDetailsPopup.shownEmoji
                })
                width: 320
                implicitHeight: Math.min(384, reactionDetailsColumn.implicitHeight + 24)
                padding: 12
                modal: false
                focus: true
                // See quickReactionPopup: this is positioned inside the application
                // window, so it has to be an item in that window's overlay.
                Component.onCompleted: {
                    if (typeof popupType !== "undefined" && typeof Popup.Item !== "undefined")
                        popupType = Popup.Item
                }
                closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
                background: Rectangle {
                    radius: 12
                    color: Theme.surfaceRaised
                    border.color: Theme.border
                }

                contentItem: Column {
                    id: reactionDetailsColumn
                    spacing: 8

                    Label {
                        objectName: "reactionDetailsTitle"
                        // "1 reaction", "4 reactions": what WhatsApp Web heads the
                        // panel with. "1 emoji reactions" was neither.
                        text: root.reactionTotal === 1
                            ? qsTr("1 reaction") : qsTr("%1 reactions").arg(root.reactionTotal)
                        color: Theme.text
                        font.pixelSize: 14
                        font.weight: Font.DemiBold
                    }

                    Row {
                        spacing: 6

                        Repeater {
                            // "All" is a choice between emoji, so it appears once
                            // there is more than one to choose between.
                            model: (root.reactionSummary.length > 1
                                ? [{ emoji: "", count: root.reactionTotal }] : []).concat(root.reactionSummary)
                            delegate: Rectangle {
                                required property var modelData
                                readonly property bool current: reactionDetailsPopup.shownEmoji === String(modelData.emoji)
                                width: headingRow.implicitWidth + 18
                                height: 28
                                radius: 14
                                color: current ? Theme.selectedRow : "transparent"
                                border.color: current ? Theme.primary : Theme.border
                                border.width: 1

                                Row {
                                    id: headingRow
                                    anchors.centerIn: parent
                                    spacing: 4
                                    Label {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: String(parent.parent.modelData.emoji) === ""
                                            ? qsTr("All") : String(parent.parent.modelData.emoji)
                                        font.family: Theme.emojiFontFamily
                                        font.pixelSize: 14
                                        color: Theme.text
                                    }
                                    Label {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: Number(parent.parent.modelData.count)
                                        font.pixelSize: 12
                                        color: Theme.textMuted
                                    }
                                }

                                TapHandler {
                                    onTapped: reactionDetailsPopup.shownEmoji = String(parent.modelData.emoji)
                                }
                            }
                        }
                    }

                    ListView {
                        objectName: "reactionDetailsList"
                        width: parent.width
                        height: Math.min(288, Math.max(48, contentHeight))
                        clip: true
                        model: reactionDetailsPopup.shownPeople
                        boundsBehavior: Flickable.StopAtBounds
                        delegate: Item {
                            required property var modelData
                            width: ListView.view.width
                            height: 48

                            Avatar {
                                id: reactorAvatar
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                diameter: 36
                                title: String(parent.modelData.name || parent.modelData.number)
                                source: parent.modelData.avatar ? Theme.fileUrl(String(parent.modelData.avatar)) : ""
                                fallbackIdentity: source.toString() === ""
                            }

                            Column {
                                anchors.left: reactorAvatar.right
                                anchors.leftMargin: 10
                                anchors.right: reactorEmoji.left
                                anchors.rightMargin: 8
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: 1

                                Label {
                                    width: parent.width
                                    elide: Text.ElideRight
                                    text: parent.parent.modelData.mine
                                        ? qsTr("You")
                                        : (String(parent.parent.modelData.name)
                                            || String(parent.parent.modelData.number)
                                            || qsTr("Unknown"))
                                    color: Theme.text
                                    font.pixelSize: 14
                                }
                                Label {
                                    width: parent.width
                                    elide: Text.ElideRight
                                    // WhatsApp Web puts the number under a saved name,
                                    // and tells the reader their own can be taken back.
                                    visible: text !== ""
                                    text: parent.parent.modelData.mine
                                        ? qsTr("Click to remove")
                                        : (String(parent.parent.modelData.name) !== ""
                                            ? String(parent.parent.modelData.number) : "")
                                    color: Theme.textMuted
                                    font.pixelSize: 12
                                }
                            }

                            Label {
                                id: reactorEmoji
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: String(parent.modelData.emoji)
                                font.family: Theme.emojiFontFamily
                                font.pixelSize: 18
                            }

                            TapHandler {
                                enabled: parent.modelData !== undefined && parent.modelData.mine
                                onTapped: {
                                    root.reactWith("")
                                    reactionDetailsPopup.close()
                                }
                            }
                        }
                    }
                }
            }

            Popup {
                id: quickReactionPopup
                objectName: "quickReactionPopup"
                parent: Overlay.overlay
                property bool pairedWithMenu: false
                width: quickReactionRow.implicitWidth + 12
                height: 44
                padding: 6
                modal: false
                focus: true
                // Qt 6.8 added Popup.popupType, and 6.9 opens a plain Popup in its own
                // window by default. This one is positioned by clamping it inside the
                // application window - see clampPopupX and clampPopupY - so it has to
                // be an item in that window's overlay. The guard keeps the file
                // loadable on the Qt 6.5 the build still supports, where every popup
                // was an item already.
                Component.onCompleted: {
                    if (typeof popupType !== "undefined" && typeof Popup.Item !== "undefined")
                        popupType = Popup.Item
                }
                closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
                onClosed: {
                    if (pairedWithMenu) {
                        pairedWithMenu = false
                        contextMenu.close()
                    }
                }
                background: Item {
                    Rectangle {
                        anchors.fill: parent
                        anchors.leftMargin: 2
                        anchors.topMargin: 4
                        radius: 24
                        color: Theme.dark ? "#52000000" : "#26000000"
                    }
                    Rectangle {
                        anchors.fill: parent
                        anchors.rightMargin: 2
                        anchors.bottomMargin: 4
                        radius: 24
                        color: Theme.surfaceRaised
                        border.color: Theme.border
                    }
                }
                contentItem: Row {
                    id: quickReactionRow
                    spacing: 1
                    Repeater {
                        model: ["👍", "❤️", "😂", "😮", "😢", "🙏"]
                        ToolButton {
                            required property string modelData
                            width: 32
                            height: 32
                            focusPolicy: Qt.TabFocus
                            Accessible.name: qsTr("React with %1").arg(modelData)
                            onClicked: root.reactWith(modelData)
                            contentItem: Label {
                                text: parent.modelData
                                font.family: Theme.emojiFontFamily
                                font.pixelSize: 18
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                            background: Rectangle {
                                radius: 16
                                color: parent.down ? Theme.pressedRow
                                    : parent.hovered || parent.activeFocus ? Theme.hoverRow : "transparent"
                                border.width: parent.activeFocus ? 2 : 0
                                border.color: Theme.primary
                            }
                        }
                    }
                    ToolButton {
                        width: 32
                        height: 32
                        focusPolicy: Qt.TabFocus
                        Accessible.name: qsTr("More reactions")
                        onClicked: root.openFullReactionPicker()
                        contentItem: TintedIcon {
                            source: Qt.resolvedUrl("icons/plus.svg")
                            tint: Theme.icon
                            anchors.margins: 8
                        }
                        background: Rectangle {
                            radius: 16
                            color: parent.down ? Theme.pressedRow
                                : parent.hovered || parent.activeFocus ? Theme.hoverRow : "transparent"
                            border.width: parent.activeFocus ? 2 : 0
                            border.color: Theme.primary
                        }
                    }
                }
            }

            EmojiPicker {
                id: fullReactionPicker
                objectName: "messageReactionPicker"
                parent: Overlay.overlay
                onEmojiChosen: emoji => root.reactWith(emoji)
            }

            WhatsAppMenuPopup {
                id: contextMenu
                objectName: "messageContextMenu"
                parent: Overlay.overlay
                width: 196
                property string capturedSelection: ""
                closePolicy: Popup.CloseOnEscape
                // Qt 6.8 added Popup.popupType, and 6.9 opens a plain Popup in its own
                // window by default. This one is positioned by clamping it inside the
                // application window - see clampPopupX and clampPopupY - so it has to
                // be an item in that window's overlay. The guard keeps the file
                // loadable on the Qt 6.5 the build still supports, where every popup
                // was an item already.
                Component.onCompleted: {
                    if (typeof popupType !== "undefined" && typeof Popup.Item !== "undefined")
                        popupType = Popup.Item
                }

                WhatsAppMenuItem {
                    id: messageInfoAction
                    objectName: "messageInfoAction"
                    visible: Boolean(root.modelData.from_me)
                    height: visible ? 36 : 0
                    text: qsTr("Message info")
                    iconSource: Qt.resolvedUrl("icons/info.svg")
                    onClicked: {
                        root.closeActionPopups()
                        root.infoRequested(root.modelData)
                    }
                }

                WhatsAppMenuItem {
                    visible: Boolean(contextMenu.capturedSelection)
                    height: visible ? 36 : 0
                    text: qsTr("Copy selected text")
                    iconSource: Qt.resolvedUrl("icons/copy.svg")
                    onClicked: {
                        backend.copyText(contextMenu.capturedSelection)
                        root.closeActionPopups()
                    }
                }

                WhatsAppMenuItem {
                    visible: !Boolean(contextMenu.capturedSelection) && Boolean(root.modelData.body) && !root.viewOnceKind
                    height: visible ? 36 : 0
                    text: qsTr("Copy")
                    iconSource: Qt.resolvedUrl("icons/copy.svg")
                    onClicked: {
                        backend.copyText(root.modelData.body || "")
                        root.closeActionPopups()
                    }
                }

                WhatsAppMenuItem {
                    visible: root.modelData.kind === "image" || root.modelData.kind === "sticker"
                    height: visible ? 36 : 0
                    text: qsTr("Copy image")
                    iconSource: Qt.resolvedUrl("icons/copy.svg")
                    onClicked: {
                        root.closeActionPopups()
                        backend.copyImage(root.modelData.id, root.modelData.media_path || "")
                    }
                }

                WhatsAppMenuItem {
                    text: qsTr("Reply")
                    iconSource: Qt.resolvedUrl("icons/reply.svg")
                    onClicked: {
                        root.closeActionPopups()
                        root.replyRequested(root.modelData.id, root.modelData.body || root.mediaLabel)
                    }
                }
                WhatsAppMenuItem {
                    text: qsTr("React")
                    iconSource: Qt.resolvedUrl("icons/smile.svg")
                    onClicked: {
                        quickReactionPopup.pairedWithMenu = false
                        contextMenu.close()
                        root.openReactionTray(messageReactionButton)
                    }
                }
                WhatsAppMenuItem {
                    visible: !root.modelData.revoked
                    height: visible ? 36 : 0
                    text: qsTr("Pin")
                    iconSource: Qt.resolvedUrl("icons/pin.svg")
                    onClicked: {
                        root.closeActionPopups()
                        root.pinRequested(root.modelData.id, root.modelData.sender_jid || "",
                                          root.modelData.body || root.mediaLabel)
                    }
                }
                WhatsAppMenuItem {
                    objectName: "messageStarAction"
                    visible: !root.modelData.revoked && !root.viewOnceKind
                    height: visible ? 36 : 0
                    // The label names the result, matching how the web client flips
                    // between starring and removing a star on the same row.
                    text: root.modelData.starred ? qsTr("Unstar") : qsTr("Star")
                    iconSource: Qt.resolvedUrl(root.modelData.starred ? "icons/star-filled.svg" : "icons/star.svg")
                    onClicked: {
                        root.closeActionPopups()
                        root.starRequested(root.modelData.id, root.modelData.sender_jid || "",
                                           Boolean(root.modelData.from_me), !root.modelData.starred)
                    }
                }
                WhatsAppMenuItem {
                    objectName: "messageForwardAction"
                    visible: !root.modelData.revoked && !root.viewOnceKind
                    height: visible ? 36 : 0
                    text: qsTr("Forward")
                    iconSource: Qt.resolvedUrl("icons/forward.svg")
                    onClicked: {
                        root.closeActionPopups()
                        root.forwardRequested(root.modelData.id)
                    }
                }
                Rectangle {
                    visible: Boolean(root.modelData.from_me)
                    width: parent.width
                    height: visible ? 1 : 0
                    color: Theme.border
                }
                WhatsAppMenuItem {
                    visible: Boolean(root.modelData.from_me) && root.modelData.kind === "text" && !root.modelData.revoked
                    height: visible ? 36 : 0
                    text: qsTr("Edit")
                    iconSource: Qt.resolvedUrl("icons/edit.svg")
                    onClicked: {
                        root.closeActionPopups()
                        root.editRequested(root.modelData.id, root.modelData.body || "")
                    }
                }
                WhatsAppMenuItem {
                    visible: Boolean(root.modelData.from_me) && !root.modelData.revoked
                    height: visible ? 36 : 0
                    text: qsTr("Delete for everyone")
                    iconSource: Qt.resolvedUrl("icons/delete.svg")
                    destructive: true
                    onClicked: {
                        root.closeActionPopups()
                        root.deleteRequested(root.modelData.id, root.modelData.sender_jid || "")
                    }
                }
            }

        }
    }

    // Selection chrome sits above everything else in the row so a tap always
    // toggles, whatever it lands on.
    Rectangle {
        objectName: "messageSelectionOverlay"
        anchors.fill: parent
        z: 40
        visible: root.selectionActive
        color: root.selected ? Theme.selectedRow : "transparent"
        opacity: root.selected ? 0.55 : 1

        MouseArea {
            anchors.fill: parent
            onClicked: root.selectionToggled(root.modelData)
        }

        Rectangle {
            anchors.right: parent.right
            anchors.rightMargin: 10
            anchors.verticalCenter: parent.verticalCenter
            width: 22
            height: 24
            radius: 11
            color: root.selected ? Theme.primary : "transparent"
            border.width: root.selected ? 0 : 2
            border.color: Theme.textMuted
            TintedIcon {
                anchors.centerIn: parent
                width: 14
                height: 14
                visible: root.selected
                source: Qt.resolvedUrl("icons/check.svg")
                tint: Theme.primaryText
            }
        }
    }
}
