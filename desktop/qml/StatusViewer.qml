import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtMultimedia
import org.whatsappgo

Item {
    id: root
    property var groups: []
    property bool opened: false
    property int groupIndex: 0
    property int itemIndex: 0
    property real progress: 0
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
    readonly property bool videoReady: currentItem.kind === "video" && Boolean(currentItem.media_path)
    readonly property bool interactionPaused: replyPending || replyComposer.activeFocus || statusEmojiPicker.opened
    signal closeRequested()
    signal mediaRequested(string messageId)
    signal replyRequested(string recipientJid, string statusMessageId, string text)

    visible: opened
    focus: opened

    function mediaUrl(value) {
        const source = String(value || "")
        if (!source)
            return ""
        if (source.indexOf("://") >= 0 || source.indexOf("qrc:") === 0)
            return source
        return "file://" + source
    }

    function openAt(index) {
        groupIndex = Math.max(0, Math.min(groups.length - 1, index))
        itemIndex = 0
        opened = groups.length > 0
        restartPlayback()
        forceActiveFocus()
    }

    function close() {
        storyTimer.stop()
        progressAnimation.stop()
        statusPlayer.stop()
        statusEmojiPicker.close()
        opened = false
        closeRequested()
    }

    function pausePlayback() {
        storyTimer.stop()
        progressAnimation.stop()
        if (statusPlayer.playbackState === MediaPlayer.PlayingState)
            statusPlayer.pause()
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
        storyTimer.stop()
        progressAnimation.stop()
        statusPlayer.stop()
        progress = 0
        if (!opened)
            return
        if (currentItem.id && (currentItem.kind === "image" || currentItem.kind === "video") && !currentItem.media_path)
            mediaRequested(currentItem.id)
        Qt.callLater(function() {
            if (!root.opened || root.interactionPaused)
                return
            if (root.videoReady) {
                statusPlayer.source = root.mediaUrl(root.currentItem.media_path)
                statusPlayer.play()
            } else {
                progressAnimation.start()
                storyTimer.restart()
            }
        })
    }

    onOpenedChanged: if (opened) restartPlayback(); else statusPlayer.stop()
    onCurrentItemChanged: {
        replyText = ""
        replyPending = false
        replyFeedback = ""
        lastReplyRecipient = ""
        lastReplyStatusId = ""
        lastReplyText = ""
        statusEmojiPicker.close()
        if (opened)
            restartPlayback()
    }
    onInteractionPausedChanged: interactionPaused ? pausePlayback() : restartPlayback()

    Keys.onEscapePressed: close()
    Keys.onLeftPressed: previous()
    Keys.onRightPressed: advance()

    Rectangle {
        anchors.fill: parent
        color: "#E60B1014"

        Image {
            anchors.fill: parent
            source: root.mediaUrl(root.currentItem.media_thumbnail || root.currentItem.media_path)
            fillMode: Image.PreserveAspectCrop
            opacity: status === Image.Ready ? 0.22 : 0
        }
        Rectangle { anchors.fill: parent; color: "#73000000" }
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
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/smile.svg")
                iconTint: "#FFFFFF"
                Accessible.name: qsTr("Choose an emoji for the status reply")
                onClicked: statusEmojiPicker.opened ? statusEmojiPicker.close() : statusEmojiPicker.open()
                background: Rectangle {
                    radius: 22
                    color: parent.hovered || statusEmojiPicker.opened ? "#33FFFFFF" : "transparent"
                }
            }

            TextField {
                id: replyComposer
                objectName: "statusReplyComposer"
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
                anchors.centerIn: parent
                width: parent.width - 80
                text: Theme.emojiRichText(root.currentItem.body || qsTr("Status unavailable"))
                textFormat: Text.RichText
                color: "#FFFFFF"
                font.pixelSize: 25
                font.weight: Font.Medium
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
            }
        }

        VideoOutput {
            id: statusVideo
            anchors.fill: parent
            fillMode: VideoOutput.PreserveAspectFit
            visible: root.videoReady
        }

        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton
            onClicked: mouse => mouse.x < width / 3 ? root.previous() : root.advance()
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

    ColumnLayout {
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
                source: root.currentGroup.avatar_path ? "file://" + root.currentGroup.avatar_path : ""
            }
            ColumnLayout {
                Layout.fillWidth: true
                spacing: 0
                Label {
                    text: root.currentGroup.sender_name || qsTr("Unknown contact")
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
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconTint: "#FFFFFF"
                Accessible.name: qsTr("Close status viewer")
                background: Rectangle { radius: 22; color: parent.hovered ? "#33FFFFFF" : "transparent" }
                onClicked: root.close()
            }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: 24
        width: 52
        height: 52
        radius: 26
        color: previousArea.containsMouse ? "#99000000" : "#66000000"
        Label { anchors.centerIn: parent; text: "‹"; color: "white"; font.pixelSize: 38 }
        MouseArea { id: previousArea; anchors.fill: parent; hoverEnabled: true; onClicked: root.previous() }
    }
    Rectangle {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.rightMargin: 24
        width: 52
        height: 52
        radius: 26
        color: nextArea.containsMouse ? "#99000000" : "#66000000"
        Label { anchors.centerIn: parent; text: "›"; color: "white"; font.pixelSize: 38 }
        MouseArea { id: nextArea; anchors.fill: parent; hoverEnabled: true; onClicked: root.advance() }
    }

    Timer { id: storyTimer; interval: 5500; onTriggered: root.advance() }
    Timer { id: replyFeedbackTimer; interval: 2800; onTriggered: root.replyFeedback = "" }
    NumberAnimation {
        id: progressAnimation
        target: root
        property: "progress"
        from: 0
        to: 1
        duration: storyTimer.interval
    }
}
