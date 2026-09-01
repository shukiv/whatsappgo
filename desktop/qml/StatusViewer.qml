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
    readonly property var currentGroup: groups && groupIndex >= 0 && groupIndex < groups.length ? groups[groupIndex] : ({})
    readonly property var currentItems: currentGroup.items || []
    readonly property var currentItem: itemIndex >= 0 && itemIndex < currentItems.length ? currentItems[itemIndex] : ({})
    readonly property bool videoReady: currentItem.kind === "video" && Boolean(currentItem.media_path)
    signal closeRequested()
    signal mediaRequested(string messageId)

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
        statusPlayer.stop()
        opened = false
        closeRequested()
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
            if (!root.opened)
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
        if (opened)
            restartPlayback()
    }

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
    NumberAnimation {
        id: progressAnimation
        target: root
        property: "progress"
        from: 0
        to: 1
        duration: storyTimer.interval
    }
}
