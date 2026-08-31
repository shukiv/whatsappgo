import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Rectangle {
    id: root
    objectName: "composerLinkPreview"

    property var preview: ({})
    signal dismissed()

    readonly property bool hasPreview: Boolean(preview && preview.url)
    readonly property bool hasThumbnail: Boolean(preview && preview.thumbnail_source)

    visible: hasPreview
    implicitHeight: visible ? (hasThumbnail ? 132 : 94) : 0
    radius: 10
    color: Theme.composer
    border.color: Theme.border
    clip: true

    Image {
        id: previewImage
        objectName: "composerLinkPreviewImage"
        visible: root.hasThumbnail && status !== Image.Error
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: visible ? Math.min(156, parent.width * 0.28) : 0
        source: root.hasThumbnail ? root.preview.thumbnail_source : ""
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        cache: false
    }

    Column {
        anchors.left: previewImage.visible ? previewImage.right : parent.left
        anchors.leftMargin: 12
        anchors.right: dismissButton.left
        anchors.rightMargin: 8
        anchors.verticalCenter: parent.verticalCenter
        spacing: 4

        Label {
            objectName: "composerLinkPreviewTitle"
            width: parent.width
            text: root.preview ? root.preview.title || "" : ""
            color: Theme.text
            font.pixelSize: 14
            font.weight: Font.DemiBold
            elide: Text.ElideRight
            maximumLineCount: 2
            wrapMode: Text.Wrap
        }
        Label {
            width: parent.width
            visible: text !== ""
            text: root.preview ? root.preview.description || "" : ""
            color: Theme.textMuted
            font.pixelSize: 12
            elide: Text.ElideRight
            maximumLineCount: 2
            wrapMode: Text.Wrap
        }
        Label {
            width: parent.width
            text: {
                const raw = root.preview ? String(root.preview.url || "") : ""
                return raw.replace(/^[a-z]+:\/\//i, "").split("/")[0].replace(/^www\./i, "")
            }
            color: Theme.textMuted
            font.pixelSize: 12
            elide: Text.ElideRight
        }
    }

    ThemedToolButton {
        id: dismissButton
        objectName: "composerLinkPreviewDismiss"
        anchors.right: parent.right
        anchors.rightMargin: 6
        anchors.top: parent.top
        anchors.topMargin: 6
        width: 36
        height: 36
        iconSource: Qt.resolvedUrl("icons/close.svg")
        iconSize: 18
        Accessible.name: qsTr("Remove link preview")
        onClicked: root.dismissed()
        background: Rectangle { radius: 18; color: parent.hovered ? Theme.hoverRow : "transparent" }
    }
}
