import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtMultimedia
import org.whatsappgo

Item {
    id: root
    objectName: "statusViewer"
    property var groups: []
    property string profile: ""
    property bool opened: false
    property int groupIndex: 0
    property int itemIndex: 0
    property real progress: 0
    property bool manuallyPaused: false
    property string replyText: ""
    property bool replyPending: false
    property string replyFeedback: ""
    property bool replyFailed: false
    property string lastReplyRecipient: ""
    property string lastReplyStatusId: ""
    property string lastReplyText: ""
    readonly property var currentGroup: groups && groupIndex >= 0 && groupIndex < groups.length ? groups[groupIndex] : ({})
    readonly property var currentItems: currentGroup.items || []
    readonly property var currentItem: itemIndex >= 0 && itemIndex < currentItems.length ? currentItems[itemIndex] : ({})
    readonly property string currentIdentity: currentItem.id
        ? profile + "/" + String(currentGroup.sender_jid || "") + "/" + String(currentItem.id) : ""
    readonly property bool videoReady: currentItem.kind === "video" && Boolean(currentItem.media_path)
    readonly property bool interactionPaused: manuallyPaused || replyPending || replyComposer.activeFocus || statusEmojiPicker.opened
    signal closeRequested()
    signal mediaRequested(string messageId)
    signal replyRequested(string recipientJid, string statusMessageId, string text)

    visible: opened
    focus: opened

    function mediaUrl(value) {
        const source = String(value || "")
        if (!source)
            return ""
        return Theme.fileUrl(source)
    }

    function openAt(index) {
        manuallyPaused = false
        groupIndex = Math.max(0, Math.min(groups.length - 1, index))
        itemIndex = 0
        opened = groups.length > 0
        restartPlayback()
        forceActiveFocus()
    }

    function close() {
        if (!opened)
            return
        opened = false
        progressAnimation.stop()
        statusPlayer.stop()
        statusEmojiPicker.close()
        closeRequested()
    }

    function pausePlayback() {
        if (progressAnimation.running && !progressAnimation.paused)
            progressAnimation.pause()
        if (statusPlayer.playbackState === MediaPlayer.PlayingState)
            statusPlayer.pause()
    }

    function resumePlayback() {
        if (!opened || interactionPaused)
            return
        if (videoReady) {
            const source = mediaUrl(currentItem.media_path)
            if (String(statusPlayer.source) !== source)
                statusPlayer.source = source
            statusPlayer.play()
        } else if (progressAnimation.paused) {
            progressAnimation.resume()
        } else if (!progressAnimation.running) {
            progressAnimation.start()
        }
    }

    function submitReply() {
        const text = replyText.trim()
        if (!text || replyPending || !currentGroup.sender_jid || !currentItem.id)
            return
        lastReplyRecipient = String(currentGroup.sender_jid)
        lastReplyStatusId = String(currentItem.id)
        lastReplyText = text
        replyPending = true
        replyFeedback = ""
        replyFailed = false
        replyRequested(lastReplyRecipient, lastReplyStatusId, text)
    }

    function finishReply(recipientJid, statusMessageId, success, message) {
        if (recipientJid !== lastReplyRecipient || statusMessageId !== lastReplyStatusId)
            return
        replyPending = false
        replyFailed = !success
        replyFeedback = message || (success ? qsTr("Reply sent") : qsTr("Could not send the reply"))
        if (success)
            replyText = ""
        replyFeedbackTimer.restart()
    }

    function insertReplyEmoji(emoji) {
        const position = Math.max(0, replyComposer.cursorPosition)
        replyComposer.insert(position, emoji)
        replyComposer.cursorPosition = position + emoji.length
        replyComposer.forceActiveFocus()
    }

    function advance() {
        if (!opened || groups.length === 0)
            return
        if (itemIndex + 1 < currentItems.length) {
            itemIndex += 1
        } else if (groupIndex + 1 < groups.length) {
            groupIndex += 1
            itemIndex = 0
        } else {
            close()
            return
        }
        restartPlayback()
    }

    function previous() {
        if (!opened || groups.length === 0)
            return
        if (itemIndex > 0) {
            itemIndex -= 1
        } else if (groupIndex > 0) {
            groupIndex -= 1
            itemIndex = Math.max(0, (groups[groupIndex].items || []).length - 1)
        } else {
            restartPlayback()
            return
        }
        restartPlayback()
    }

    function restartPlayback() {
        progressAnimation.stop()
        statusPlayer.stop()
        progress = 0
        if (!opened)
            return
        if (currentItem.id && (currentItem.kind === "image" || currentItem.kind === "video") && !currentItem.media_path)
            mediaRequested(currentItem.id)
        Qt.callLater(function() {
            root.resumePlayback()
        })
    }

    onOpenedChanged: {
        if (opened) {
            restartPlayback()
        } else {
            progressAnimation.stop()
            statusPlayer.stop()
        }
    }
    onCurrentIdentityChanged: {
        replyText = ""
        replyPending = false
        replyFeedback = ""
        replyFailed = false
        lastReplyRecipient = ""
        lastReplyStatusId = ""
        lastReplyText = ""
        statusEmojiPicker.close()
    }
    onCurrentItemChanged: {
        if (opened)
            restartPlayback()
    }
    onInteractionPausedChanged: interactionPaused ? pausePlayback() : resumePlayback()

    // Keep keyboard traversal inside this overlay, including when it was
    // opened from a chat avatar rather than from the Status page.
    function focusControl(backwards) {
        const controls = [pauseButton, closeButton, previousButton, nextButton,
                          replyEmojiButton, replyComposer, replySendButton]
        const focused = root.Window.window ? root.Window.window.activeFocusItem : null
        let index = controls.indexOf(focused)
        if (index < 0)
            index = backwards ? 0 : -1
        for (let step = 0; step < controls.length; ++step) {
            index = (index + (backwards ? -1 : 1) + controls.length) % controls.length
            if (controls[index].visible && controls[index].enabled) {
                controls[index].forceActiveFocus(backwards ? Qt.BacktabFocusReason : Qt.TabFocusReason)
                return
            }
        }
    }

    Shortcut {
        sequence: "Escape"
        autoRepeat: false
        enabled: root.opened && !Theme.popupOwnsFocus(root.Overlay.overlay,
            root.Window.window ? root.Window.window.activeFocusItem : null)
        onActivated: root.close()
    }
    Keys.onLeftPressed: previous()
    Keys.onRightPressed: advance()
    Keys.onTabPressed: event => { focusControl(false); event.accepted = true }
    Keys.onBacktabPressed: event => { focusControl(true); event.accepted = true }
    Keys.onSpacePressed: event => {
        manuallyPaused = !manuallyPaused
        event.accepted = true
    }

    Rectangle {
        anchors.fill: parent
        color: "#E60B1014"

        Image {
            anchors.fill: parent
            source: root.mediaUrl(root.currentItem.media_thumbnail
                || (root.currentItem.kind === "image" ? root.currentItem.media_path : ""))
            asynchronous: true
            fillMode: Image.PreserveAspectCrop
            opacity: status === Image.Ready ? 0.22 : 0
        }
        Rectangle { anchors.fill: parent; color: "#73000000" }
    }

    // Consume clicks and scrolling outside the controls so the covered chat
    // cannot receive input or steal focus. Later siblings remain interactive.
    MouseArea {
        objectName: "statusInputShield"
        anchors.fill: parent
        acceptedButtons: Qt.AllButtons
        onPressed: root.forceActiveFocus()
        onWheel: wheel => { wheel.accepted = true }
    }

    Label {
        anchors.horizontalCenter: replyBar.horizontalCenter
        anchors.bottom: replyBar.top
        anchors.bottomMargin: 8
        z: 21
        visible: Boolean(root.replyFeedback)
        text: root.replyFeedback
        color: "#FFFFFF"
        font.pixelSize: 13
        leftPadding: 12
        rightPadding: 12
        topPadding: 7
        bottomPadding: 7
        background: Rectangle {
            radius: 8
            color: root.replyFailed ? "#CCB3261E" : "#CC202C33"
        }
    }

    Rectangle {
        id: replyBar
        objectName: "statusReplyBar"
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 24
        z: 20
        width: Math.max(280, Math.min(parent.width - 120, 1040))
        height: 56
        radius: 12
        color: "#B3202529"
        border.width: replyComposer.activeFocus ? 2 : 1
        border.color: replyComposer.activeFocus ? "#FFFFFFFF" : "#66FFFFFF"

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 6
            anchors.rightMargin: 6
            spacing: 4

            ThemedToolButton {
                id: replyEmojiButton
                objectName: "statusReplyEmojiButton"
                focusPolicy: Qt.StrongFocus
                KeyNavigation.priority: KeyNavigation.BeforeItem
                KeyNavigation.tab: replyComposer
                KeyNavigation.backtab: nextButton
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/smile.svg")
                iconTint: "#FFFFFF"
                Accessible.name: qsTr("Choose an emoji for the status reply")
                onClicked: statusEmojiPicker.opened ? statusEmojiPicker.close() : statusEmojiPicker.open()
                background: Rectangle {
                    radius: 22
                    color: parent.hovered || statusEmojiPicker.opened ? "#33FFFFFF" : "transparent"
                    border.width: parent.visualFocus ? 2 : 0
                    border.color: "#FFFFFF"
                }
            }

            TextField {
                id: replyComposer
                objectName: "statusReplyComposer"
                KeyNavigation.priority: KeyNavigation.BeforeItem
                KeyNavigation.tab: replySendButton
                KeyNavigation.backtab: replyEmojiButton
                Layout.fillWidth: true
                Layout.fillHeight: true
                text: root.replyText
                placeholderText: qsTr("Type a reply")
                placeholderTextColor: "#CCFFFFFF"
                color: "#FFFFFF"
                font.pixelSize: 15
                leftPadding: 10
                rightPadding: 10
                enabled: !root.replyPending
                Accessible.name: qsTr("Reply to %1's status").arg(root.currentGroup.sender_name || qsTr("contact"))
                onTextChanged: if (root.replyText !== text) root.replyText = text
                onAccepted: root.submitReply()
                background: Item {}
            }

            BusyIndicator {
                Layout.preferredWidth: 42
                Layout.preferredHeight: 42
                running: root.replyPending
                visible: running
                palette.dark: "#FFFFFF"
                palette.light: "#FFFFFF"
            }

            ThemedToolButton {
                id: replySendButton
                objectName: "statusReplySendButton"
                focusPolicy: Qt.StrongFocus
                KeyNavigation.priority: KeyNavigation.BeforeItem
                KeyNavigation.tab: pauseButton
                KeyNavigation.backtab: replyComposer
                visible: !root.replyPending
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/send.svg")
                iconTint: enabled ? "#FFFFFF" : "#66FFFFFF"
                enabled: Boolean(root.replyText.trim())
                Accessible.name: qsTr("Send status reply")
                onClicked: root.submitReply()
                background: Rectangle {
                    radius: 22
                    color: parent.hovered && parent.enabled ? "#33FFFFFF" : "transparent"
                    border.width: parent.visualFocus ? 2 : 0
                    border.color: "#FFFFFF"
                }
            }
        }
    }

    EmojiPicker {
        id: statusEmojiPicker
        parent: replyEmojiButton
        x: 0
        y: -height - 12
        onEmojiChosen: emoji => root.insertReplyEmoji(emoji)
    }

    Rectangle {
        id: storyFrame
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.horizontalCenter: parent.horizontalCenter
        width: Math.min(parent.width * 0.62, 620)
        color: "#111111"

        Image {
            id: storyImage
            anchors.fill: parent
            source: root.currentItem.kind !== "video"
                ? root.mediaUrl(root.currentItem.media_path || root.currentItem.media_thumbnail)
                : ""
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            visible: source && status === Image.Ready
        }

        Rectangle {
            anchors.fill: parent
            visible: root.currentItem.kind === "text" || (!storyImage.visible && !root.videoReady)
            color: root.currentItem.kind === "text" ? "#236B63" : "#202124"
            Label {
                id: statusBody
                objectName: "statusBodyText"
                anchors.centerIn: parent
                width: parent.width - 80
                // A link in a text status opens, the way it does in a message.
                // White and underlined rather than the chat's link green, which
                // all but disappears against this teal.
                text: Theme.messageRichText(root.currentItem.body || qsTr("Status unavailable"),
                                            "#FFFFFF", true)
                textFormat: Text.RichText
                color: "#FFFFFF"
                font.pixelSize: 25
                font.weight: Font.Medium
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
            }
        }

        // VideoSurface rather than VideoOutput: the software scene graph this
        // application runs on cannot draw a VideoOutput, so the story played
        // its audio against a black rectangle. See src/videosurface.h.
        VideoSurface {
            id: statusVideo
            anchors.fill: parent
            visible: root.videoReady
        }

        MouseArea {
            id: storyTapArea
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton
            hoverEnabled: true
            // The tap area covers the whole story so a click anywhere steps
            // through it. That also puts it over the text, so the link under
            // the pointer is resolved here rather than by the label itself.
            function linkUnder(x, y) {
                if (!statusBody.visible)
                    return ""
                const point = statusBody.mapFromItem(storyTapArea, x, y)
                return statusBody.linkAt(point.x, point.y)
            }
            cursorShape: linkUnder(mouseX, mouseY) ? Qt.PointingHandCursor : Qt.ArrowCursor
            onClicked: mouse => {
                const link = linkUnder(mouse.x, mouse.y)
                if (link) {
                    Qt.openUrlExternally(link)
                    return
                }
                mouse.x < width / 3 ? root.previous() : root.advance()
            }
        }
    }

    MediaPlayer {
        id: statusPlayer
        videoOutput: statusVideo
        audioOutput: AudioOutput {}
        onPositionChanged: if (duration > 0) root.progress = Math.min(1, position / duration)
        onMediaStatusChanged: if (mediaStatus === MediaPlayer.EndOfMedia) root.advance()
        onErrorOccurred: root.advance()
    }

    // Keep the name and controls legible over a bright photo/video.
    Rectangle {
        anchors.left: storyFrame.left
        anchors.right: storyFrame.right
        anchors.top: parent.top
        height: statusHeader.height + 40
        gradient: Gradient {
            GradientStop { position: 0; color: "#D9000000" }
            GradientStop { position: 1; color: "#00000000" }
        }
    }

    ColumnLayout {
        id: statusHeader
        anchors.left: storyFrame.left
        anchors.right: storyFrame.right
        anchors.top: parent.top
        anchors.margins: 12
        spacing: 10

        RowLayout {
            Layout.fillWidth: true
            spacing: 4
            Repeater {
                model: root.currentItems.length
                Rectangle {
                    required property int index
                    Layout.fillWidth: true
                    Layout.preferredHeight: 3
                    radius: 2
                    color: "#66FFFFFF"
                    Rectangle {
                        width: parent.width * (index < root.itemIndex ? 1 : index === root.itemIndex ? root.progress : 0)
                        height: parent.height
                        radius: parent.radius
                        color: "#FFFFFF"
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 10
            Avatar {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                diameter: 44
                title: root.currentGroup.sender_name || "?"
                source: Theme.fileUrl(root.currentGroup.avatar_path)
            }
            ColumnLayout {
                Layout.fillWidth: true
                spacing: 0
                Label {
                    Layout.fillWidth: true
                    text: root.currentGroup.sender_name || qsTr("Unknown contact")
                    elide: Text.ElideRight
                    color: "#FFFFFF"
                    font.pixelSize: 16
                    font.weight: Font.Medium
                }
                Label {
                    text: Qt.formatDateTime(new Date(root.currentItem.timestamp || 0), "dd/MM/yyyy HH:mm")
                    color: "#D6FFFFFF"
                    font.pixelSize: 12
                }
            }
            ThemedToolButton {
                id: pauseButton
                objectName: "statusPauseButton"
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                focusPolicy: Qt.StrongFocus
                KeyNavigation.priority: KeyNavigation.BeforeItem
                KeyNavigation.tab: closeButton
                KeyNavigation.backtab: replySendButton
                iconSource: Qt.resolvedUrl(root.manuallyPaused ? "icons/play.svg" : "icons/pause.svg")
                iconTint: "#FFFFFF"
                Accessible.name: root.manuallyPaused ? qsTr("Resume status") : qsTr("Pause status")
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
                background: Rectangle {
                    radius: 22
                    color: parent.hovered ? "#66000000" : "#33000000"
                    border.width: parent.visualFocus ? 2 : 0
                    border.color: "#FFFFFF"
                }
                onClicked: root.manuallyPaused = !root.manuallyPaused
            }
            ThemedToolButton {
                id: closeButton
                objectName: "statusCloseButton"
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                focusPolicy: Qt.StrongFocus
                KeyNavigation.priority: KeyNavigation.BeforeItem
                KeyNavigation.tab: previousButton
                KeyNavigation.backtab: pauseButton
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconTint: "#FFFFFF"
                Accessible.name: qsTr("Close status viewer")
                ToolTip.visible: hovered
                ToolTip.text: qsTr("Close status viewer (Escape)")
                background: Rectangle {
                    radius: 22
                    color: parent.hovered ? "#66000000" : "#33000000"
                    border.width: parent.visualFocus ? 2 : 0
                    border.color: "#FFFFFF"
                }
                onClicked: root.close()
            }
        }
    }

    ThemedToolButton {
        id: previousButton
        objectName: "statusPreviousButton"
        KeyNavigation.priority: KeyNavigation.BeforeItem
        KeyNavigation.tab: nextButton
        KeyNavigation.backtab: closeButton
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: 24
        width: 52
        height: 52
        focusPolicy: Qt.StrongFocus
        iconSource: Qt.resolvedUrl("icons/chevron-right.svg")
        // The same Lucide chevron, facing toward the preceding status.
        contentItem.rotation: 180
        iconTint: "#FFFFFF"
        Accessible.name: qsTr("Previous status")
        ToolTip.visible: hovered
        ToolTip.text: Accessible.name
        onClicked: root.previous()
        background: Rectangle {
            radius: 26
            color: parent.hovered ? "#99000000" : "#66000000"
            border.width: parent.visualFocus ? 2 : 0
            border.color: "#FFFFFF"
        }
    }
    ThemedToolButton {
        id: nextButton
        objectName: "statusNextButton"
        KeyNavigation.priority: KeyNavigation.BeforeItem
        KeyNavigation.tab: replyEmojiButton
        KeyNavigation.backtab: previousButton
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.rightMargin: 24
        width: 52
        height: 52
        focusPolicy: Qt.StrongFocus
        iconSource: Qt.resolvedUrl("icons/chevron-right.svg")
        iconTint: "#FFFFFF"
        Accessible.name: qsTr("Next status")
        ToolTip.visible: hovered
        ToolTip.text: Accessible.name
        onClicked: root.advance()
        background: Rectangle {
            radius: 26
            color: parent.hovered ? "#99000000" : "#66000000"
            border.width: parent.visualFocus ? 2 : 0
            border.color: "#FFFFFF"
        }
    }

    Timer { id: replyFeedbackTimer; interval: 2800; onTriggered: root.replyFeedback = "" }
    NumberAnimation {
        id: progressAnimation
        target: root
        property: "progress"
        from: 0
        to: 1
        duration: 5500
        onFinished: if (root.opened && !root.interactionPaused) root.advance()
    }
}
