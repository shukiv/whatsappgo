import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// Read-only identity text: never interpret names as HTML or shorten clipboard
// values. Keep the explicit copy button independent of the current selection.
Item {
    id: root
    property string text: ""
    property string copyLabel: qsTr("Copy")
    property color color: Theme.text
    property font font: Qt.font({pixelSize: 16})
    property bool centered: false
    property bool copyEnabled: text !== ""
    property bool editEnabled: false
    property string editLabel: qsTr("Edit")
    property string capturedSelection: ""
    signal copyRequested(string value)
    signal editRequested()
    implicitHeight: Math.max(32, editor.implicitHeight)

    function copyValue(value) {
        if (!copyEnabled || !value) return
        copyRequested(value)
        copiedTimer.restart()
    }
    function openCopyMenu() {
        if (!copyEnabled) return
        capturedSelection = editor.selectedText || root.text
        copyMenu.openUnder(editor)
    }
    onTextChanged: { copiedTimer.stop(); copyMenu.close(); capturedSelection = "" }

    RowLayout {
        // Center the text and its action as one compact unit. Measuring an
        // unwrapped label keeps this independent of the editor's wrap width.
        width: Math.min(root.width, Math.ceil(naturalMeasure.implicitWidth) + (copyButton.visible ? 30 : 0) + (editButton.visible ? 30 : 0))
        height: parent.height
        x: root.centered ? Math.round((root.width - width) / 2) : 0
        spacing: 2
        TextEdit {
            id: editor
            objectName: root.objectName + "Text"
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            Layout.alignment: Qt.AlignVCenter
            text: root.text
            textFormat: TextEdit.PlainText
            readOnly: true
            selectByMouse: true
            selectByKeyboard: true
            activeFocusOnTab: true
            wrapMode: TextEdit.Wrap
            horizontalAlignment: root.centered ? TextEdit.AlignHCenter : TextEdit.AlignLeft
            font: root.font
            color: root.color
            selectionColor: Theme.primary
            selectedTextColor: Theme.primaryText
            Accessible.name: root.text
            Keys.onPressed: event => {
                if (event.matches(StandardKey.Copy) && selectedText !== "") {
                    root.copyValue(selectedText)
                    event.accepted = true
                } else if (event.key === Qt.Key_Menu || (event.key === Qt.Key_F10 && event.modifiers & Qt.ShiftModifier)) {
                    root.openCopyMenu()
                    event.accepted = true
                }
            }
            HoverHandler { cursorShape: Qt.IBeamCursor }
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.RightButton
                onClicked: root.openCopyMenu()
            }
        }
        ThemedToolButton {
            id: copyButton
            objectName: root.objectName + "CopyButton"
            Layout.preferredWidth: 28
            Layout.preferredHeight: 28
            visible: root.copyEnabled
            iconSize: 14
            iconSource: Qt.resolvedUrl(copiedTimer.running ? "icons/check.svg" : "icons/copy.svg")
            iconTint: copiedTimer.running ? Theme.primary : Theme.icon
            focusPolicy: Qt.StrongFocus
            Accessible.name: root.copyLabel
            ToolTip.visible: hovered
            ToolTip.text: copiedTimer.running ? qsTr("Copied") : root.copyLabel
            onClicked: root.copyValue(root.text)
            background: Rectangle {
                radius: 14
                color: copyButton.down ? Theme.pressedRow : copyButton.hovered ? Theme.hoverRow : "transparent"
                border.width: copyButton.activeFocus ? 1 : 0
                border.color: Theme.primary
            }
        }
        ThemedToolButton {
            id: editButton
            objectName: root.objectName + "EditButton"
            Layout.preferredWidth: 28
            Layout.preferredHeight: 28
            visible: root.editEnabled
            iconSize: 14
            iconSource: Qt.resolvedUrl("icons/edit.svg")
            focusPolicy: Qt.StrongFocus
            Accessible.name: root.editLabel
            ToolTip.visible: hovered
            ToolTip.text: root.editLabel
            onClicked: root.editRequested()
            background: Rectangle {
                radius: 14
                color: editButton.down ? Theme.pressedRow : editButton.hovered ? Theme.hoverRow : "transparent"
                border.width: editButton.activeFocus ? 1 : 0
                border.color: Theme.primary
            }
        }
    }
    Text {
        id: naturalMeasure
        visible: false
        text: root.text
        textFormat: Text.PlainText
        font: root.font
        wrapMode: Text.NoWrap
    }
    Timer { id: copiedTimer; interval: 1500 }
    WhatsAppMenuPopup {
        id: copyMenu
        parent: Overlay.overlay
        WhatsAppMenuItem {
            objectName: root.objectName + "CopyMenuItem"
            text: qsTr("Copy")
            iconSource: Qt.resolvedUrl("icons/copy.svg")
            onClicked: { copyMenu.close(); root.copyValue(root.capturedSelection) }
        }
        WhatsAppMenuItem {
            text: qsTr("Select all")
            onClicked: { copyMenu.close(); editor.forceActiveFocus(); editor.selectAll() }
        }
    }
}
