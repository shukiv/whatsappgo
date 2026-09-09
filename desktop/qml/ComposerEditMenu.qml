import QtQuick
import QtQuick.Controls
import org.whatsappgo

WhatsAppMenuPopup {
    id: root
    required property var editor
    required property var formatter
    parent: Overlay.overlay
    width: 230

    function showAt(position, localX, localY) {
        formatter.suggestAt(position)
        const point = editor.mapToItem(parent, localX, localY)
        x = Math.max(8, Math.min(point.x, parent.width - width - 8))
        y = Math.max(8, Math.min(point.y, parent.height - implicitHeight - 8))
        open()
    }
    Repeater {
        model: root.formatter.suggestions
        WhatsAppMenuItem {
            required property string modelData
            text: modelData
            onClicked: { root.formatter.replaceSpelling(modelData); root.close(); root.editor.forceActiveFocus() }
        }
    }
    WhatsAppMenuItem {
        text: qsTr("Ignore this word")
        visible: root.formatter.misspelledWord !== ""
        onClicked: { root.formatter.ignoreSpelling(); root.close(); root.editor.forceActiveFocus() }
    }
    WhatsAppMenuItem { text: qsTr("Undo"); enabled: root.editor.canUndo; onClicked: { root.editor.undo(); root.close() } }
    WhatsAppMenuItem { text: qsTr("Redo"); enabled: root.editor.canRedo; onClicked: { root.editor.redo(); root.close() } }
    WhatsAppMenuItem { text: qsTr("Cut"); enabled: root.editor.selectedText !== ""; onClicked: { root.editor.cut(); root.close() } }
    WhatsAppMenuItem { text: qsTr("Copy"); enabled: root.editor.selectedText !== ""; onClicked: { root.editor.copy(); root.close() } }
    WhatsAppMenuItem { text: qsTr("Paste"); enabled: root.editor.canPaste; onClicked: { root.editor.paste(); root.close() } }
    WhatsAppMenuItem { text: qsTr("Select all"); onClicked: { root.editor.selectAll(); root.close() } }
}
