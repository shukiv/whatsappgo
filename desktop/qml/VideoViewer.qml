import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window
import org.whatsappgo

Rectangle {
    id: root
    objectName: "videoViewer"
    color: "#F2000000"
    focus: true
    property var previousFocus: null
    property int previousVisibility: Window.Windowed
    property bool ownsFullscreen: false
    readonly property var host: root.Window.window
    readonly property var message: backend.messageById(Playback.currentId)
    readonly property string title: String(message.sender_name || backend.selectedChat.title || qsTr("Video"))
    readonly property string caption: String(message.body || message.media_name || "")
    function clock(value) {
        const seconds = Math.max(0, Math.floor(value / 1000))
        return Math.floor(seconds / 60) + ":" + String(seconds % 60).padStart(2, "0")
    }
    function playPause() {
        if (!Playback.playing && Playback.duration > 0 && Playback.position >= Playback.duration - 20)
            Playback.seek(0)
        Playback.toggle()
    }
    function restoreWindow() {
        if (ownsFullscreen && host && host.visibility === Window.FullScreen) host.visibility = previousVisibility
        ownsFullscreen = false
    }
    function toggleFullscreen() {
        if (!host) return
        if (host.visibility === Window.FullScreen) {
            host.visibility = ownsFullscreen ? previousVisibility : Window.Windowed
            ownsFullscreen = false
        } else {
            previousVisibility = host.visibility
            ownsFullscreen = true
            host.visibility = Window.FullScreen
        }
    }
    function closeViewer() {
        const previous = previousFocus
        restoreWindow()
        Playback.stop()
        Qt.callLater(() => { if (previous && previous.visible) previous.forceActiveFocus() })
    }
    function goBack() {
        if (ownsFullscreen) restoreWindow()
        else closeViewer()
    }
    Component.onCompleted: {
        previousFocus = host ? host.activeFocusItem : null
        Playback.videoSurface = surface
        forceActiveFocus()
    }
    Component.onDestruction: {
        restoreWindow()
        Playback.videoSurface = null
    }
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Escape) goBack()
        else if (event.key === Qt.Key_Space) playPause()
        else if (event.key === Qt.Key_Left) Playback.seek(Playback.position - 5000)
        else if (event.key === Qt.Key_Right) Playback.seek(Playback.position + 5000)
        else if (event.key === Qt.Key_M) Playback.videoMuted = !Playback.videoMuted
        else if (event.key === Qt.Key_F) toggleFullscreen()
        else return
        event.accepted = true
    }
    MouseArea { anchors.fill: parent; onClicked: { root.forceActiveFocus(); root.playPause() } }
    VideoSurface { id: surface; anchors.fill: parent; anchors.topMargin: 62; anchors.bottomMargin: controls.height + 24; anchors.leftMargin: 24; anchors.rightMargin: 24 }
    RowLayout {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.margins: 12
        Label { Layout.fillWidth: true; text: root.title; color: "white"; elide: Text.ElideRight; textFormat: Text.PlainText }
        ThemedToolButton {
            objectName: "videoClose"
            Layout.preferredWidth: 44; Layout.preferredHeight: 44
            iconSource: Qt.resolvedUrl("icons/close.svg"); iconTint: "white"
            Accessible.name: qsTr("Close video")
            onClicked: root.closeViewer()
            background: Rectangle { radius: 22; color: parent.hovered ? "#33FFFFFF" : "transparent"; border.width: parent.activeFocus ? 2 : 0; border.color: Theme.primary }
        }
    }
    ColumnLayout {
        id: controls
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.margins: 18
        spacing: 4
        Label {
            objectName: "videoCaption"
            Layout.fillWidth: true
            visible: root.caption !== ""
            text: root.caption; textFormat: Text.PlainText; color: "white"
            wrapMode: Text.Wrap; maximumLineCount: 2; elide: Text.ElideRight
        }
        RowLayout {
            Layout.fillWidth: true
            ThemedToolButton {
                objectName: "videoPlayPause"
                Layout.preferredWidth: 44; Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl(Playback.playing ? "icons/pause.svg" : "icons/play.svg"); iconTint: "white"
                Accessible.name: Playback.playing ? qsTr("Pause") : qsTr("Play")
                onClicked: root.playPause()
                background: Rectangle { radius: 22; color: parent.hovered ? "#33FFFFFF" : "transparent"; border.width: parent.activeFocus ? 2 : 0; border.color: Theme.primary }
            }
            Slider {
                id: seek
                objectName: "videoSeek"
                Layout.fillWidth: true
                from: 0; to: Math.max(1, Playback.duration); value: Playback.position
                enabled: Playback.seekable && Playback.duration > 0
                stepSize: 0
                property bool resumeAfterDrag: false
                Accessible.name: qsTr("Video position")
                Accessible.description: root.clock(value) + " / " + root.clock(Playback.duration)
                onPressedChanged: {
                    if (pressed) { resumeAfterDrag = Playback.playing; if (resumeAfterDrag) Playback.toggle() }
                    else { Playback.seek(value); if (resumeAfterDrag && !Playback.playing) Playback.toggle(); resumeAfterDrag = false }
                }
                onMoved: Playback.seek(value)
                Keys.onLeftPressed: Playback.seek(Playback.position - 5000)
                Keys.onRightPressed: Playback.seek(Playback.position + 5000)
            }
            Label { text: root.clock(Playback.position) + " / " + root.clock(Playback.duration); color: "white"; font.pixelSize: 12 }
        }
        RowLayout {
            Layout.fillWidth: true
            ThemedToolButton {
                objectName: "videoMute"
                Layout.preferredWidth: 40; Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl(Playback.videoMuted ? "icons/volume-off.svg" : "icons/volume.svg"); iconTint: "white"
                Accessible.name: Playback.videoMuted ? qsTr("Unmute") : qsTr("Mute")
                checkable: true; checked: Playback.videoMuted
                onClicked: Playback.videoMuted = !Playback.videoMuted
                background: Rectangle { radius: 20; color: parent.hovered ? "#33FFFFFF" : "transparent"; border.width: parent.activeFocus ? 2 : 0; border.color: Theme.primary }
            }
            Slider {
                objectName: "videoVolume"
                Layout.preferredWidth: Math.min(120, root.width / 5)
                from: 0; to: 1; stepSize: 0.05; value: Playback.videoVolume
                Accessible.name: qsTr("Video volume")
                onMoved: { Playback.videoVolume = value; Playback.videoMuted = value === 0 }
                Keys.onPressed: event => {
                    if (event.key === Qt.Key_Home) { Playback.videoVolume = 0; Playback.videoMuted = true }
                    else if (event.key === Qt.Key_End) { Playback.videoVolume = 1; Playback.videoMuted = false }
                    else return
                    event.accepted = true
                }
            }
            Item { Layout.fillWidth: true }
            ComboBox {
                objectName: "videoSpeed"
                model: ["0.5×", "0.75×", "1×", "1.25×", "1.5×", "2×"]
                currentIndex: [0.5, 0.75, 1, 1.25, 1.5, 2].indexOf(Playback.videoRate)
                Accessible.name: qsTr("Playback speed")
                onActivated: index => Playback.videoRate = [0.5, 0.75, 1, 1.25, 1.5, 2][index]
            }
            ThemedToolButton {
                objectName: "videoFullscreen"
                Layout.preferredWidth: 40; Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/fullscreen.svg"); iconTint: "white"
                Accessible.name: root.host && root.host.visibility === Window.FullScreen ? qsTr("Exit fullscreen") : qsTr("Fullscreen")
                onClicked: root.toggleFullscreen()
                background: Rectangle { radius: 20; color: parent.hovered ? "#33FFFFFF" : "transparent"; border.width: parent.activeFocus ? 2 : 0; border.color: Theme.primary }
            }
        }
    }
}
