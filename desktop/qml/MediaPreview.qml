import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Item {
    id: root
    objectName: "mediaPreviewOverlay"

    property url imageUrl: ""
    property real imageRotation: 0
    readonly property bool previewActive: String(imageUrl).length > 0
    signal sendRequested(url imageUrl, string caption)
    signal canceled(url imageUrl)
    signal addRequested()

    visible: previewActive
    Accessible.name: qsTr("Image preview")

    function openImage(url) {
        imageUrl = url
        imageRotation = 0
        caption.clear()
        Qt.callLater(() => caption.forceActiveFocus())
    }

    function closePreview() {
        const discardedUrl = imageUrl
        imageUrl = ""
        canceled(discardedUrl)
    }

    Rectangle {
        anchors.fill: parent
        color: Theme.surface
    }

    Rectangle {
        id: toolbar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 68
        color: Theme.surface

        ThemedToolButton {
            anchors.left: parent.left
            anchors.leftMargin: 16
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            height: 44
            iconSource: Qt.resolvedUrl("icons/close.svg")
            iconSize: 20
            Accessible.name: qsTr("Close image preview")
            onClicked: root.closePreview()
            background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
        }

        RowLayout {
            anchors.centerIn: parent
            spacing: 8

            ThemedToolButton {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/rotate-left.svg")
                iconSize: 21
                Accessible.name: qsTr("Rotate left")
                onClicked: root.imageRotation -= 90
                background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
            ThemedToolButton {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/rotate-right.svg")
                iconSize: 21
                Accessible.name: qsTr("Rotate right")
                onClicked: root.imageRotation += 90
                background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
            ThemedToolButton {
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/edit.svg")
                iconSize: 20
                Accessible.name: qsTr("Edit caption")
                onClicked: caption.forceActiveFocus()
                background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
            ThemedToolButton {
                id: previewEmojiButton
                Layout.preferredWidth: 44
                Layout.preferredHeight: 44
                iconSource: Qt.resolvedUrl("icons/smile.svg")
                iconSize: 21
                Accessible.name: qsTr("Add emoji to caption")
                onClicked: previewEmoji.opened ? previewEmoji.close() : previewEmoji.open()
                background: Rectangle { radius: 22; color: parent.hovered || previewEmoji.opened ? Theme.hoverRow : "transparent" }
            }
        }
    }

    Item {
        id: stage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: toolbar.bottom
        anchors.bottom: footer.top
        anchors.margins: 20

        Image {
            id: previewImage
            anchors.centerIn: parent
            width: Math.min(sourceSize.width > 0 ? sourceSize.width : 520, parent.width - 40)
            height: Math.min(sourceSize.height > 0 ? sourceSize.height : 520, parent.height - 36)
            source: root.imageUrl
            rotation: root.imageRotation
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: false
            smooth: true
            Accessible.name: qsTr("Image to send")
        }
    }

    Item {
        id: footer
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 142

        Rectangle {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.top
            width: Math.min(parent.width - 150, 760)
            height: 56
            radius: 10
            color: Theme.surfaceMuted

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 14
                anchors.rightMargin: 6
                spacing: 4

                TextArea {
                    id: caption
                    objectName: "mediaPreviewCaption"
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    leftPadding: 0
                    rightPadding: 0
                    topPadding: 15
                    bottomPadding: 8
                    placeholderText: qsTr("Add a caption")
                    color: Theme.text
                    font.pixelSize: 14
                    wrapMode: TextEdit.Wrap
                    Accessible.name: qsTr("Image caption")
                    background: Item {}
                    Keys.onPressed: event => {
                        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && !(event.modifiers & Qt.ShiftModifier)) {
                            sendButton.clicked()
                            event.accepted = true
                        }
                    }
                }

                ThemedToolButton {
                    Layout.preferredWidth: 44
                    Layout.preferredHeight: 44
                    iconSource: Qt.resolvedUrl("icons/smile.svg")
                    iconSize: 21
                    Accessible.name: qsTr("Add emoji to caption")
                    onClicked: previewEmoji.opened ? previewEmoji.close() : previewEmoji.open()
                    background: Rectangle { radius: 22; color: parent.hovered ? Theme.hoverRow : "transparent" }
                }
            }
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 16
            spacing: 12

            Rectangle {
                width: 66
                height: 66
                radius: 6
                color: "transparent"
                border.width: 2
                border.color: Theme.primary
                Image {
                    anchors.fill: parent
                    anchors.margins: 3
                    source: root.imageUrl
                    fillMode: Image.PreserveAspectCrop
                    cache: false
                }
            }

            Button {
                width: 66
                height: 66
                text: "+"
                font.pixelSize: 28
                font.weight: Font.Light
                Accessible.name: qsTr("Replace with clipboard image")
                onClicked: root.addRequested()
                contentItem: Label {
                    text: parent.text
                    color: Theme.text
                    font: parent.font
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 6
                    color: parent.hovered ? Theme.hoverRow : Theme.surface
                    border.width: 1
                    border.color: Theme.border
                }
            }
        }

        RoundButton {
            id: sendButton
            objectName: "mediaPreviewSendButton"
            anchors.right: parent.right
            anchors.rightMargin: 24
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 16
            width: 64
            height: 64
            enabled: root.previewActive
            Accessible.name: qsTr("Send image")
            onClicked: {
                const sentUrl = root.imageUrl
                const sentCaption = caption.text
                root.imageUrl = ""
                root.sendRequested(sentUrl, sentCaption)
            }
            background: Rectangle {
                radius: width / 2
                color: sendButton.down ? Qt.darker(Theme.primary, 1.12) : Theme.primary
            }
            contentItem: TintedIcon {
                width: 28
                height: 28
                source: Qt.resolvedUrl("icons/send.svg")
                tint: Theme.primaryText
            }
        }
    }

    EmojiPicker {
        id: previewEmoji
        parent: root
        x: Math.max(12, (root.width - width) / 2)
        y: Math.max(76, footer.y - height - 8)
        onEmojiChosen: emoji => {
            const position = Math.max(0, caption.cursorPosition)
            caption.insert(position, emoji)
            caption.cursorPosition = position + emoji.length
            caption.forceActiveFocus()
        }
    }
}
