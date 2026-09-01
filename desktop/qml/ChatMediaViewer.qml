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
    property real panX: 0
    property real panY: 0
    readonly property bool previewActive: String(imageUrl).length > 0
    readonly property real minimumZoom: 1.0
    readonly property real maximumZoom: 5.0
    readonly property real zoomRatio: 1.2

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
        panX = 0
        panY = 0
        forceActiveFocus()
    }

    function closePreview() {
        imageUrl = ""
        messageId = ""
        zoomFactor = minimumZoom
        panX = 0
        panY = 0
    }

    function clampPan() {
        const maxX = stage.width * Math.max(0, zoomFactor - 1) / 2
        const maxY = stage.height * Math.max(0, zoomFactor - 1) / 2
        panX = Math.max(-maxX, Math.min(maxX, panX))
        panY = Math.max(-maxY, Math.min(maxY, panY))
    }

    function setZoomAt(value, pointerX, pointerY) {
        const oldZoom = zoomFactor
        const nextZoom = Math.max(minimumZoom, Math.min(maximumZoom, value))
        if (Math.abs(nextZoom - oldZoom) < 0.0001)
            return
        if (nextZoom <= minimumZoom) {
            zoomFactor = minimumZoom
            panX = 0
            panY = 0
            return
        }
        const centerX = stage.width / 2
        const centerY = stage.height / 2
        const ratio = nextZoom / oldZoom
        panX = pointerX - centerX - (pointerX - centerX - panX) * ratio
        panY = pointerY - centerY - (pointerY - centerY - panY) * ratio
        zoomFactor = nextZoom
        clampPan()
    }

    function zoomIn() {
        setZoomAt(zoomFactor * zoomRatio, stage.width / 2, stage.height / 2)
    }

    function zoomOut() {
        setZoomAt(zoomFactor / zoomRatio, stage.width / 2, stage.height / 2)
    }

    function adjustZoomFromWheel(delta, pointerX, pointerY) {
        if (delta === 0)
            return
        // Multiplicative zoom respects both a mouse-wheel notch and the smaller,
        // continuous deltas emitted by a touchpad.
        const factor = Math.exp(delta * 0.0015)
        setZoomAt(zoomFactor * factor,
                  pointerX === undefined ? stage.width / 2 : pointerX,
                  pointerY === undefined ? stage.height / 2 : pointerY)
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

        Item {
            id: zoomSurface
            objectName: "chatMediaViewerZoomSurface"
            anchors.fill: parent
            anchors.leftMargin: root.panX
            anchors.rightMargin: -root.panX
            anchors.topMargin: root.panY
            anchors.bottomMargin: -root.panY
            scale: root.zoomFactor
            transformOrigin: Item.Center
            layer.enabled: true
            layer.smooth: true
            readonly property bool renderCached: layer.enabled

            Image {
                id: fullImage
                objectName: "chatMediaViewerImage"
                anchors.fill: parent
                source: root.imageUrl
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                cache: true
                smooth: true
                mipmap: true
                Accessible.name: root.caption || qsTr("Shared photo")
            }
        }

        WheelHandler {
            objectName: "chatMediaViewerZoomWheel"
            target: null
            acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            onWheel: event => {
                const delta = event.angleDelta.y !== 0 ? event.angleDelta.y : event.pixelDelta.y
                root.adjustZoomFromWheel(delta, event.position.x, event.position.y)
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
