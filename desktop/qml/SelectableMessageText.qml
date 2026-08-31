import QtQuick
import org.whatsappgo

Item {
    id: root

    property string plainText: ""
    property real maximumWidth: 420
    property color color: Theme.text
    property font font: Qt.font({ pixelSize: 14 })
    readonly property alias selectedText: editor.selectedText
    // A message that fits on one line can carry its timestamp beside it.
    //
    // This is decided from the text itself, never from the width the item was
    // given: the width depends on this answer, so reading the editor's line
    // count here would make the two chase each other.
    readonly property bool wrapped: plainText.indexOf("\n") >= 0
        || Math.ceil(naturalMeasure.implicitWidth) + 2 > maximumWidth

    implicitWidth: Math.min(maximumWidth, Math.max(24, Math.ceil(naturalMeasure.implicitWidth) + 2))
    implicitHeight: Math.ceil(editor.contentHeight)
    Accessible.name: plainText

    function copy() {
        editor.copy()
    }

    TextEdit {
        id: editor
        objectName: "messageBody"
        property string activeLink: ""
        anchors.fill: parent
        text: Theme.messageRichText(root.plainText)
        textFormat: TextEdit.RichText
        readOnly: true
        selectByMouse: true
        // Keeping a selection alive after the editor loses focus left the
        // highlight painted in every message that had ever been selected, so
        // a drag appeared to select across several bubbles at once.
        persistentSelection: false
        wrapMode: TextEdit.Wrap
        color: root.color
        selectionColor: Theme.primary
        selectedTextColor: Theme.primaryText
        font: root.font
        renderType: Text.NativeRendering
        onLinkActivated: link => Qt.openUrlExternally(link)
        onLinkHovered: link => activeLink = link

        HoverHandler {
            objectName: "messageLinkHover"
            cursorShape: editor.activeLink !== "" ? Qt.PointingHandCursor : Qt.IBeamCursor
        }
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
