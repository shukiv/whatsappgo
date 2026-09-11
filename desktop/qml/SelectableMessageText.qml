import QtQuick
import org.whatsappgo

Item {
    id: root

    property string plainText: ""
    property var mentions: []
    property bool mentionNavigationEnabled: true
    signal mentionActivated(string jid, string name)
    readonly property var navigableMentions: mentionNavigationEnabled
        ? Theme.mentionParts(plainText, mentions).filter(part => part.mention) : []
    property int focusedMentionIndex: 0
    readonly property string displayText: Theme.mentionDisplayText(plainText, mentions)
    property real maximumWidth: 420
    property color color: Theme.text
    property font font: Qt.font({ pixelSize: 14 })
    readonly property alias selectedText: editor.selectedText
    // A message that fits on one line can carry its timestamp beside it.
    //
    // This is decided from the text itself, never from the width the item was
    // given: the width depends on this answer, so reading the editor's line
    // count here would make the two chase each other.
    readonly property bool wrapped: displayText.indexOf("\n") >= 0
        || Math.ceil(naturalMeasure.implicitWidth) + 2 > maximumWidth

    implicitWidth: Math.min(maximumWidth, Math.max(24, Math.ceil(naturalMeasure.implicitWidth) + 2))
    implicitHeight: Math.ceil(editor.contentHeight)
    activeFocusOnTab: navigableMentions.length > 0
    Accessible.role: navigableMentions.length ? Accessible.Link : Accessible.StaticText
    Accessible.name: activeFocus && navigableMentions[focusedMentionIndex]
        ? qsTr("Open chat with %1").arg(navigableMentions[focusedMentionIndex].name) : displayText
    Accessible.description: navigableMentions.length
        ? qsTr("Press Tab or Shift+Tab to move between tagged names, and Enter to open a chat.") : ""
    Accessible.onPressAction: activateFocusedMention()
    onActiveFocusChanged: {
        focusedMentionIndex = 0
        if (activeFocus) editor.deselect()
    }
    Keys.onPressed: event => {
        if (!activeFocus || !navigableMentions.length) return
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
            event.accepted = true
            activateFocusedMention()
        } else if (event.key === Qt.Key_Tab || event.key === Qt.Key_Backtab) {
            const backwards = event.key === Qt.Key_Backtab || (event.modifiers & Qt.ShiftModifier)
            const next = focusedMentionIndex + (backwards ? -1 : 1)
            if (next >= 0 && next < navigableMentions.length) {
                focusedMentionIndex = next
                event.accepted = true
            }
            // Let normal focus traversal leave the message at either end.
        }
    }

    Rectangle {
        anchors.fill: parent
        anchors.margins: -2
        visible: root.activeFocus && root.navigableMentions.length > 0
        color: "transparent"
        border.color: Theme.primary
        radius: 3
    }

    function activateFocusedMention() {
        const member = navigableMentions[focusedMentionIndex]
        if (member) mentionActivated(member.jid, member.name)
    }

    function copy() {
        editor.copy()
    }

    function activateLink(link) {
        const value = String(link)
        if (value.startsWith("whatsappgo-mention:")) {
            // Only a rendered mention can navigate. Never send an internal
            // identifier to the browser or treat arbitrary message text as one.
            const jid = value.slice("whatsappgo-mention:".length)
            const member = Theme.mentionParts(plainText, mentions).find(part => part.mention && part.jid === jid)
            if (mentionNavigationEnabled && member)
                mentionActivated(member.jid, member.name)
            return
        }
        if (/^https?:\/\//i.test(value))
            Qt.openUrlExternally(value)
    }

    TextEdit {
        id: editor
        objectName: "messageBody"
        property string activeLink: ""
        anchors.fill: parent
        text: Theme.mentionRichText(root.plainText, root.mentions, root.activeFocus ? root.focusedMentionIndex : -1)
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
        onLinkActivated: link => root.activateLink(link)
        onLinkHovered: link => activeLink = link

        HoverHandler {
            objectName: "messageLinkHover"
            cursorShape: editor.activeLink !== "" ? Qt.PointingHandCursor : Qt.IBeamCursor
        }
    }

    Text {
        id: naturalMeasure
        visible: false
        text: root.mentions.length ? Theme.mentionRichText(root.plainText, root.mentions) : root.displayText
        textFormat: root.mentions.length ? Text.RichText : Text.PlainText
        wrapMode: Text.NoWrap
        font: root.font
    }
}
