import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Item {
    id: root
    required property var modelData
    signal editRequested(string messageId, string body)
    signal deleteRequested(string messageId, string senderJid)
    signal replyRequested(string messageId, string body)

    readonly property bool hasReply: Boolean(modelData.reply_to)
    readonly property bool hasMedia: Boolean(modelData.media_path)
    readonly property bool mediaKind: ["image", "video", "audio", "document", "sticker"].indexOf(modelData.kind) >= 0
    readonly property bool visualKind: ["image", "video", "sticker"].indexOf(modelData.kind) >= 0
    readonly property bool audioKind: modelData.kind === "audio"

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
    readonly property real horizontalPadding: 9
    readonly property real maxBubbleWidth: Math.max(180, Math.min(root.width * 0.68, 620))
    readonly property real contentMaxWidth: maxBubbleWidth - 2 * horizontalPadding
    readonly property real mediaWidth: Math.min(contentMaxWidth, modelData.kind === "sticker" ? 160 : 300)

    // WhatsApp Web's message metrics: 14.2 px body text on a 19 px line.
    readonly property int bodyFontSize: 14
    readonly property int captionFontSize: 13
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
        && !linkPreview.visible && !contactCard.visible && !locationCard.visible
        && !reactionsFlow.visible
        && (bodyText.implicitWidth + metaRow.implicitWidth + 12) <= contentMaxWidth

    // The bubble is exactly as wide as its widest part, capped by the
    // conversation. Every term below is a natural size that does not depend on
    // the width the bubble ends up with, so the two cannot chase each other:
    // a ColumnLayout with filling children could inflate itself instead of
    // hugging the text.
    readonly property real naturalContentWidth: Math.max(
        senderLabel.visible ? Math.min(senderLabel.implicitWidth, contentMaxWidth) : 0,
        replyBox.visible ? Math.min(Math.max(replyBox.naturalWidth, 96), contentMaxWidth) : 0,
        mediaFrame.visible ? mediaWidth : 0,
        voiceRow.visible ? 232 : 0,
        fileRow.visible ? Math.min(fileRow.implicitWidth, contentMaxWidth) : 0,
        linkPreview.visible ? Math.min(Math.max(linkPreview.naturalWidth, 240), contentMaxWidth) : 0,
        contactCard.visible ? Math.min(Math.max(contactCard.naturalWidth, 220), contentMaxWidth) : 0,
        locationCard.visible ? Math.min(260, contentMaxWidth) : 0,
        bodyText.visible ? (metaBeside ? bodyText.implicitWidth + metaRow.implicitWidth + 12 : bodyText.implicitWidth) : 0,
        unsupportedLabel.visible ? unsupportedLabel.implicitWidth : 0,
        metaRow.implicitWidth,
        reactionsFlow.visible ? Math.min(reactionsFlow.implicitWidth, contentMaxWidth) : 0)
    readonly property real contentWidth: Math.min(Math.max(naturalContentWidth, 54), contentMaxWidth)

    readonly property string mediaLabel: {
        switch (modelData.kind) {
        case "image": return qsTr("Photo")
        case "video": return qsTr("Video")
        case "audio": return qsTr("Voice message")
        case "document": return modelData.media_name || qsTr("Document")
        case "sticker": return qsTr("Sticker")
        case "contact": return qsTr("Contact")
        case "location": return qsTr("Location")
        case "poll": return qsTr("Poll")
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

    readonly property real playProgress: playingThis && Playback.duration > 0
        ? Playback.position / Playback.duration : 0

    // The amplitude bars the sender recorded. Messages that reached this
    // device without them still read as a voice note rather than a flat line:
    // the shape is derived from the message id, so it is stable while
    // scrolling instead of changing on every redraw.
    readonly property var waveformBars: {
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

    width: ListView.view ? ListView.view.width : 640
    implicitHeight: bubble.implicitHeight + 4

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
        if (needsAutoDownload && modelData.id)
            backend.ensureMedia(modelData.id)
    }
    Component.onCompleted: fetchMediaIfNeeded()
    onModelDataChanged: fetchMediaIfNeeded()

    Rectangle {
        id: bubble
        objectName: "messageBubble"
        implicitWidth: root.contentWidth + 2 * root.horizontalPadding
        width: implicitWidth
        // The meta line is placed rather than stacked, so a short message can
        // keep its timestamp beside the text instead of below it.
        implicitHeight: messageContent.implicitHeight + 12 + (root.metaBeside ? 0 : metaRow.height + 2)
        // Positioned, never anchored. Anchoring one side or the other by
        // sender needs two bindings, and they do not change together: while a
        // reused delegate is rebound to a message from the other side, both
        // anchors are set for an instant. That stretches the bubble across the
        // conversation and discards its width binding for good.
        x: root.modelData.from_me ? Math.max(0, root.width - width - 40) : 40
        color: root.modelData.from_me ? Theme.outgoingBubble : Theme.incomingBubble
        radius: Theme.radiusLarge
        border.color: Theme.bubbleBorder
        border.width: root.modelData.from_me ? 0 : 1

        Canvas {
            id: messageTail
            objectName: "messageTail"
            width: 10
            height: 13
            y: 0
            x: root.modelData.from_me ? bubble.width + 8 - width : -8
            property color fillColor: bubble.color
            visible: root.modelData.kind !== "system"

            renderStrategy: Canvas.Immediate
            antialiasing: true
            onFillColorChanged: requestPaint()
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
                    ctx.moveTo(0, 0)
                    ctx.lineTo(width, 0)
                    ctx.lineTo(width, height)
                }
                ctx.closePath()
                ctx.fill()
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
            y: 6
            width: root.contentWidth
            spacing: 3

            Label {
                id: senderLabel
                // Only a group needs to say who is talking. Repeating the other
                // person's name above every message in a one-to-one chat is
                // noise, and WhatsApp does not do it.
                visible: !root.modelData.from_me && Boolean(root.modelData.sender_name)
                    && String(root.modelData.chat_jid || "").endsWith("@g.us")
                width: root.contentWidth
                text: root.modelData.sender_name || ""
                color: Theme.primary
                font.pixelSize: 12
                font.weight: Font.DemiBold
                elide: Text.ElideRight
                maximumLineCount: 1
            }

            Rectangle {
                id: replyBox
                visible: root.hasReply
                // What the quoted message would need if it were not elided.
                readonly property real naturalWidth: Math.max(replySender.implicitWidth, replyLabel.implicitWidth) + 19
                width: root.contentWidth
                height: visible ? replyPreviewColumn.implicitHeight + 10 : 0
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
                    Label {
                        id: replyLabel
                        width: parent.width
                        text: Theme.emojiRichText(root.modelData.reply_preview || qsTr("Replied message"))
                        color: Theme.textMuted
                        font.pixelSize: 12
                        elide: Text.ElideRight
                        maximumLineCount: 2
                        wrapMode: Text.Wrap
                        textFormat: Text.RichText
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
                visible: previewReady || videoPlaceholder
                width: root.mediaWidth
                height: visible ? (previewReady ? mediaImage.displayHeight : 150) : 0

                Rectangle {
                    anchors.fill: parent
                    visible: mediaFrame.videoPlaceholder
                    color: Theme.dark ? "#0C1418" : "#2A3942"
                }

                Image {
                    id: mediaImage
                    objectName: "messageMedia"
                    anchors.fill: parent
                    visible: mediaFrame.previewReady
                    source: root.hasPreview ? "file://" + root.previewPath : ""
                    fillMode: root.modelData.kind === "sticker" ? Image.PreserveAspectFit : Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    smooth: true
                    mipmap: root.previewIsThumbnail
                    readonly property real displayHeight: {
                        if (implicitWidth <= 0 || implicitHeight <= 0)
                            return 170
                        const natural = root.mediaWidth * (implicitHeight / implicitWidth)
                        return Math.max(110, Math.min(root.modelData.kind === "sticker" ? 160 : 320, natural))
                    }
                    Accessible.name: root.modelData.body || root.mediaLabel
                }

                Rectangle {
                    objectName: "mediaPlayBadge"
                    visible: root.modelData.kind === "video"
                    anchors.centerIn: parent
                    width: 48
                    height: 48
                    radius: 24
                    color: "#99000000"
                    Label {
                        anchors.centerIn: parent
                        text: root.playingThis && Playback.playing ? "❚❚" : "▶"
                        color: "#FFFFFF"
                        font.pixelSize: 18
                        Accessible.ignored: true
                    }
                }

                Button {
                    objectName: "mediaOverlayAction"
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

                MouseArea {
                    anchors.fill: parent
                    enabled: root.modelData.kind === "video" || root.hasMedia
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (root.modelData.kind === "video")
                            Playback.start(root.modelData.id, root.modelData.media_path || "", true)
                        else
                            backend.openFile(root.modelData.media_path)
                    }
                }
            }

            // Voice notes and audio files play in place.
            RowLayout {
                id: voiceRow
                objectName: "voiceRow"
                visible: root.audioKind
                width: root.contentWidth
                spacing: 8

                ToolButton {
                    objectName: "voicePlayButton"
                    Layout.preferredWidth: 30
                    Layout.preferredHeight: 30
                    text: root.playingThis && Playback.playing ? "❚❚" : "▶"
                    font.pixelSize: 12
                    padding: 0
                    Accessible.name: root.playingThis && Playback.playing
                        ? qsTr("Pause voice message")
                        : qsTr("Play voice message")
                    contentItem: Label {
                        text: parent.text
                        color: Theme.icon
                        font: parent.font
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    background: Rectangle {
                        radius: width / 2
                        color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : Theme.surfaceMuted
                    }
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
                    source: visible ? "file://" + root.modelData.media_thumbnail : ""
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

            RowLayout {
                id: fileRow
                // Files that have no picture of their own keep the descriptive
                // row; a photo or video only falls back to it without a preview.
                visible: (root.mediaKind && !root.audioKind && !mediaFrame.visible)
                    || root.modelData.kind === "poll" 
                width: root.contentWidth
                spacing: 8
                Rectangle {
                    Layout.preferredWidth: 30
                    Layout.preferredHeight: 30
                    radius: 15
                    color: Theme.surfaceMuted
                    Label {
                        anchors.centerIn: parent
                        text: root.modelData.kind === "video" ? "▶" : root.modelData.kind === "document" ? "↓" : "•"
                        color: Theme.icon
                        font.pixelSize: 13
                        Accessible.ignored: true
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
                    objectName: "mediaAction"
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
                visible: Boolean(root.modelData.link_url)
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
                    source: visible ? "file://" + root.modelData.link_thumbnail : ""
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: visible ? Math.min(150, Math.max(90, width * 0.52)) : 0
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
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

            SelectableMessageText {
                id: bodyText
                visible: Boolean(root.modelData.body) && !root.contactKind && !root.locationKind
                width: root.metaBeside ? implicitWidth : root.contentWidth
                maximumWidth: root.contentMaxWidth
                plainText: root.modelData.revoked ? qsTr("This message was deleted") : root.modelData.body || ""
                color: root.modelData.revoked ? Theme.textMuted : Theme.text
                font.pixelSize: root.bodyFontSize
                font.italic: Boolean(root.modelData.revoked)
            }

            Label {
                id: unsupportedLabel
                visible: !Boolean(root.modelData.body) && !root.mediaKind && root.modelData.kind !== "system"
                text: qsTr("Unsupported message")
                color: Theme.textMuted
                font.pixelSize: root.captionFontSize
                font.italic: true
            }

            Flow {
                id: reactionsFlow
                visible: Boolean(root.modelData.reactions) && root.modelData.reactions.length > 0
                width: root.contentWidth
                spacing: 4
                Repeater {
                    model: root.modelData.reactions || []
                    Label {
                        required property var modelData
                        text: modelData.emoji
                        font.family: Theme.emojiFontFamily
                        font.pixelSize: 15
                        padding: 3
                        background: Rectangle { color: Theme.surfaceMuted; radius: 8; border.color: Theme.border }
                        Accessible.name: qsTr("Reaction %1").arg(modelData.emoji)
                    }
                }
            }
        }
    }

    TapHandler {
        acceptedButtons: Qt.RightButton
        onTapped: (eventPoint, button) => {
            // Opening the menu moves focus away and clears the selection, so
            // what was selected is remembered before that happens.
            contextMenu.capturedSelection = bodyText.selectedText
            const mapped = root.mapToItem(contextMenu.parent, eventPoint.position.x, eventPoint.position.y)
            contextMenu.x = Math.max(8, Math.min(contextMenu.parent.width - contextMenu.width - 8, mapped.x))
            contextMenu.y = Math.max(8, Math.min(contextMenu.parent.height - contextMenu.implicitHeight - 8, mapped.y))
            contextMenu.open()
        }
    }

    WhatsAppMenuPopup {
        id: contextMenu
        parent: Overlay.overlay
        width: 246
        property string capturedSelection: ""

        WhatsAppMenuItem {
            visible: Boolean(contextMenu.capturedSelection)
            height: visible ? 46 : 0
            text: qsTr("Copy selected text")
            iconSource: Qt.resolvedUrl("icons/copy.svg")
            onClicked: {
                backend.copyText(contextMenu.capturedSelection)
                contextMenu.close()
            }
        }

        WhatsAppMenuItem {
            visible: root.modelData.kind === "image" || root.modelData.kind === "sticker"
            height: visible ? 46 : 0
            text: qsTr("Copy image")
            iconSource: Qt.resolvedUrl("icons/copy.svg")
            onClicked: {
                contextMenu.close()
                backend.copyImage(root.modelData.id, root.modelData.media_path || "")
            }
        }

        WhatsAppMenuItem {
            text: qsTr("Reply")
            iconSource: Qt.resolvedUrl("icons/reply.svg")
            onClicked: {
                contextMenu.close()
                root.replyRequested(root.modelData.id, root.modelData.body || root.mediaLabel)
            }
        }
        WhatsAppMenuItem {
            text: qsTr("React with thumbs up")
            iconSource: Qt.resolvedUrl("icons/smile.svg")
            onClicked: {
                contextMenu.close()
                backend.reactMessage(root.modelData.id, root.modelData.sender_jid || "", "👍")
            }
        }
        WhatsAppMenuItem {
            text: qsTr("React with heart")
            iconSource: Qt.resolvedUrl("icons/heart.svg")
            onClicked: {
                contextMenu.close()
                backend.reactMessage(root.modelData.id, root.modelData.sender_jid || "", "❤️")
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
            height: visible ? 46 : 0
            text: qsTr("Edit")
            iconSource: Qt.resolvedUrl("icons/edit.svg")
            onClicked: {
                contextMenu.close()
                root.editRequested(root.modelData.id, root.modelData.body || "")
            }
        }
        WhatsAppMenuItem {
            visible: Boolean(root.modelData.from_me) && !root.modelData.revoked
            height: visible ? 46 : 0
            text: qsTr("Delete for everyone")
            iconSource: Qt.resolvedUrl("icons/delete.svg")
            destructive: true
            onClicked: {
                contextMenu.close()
                root.deleteRequested(root.modelData.id, root.modelData.sender_jid || "")
            }
        }
    }
}
