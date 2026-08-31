import QtQuick
import org.whatsappgo

Item {
    id: root

    property string plainText: ""
    property real maximumWidth: 420
    property color color: Theme.text
    property font font: Qt.font({ pixelSize: 14 })
    readonly property alias selectedText: editor.selectedText

    implicitWidth: Math.min(maximumWidth, Math.max(24, Math.ceil(naturalMeasure.implicitWidth) + 2))
    implicitHeight: Math.ceil(editor.contentHeight)
    Accessible.name: plainText

    function copy() {
        editor.copy()
    }

    TextEdit {
        id: editor
        objectName: "messageBody"
        anchors.fill: parent
        text: Theme.messageRichText(root.plainText)
        textFormat: TextEdit.RichText
        readOnly: true
        selectByMouse: true
        persistentSelection: true
        wrapMode: TextEdit.Wrap
        color: root.color
        selectionColor: Theme.primary
        selectedTextColor: Theme.primaryText
        font: root.font
        renderType: Text.NativeRendering
        onLinkActivated: link => Qt.openUrlExternally(link)
    }

    Text {
        id: naturalMeasure
        visible: false
        text: root.plainText
        textFormat: Text.PlainText
        wrapMode: Text.NoWrap
        font: root.font
    }
}
