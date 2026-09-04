import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// The application's own modal, replacing Kirigami's platform dialogs: a centred
// card on a dimmed page, with the title, the body and the actions stacked the
// way WhatsApp Web stacks them.
Popup {
    id: root

    property string title
    property string subtitle
    property string acceptText: qsTr("OK")
    property string cancelText: qsTr("Cancel")
    property bool showAccept: true
    property bool showCancel: true
    property bool acceptEnabled: true
    // Named so a caller's own tests and accessibility tooling can address the
    // confirming action by what it does, not by its place in the dialog.
    property string acceptName: "dialogAcceptButton"
    // A destructive confirmation puts its weight on the red action, the way
    // WhatsApp Web does for leaving, clearing and deleting.
    property bool destructive: false
    property int preferredWidth: 420
    property int preferredHeight: 0

    default property alias dialogContent: bodyColumn.data

    signal accepted()
    signal rejected()

    parent: Overlay.overlay
    anchors.centerIn: Overlay.overlay
    width: parent ? Math.min(parent.width - 80, preferredWidth) : preferredWidth
    height: preferredHeight > 0
        ? (parent ? Math.min(parent.height - 80, preferredHeight) : preferredHeight)
        : implicitHeight
    modal: true
    focus: true
    padding: 0
    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

    function accept() {
        root.accepted()
        root.close()
    }

    function reject() {
        root.rejected()
        root.close()
    }

    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }

    background: Rectangle {
        radius: 12
        color: Theme.surface
        border.color: Theme.border
        border.width: 1
    }

    contentItem: ColumnLayout {
        spacing: 0

        Label {
            objectName: "dialogTitle"
            Layout.fillWidth: true
            Layout.leftMargin: 24
            Layout.rightMargin: 24
            Layout.topMargin: 22
            visible: root.title !== ""
            text: root.title
            color: Theme.text
            font.pixelSize: 17
            font.weight: Font.Medium
            wrapMode: Text.Wrap
        }

        Label {
            objectName: "dialogSubtitle"
            Layout.fillWidth: true
            Layout.leftMargin: 24
            Layout.rightMargin: 24
            Layout.topMargin: root.title !== "" ? 8 : 22
            visible: root.subtitle !== ""
            text: root.subtitle
            color: Theme.textMuted
            font.pixelSize: 14
            wrapMode: Text.Wrap
        }

        // Bodyless confirmations collapse to nothing rather than leaving a gap
        // between the subtitle and the actions.
        ColumnLayout {
            id: bodyColumn
            objectName: "dialogBody"
            Layout.fillWidth: true
            Layout.fillHeight: root.preferredHeight > 0
            Layout.leftMargin: 24
            Layout.rightMargin: 24
            Layout.topMargin: children.length > 0 ? 16 : 0
            spacing: 12
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 20
            spacing: 8
            visible: root.showAccept || root.showCancel

            Item { Layout.fillWidth: true }

            AbstractButton {
                id: cancelButton
                objectName: "dialogCancelButton"
                visible: root.showCancel
                implicitWidth: cancelLabel.implicitWidth + 36
                implicitHeight: 38
                Accessible.name: root.cancelText
                onClicked: root.reject()
                background: Rectangle {
                    radius: 19
                    color: cancelButton.hovered ? Theme.hoverRow : "transparent"
                }
                contentItem: Label {
                    id: cancelLabel
                    text: root.cancelText
                    color: Theme.text
                    font.pixelSize: 14
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
            }

            AbstractButton {
                id: acceptButton
                objectName: root.acceptName
                visible: root.showAccept
                enabled: root.acceptEnabled
                implicitWidth: acceptLabel.implicitWidth + 40
                implicitHeight: 38
                Accessible.name: root.acceptText
                onClicked: root.accept()
                background: Rectangle {
                    radius: 19
                    color: !acceptButton.enabled
                        ? Theme.surfaceMuted
                        : root.destructive
                            ? (acceptButton.hovered ? Qt.darker(Theme.danger, 1.1) : Theme.danger)
                            : (acceptButton.hovered ? Qt.darker(Theme.primary, 1.1) : Theme.primary)
                }
                contentItem: Label {
                    id: acceptLabel
                    text: root.acceptText
                    color: acceptButton.enabled ? Theme.primaryText : Theme.textMuted
                    font.pixelSize: 14
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }
    }
}
