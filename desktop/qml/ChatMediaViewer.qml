import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

FocusScope {
    id: root
    objectName: "chatMediaViewer"

    property url imageUrl: ""
    property string messageId: ""
    property string contactTitle: ""
    property url contactAvatarSource
    property string sentAt: ""
    property string caption: ""
    property real zoomFactor: 1.0
    readonly property bool previewActive: String(imageUrl).length > 0
    readonly property real minimumZoom: 1.0
    readonly property real maximumZoom: 5.0
    readonly property real zoomStep: 0.25

    visible: previewActive
    focus: previewActive
    Accessible.role: Accessible.Pane
    Accessible.name: qsTr("Photo viewer")

    function openImage(url, title, avatarSource, timestampText, imageCaption, sourceMessageId) {
        imageUrl = url
        messageId = sourceMessageId || ""
        contactTitle = title || qsTr("Photo")
        contactAvatarSource = avatarSource || ""
        sentAt = timestampText || ""
        caption = imageCaption || ""
        zoomFactor = minimumZoom
        forceActiveFocus()
    }

    function closePreview() {
        imageUrl = ""
        messageId = ""
        zoomFactor = minimumZoom
    }

    function setZoom(value) {
        zoomFactor = Math.max(minimumZoom, Math.min(maximumZoom, value))
    }

    function zoomIn() {
        setZoom(zoomFactor + zoomStep)
    }

    function zoomOut() {
        setZoom(zoomFactor - zoomStep)
    }

    function adjustZoomFromWheel(delta) {
        if (delta > 0)
            zoomIn()
        else if (delta < 0)
            zoomOut()
    }

    Shortcut {
        sequences: [StandardKey.Cancel]
        enabled: root.previewActive
        onActivated: root.closePreview()
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.surface
    }

    // A full-window input shield keeps clicks from activating the chat behind
    // the viewer. Controls declared later remain above it.
    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.AllButtons
    }

    Rectangle {
        id: toolbar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 72
        color: Theme.surface

        RowLayout {
            anchors.left: parent.left
            anchors.leftMargin: 20
            anchors.right: closeButton.left
            anchors.rightMargin: 12
            anchors.verticalCenter: parent.verticalCenter
            spacing: 10

            Avatar {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                diameter: 44
                title: root.contactTitle
                source: root.contactAvatarSource
                fallbackIdentity: source.toString() === ""
                Accessible.ignored: true
            }

            ColumnLayout {
                Layout.fillWidth: true
                Layout.minimumWidth: 0
                spacing: 1

                Label {
                    Layout.fillWidth: true
                    text: root.contactTitle
                    color: Theme.text
                    font.pixelSize: 15
                    font.weight: Font.Medium
                    elide: Text.ElideRight
                }

                Label {
                    Layout.fillWidth: true
                    visible: text.length > 0
                    text: root.sentAt
                    color: Theme.textMuted
                    font.pixelSize: 12
                    elide: Text.ElideRight
                }
            }
        }

        RowLayout {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.verticalCenter: parent.verticalCenter
            spacing: 4

            ThemedToolButton {
                id: zoomOutButton
                objectName: "chatMediaViewerZoomOut"
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                enabled: root.zoomFactor > root.minimumZoom
                Accessible.name: qsTr("Zoom out")
                onClicked: root.zoomOut()
                contentItem: Label {
                    text: "−"
                    color: parent.enabled ? Theme.icon : Theme.textMuted
                    font.pixelSize: 25
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 22
                    color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
                }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }

            Label {
                Layout.preferredWidth: 54
                text: Math.round(root.zoomFactor * 100) + "%"
                color: Theme.textMuted
                font.pixelSize: 12
                horizontalAlignment: Text.AlignHCenter
            }

            ThemedToolButton {
                id: zoomInButton
                objectName: "chatMediaViewerZoomIn"
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                enabled: root.zoomFactor < root.maximumZoom
                Accessible.name: qsTr("Zoom in")
                onClicked: root.zoomIn()
                contentItem: Label {
                    text: "+"
                    color: parent.enabled ? Theme.icon : Theme.textMuted
                    font.pixelSize: 23
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 22
                    color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
                }
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
        }

        ThemedToolButton {
            id: closeButton
            objectName: "chatMediaViewerCloseButton"
            anchors.right: parent.right
            anchors.rightMargin: 16
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            iconSource: Qt.resolvedUrl("icons/close.svg")
            iconSize: 21
            Accessible.name: qsTr("Close photo viewer")
            onClicked: root.closePreview()
            background: Rectangle {
                radius: 22
                color: parent.down ? Theme.pressedRow : parent.hovered ? Theme.hoverRow : "transparent"
            }
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 1
            color: Theme.border
        }
    }

    Item {
        id: stage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: toolbar.bottom
        anchors.bottom: filmstrip.top
        anchors.margins: 20
        clip: true

        Image {
            id: fullImage
            objectName: "chatMediaViewerImage"
            anchors.fill: parent
            source: root.imageUrl
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
            smooth: true
            scale: root.zoomFactor
            transformOrigin: Item.Center
            Accessible.name: root.caption || qsTr("Shared photo")
        }

        WheelHandler {
            objectName: "chatMediaViewerZoomWheel"
            target: null
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.pixelDelta.y
                root.adjustZoomFromWheel(delta)
                event.accepted = true
            }
        }

        BusyIndicator {
            anchors.centerIn: parent
            width: 44
            height: 44
            running: fullImage.status === Image.Loading
            visible: running
            Accessible.name: qsTr("Loading photo")
        }

        Label {
            anchors.centerIn: parent
            visible: fullImage.status === Image.Error
            text: qsTr("This photo could not be displayed")
            color: Theme.textMuted
            font.pixelSize: 15
        }
    }

    Rectangle {
        id: filmstrip
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 92
        color: Theme.surface

        Rectangle {
            anchors.centerIn: parent
            width: 64
            height: 64
            radius: 6
            color: "transparent"
            border.width: 3
            border.color: Theme.primary

            Image {
                anchors.fill: parent
                anchors.margins: 4
                source: root.imageUrl
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: false
            }
        }

        Label {
            anchors.left: parent.left
            anchors.leftMargin: 20
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(0, (parent.width - 180) / 2)
            text: root.caption
            visible: text.length > 0
            color: Theme.textMuted
            font.pixelSize: 13
            elide: Text.ElideRight
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: 1
            color: Theme.border
        }
    }
}
