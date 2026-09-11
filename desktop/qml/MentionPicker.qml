import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "mentionPicker"
    required property var client
    required property var editor
    required property var mentionComposer
    property string contextKey: ""
    property bool available: false
    property int queryStart: -1
    property string query: ""
    property string dismissed: ""
    property string requestedChat: ""
    property var matches: []
    property int selectedIndex: 0
    readonly property string chatJid: String(client.selectedChat.jid || "")
    readonly property var roster: String(client.groupInfo.jid || "") === chatJid ? (client.groupInfo.participants || []) : []
    padding: 8
    width: Math.min(380, parent ? parent.width - 16 : 380)
    height: Math.min(296, Math.max(80, matches.length * 56 + 16))
    function reposition() {
        if (!parent || !editor) return
        const point = editor.mapToItem(parent, 0, 0)
        x = Math.max(8, Math.min(parent.width - width - 8, point.x))
        y = Math.max(8, Math.min(parent.height - height - 8, point.y - height - 10))
    }
    // mapToItem does not subscribe to ancestor layout changes. Reposition
    // after layout settles so resizing never leaves suggestions offscreen.
    onAboutToShow: Qt.callLater(reposition)
    onWidthChanged: Qt.callLater(reposition)
    onHeightChanged: Qt.callLater(reposition)
    modal: false
    focus: false
    closePolicy: Popup.CloseOnPressOutside
    // Suggestions must not finish an old close animation after a new query.
    enter: Transition {}
    exit: Transition {}
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }
    function dismiss() { dismissed = String(editor.text).slice(0, editor.cursorPosition); close() }
    function reset() { close(); requestedChat = ""; dismissed = "" }
    function update() {
        if (!available || !editor.activeFocus || editor.inputMethodComposing || !chatJid.endsWith("@g.us")) { close(); return }
        const prefix = String(editor.text).slice(0, editor.cursorPosition)
        const match = prefix.match(/(?:^|[\s([{])@([^@\n`]{0,64})$/)
        if (!match || prefix === dismissed || prefix.split("`").length % 2 === 0) { close(); return }
        const start = prefix.length - match[1].length - 1
        const spans = mentionComposer.ranges
        for (let i = 0; i < spans.length; ++i) {
            if (spans[i].start === start) { close(); return }
        }
        queryStart = start
        query = match[1].toLocaleLowerCase()
        const found = []
        for (let i = 0; i < roster.length; ++i) {
            const person = roster[i]
            if (person.is_self || !/^[0-9]+@(s\.whatsapp\.net|lid)$/.test(String(person.jid || ""))) continue
            const name = String(person.name || person.phone || qsTr("Member · %1").arg(String(person.jid).split("@")[0].slice(-4)))
            if (query && (name + " " + String(person.phone || "")).toLocaleLowerCase().indexOf(query) < 0) continue
            found.push({ jid: person.jid, name: name, phone: String(person.phone || ""), avatar_path: String(person.avatar_path || "") })
        }
        matches = found
        selectedIndex = 0
        if (!visible) open()
        if (requestedChat !== chatJid) { requestedChat = chatJid; client.refreshGroupInfo() }
    }
    function choose(index) {
        if (index < 0 || index >= matches.length || requestedChat !== chatJid || !available) return false
        const member = matches[index]
        if (!mentionComposer.insert(queryStart, editor.cursorPosition, member.jid, member.name)) return false
        dismiss()
        editor.forceActiveFocus()
        return true
    }
    function handleKey(event) {
        if (!visible || event.modifiers !== Qt.NoModifier) return false
        if (event.key === Qt.Key_Escape) { dismiss(); return true }
        if (event.key === Qt.Key_Down || event.key === Qt.Key_Up) {
            if (matches.length) {
                selectedIndex = (selectedIndex + (event.key === Qt.Key_Down ? 1 : matches.length - 1)) % matches.length
                memberList.positionViewAtIndex(selectedIndex, ListView.Contain)
            }
            return true
        }
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Tab) {
            choose(selectedIndex)
            return true // Loading/no results must never turn Enter into Send.
        }
        return false
    }
    onContextKeyChanged: reset()
    onAvailableChanged: update()
    onRosterChanged: update()
    onAboutToHide: dismissed = String(editor.text).slice(0, editor.cursorPosition)
    Connections {
        target: root.editor
        function onHeightChanged() { Qt.callLater(root.reposition) }
        function onTextChanged() { Qt.callLater(root.update) }
        function onCursorPositionChanged() { Qt.callLater(root.update) }
        function onActiveFocusChanged() { Qt.callLater(root.update) }
        function onInputMethodComposingChanged() { Qt.callLater(root.update) }
    }
    Connections {
        target: root.parent
        function onWidthChanged() { Qt.callLater(root.reposition) }
        function onHeightChanged() { Qt.callLater(root.reposition) }
    }
    contentItem: Item {
        ListView {
            id: memberList
            objectName: "mentionMemberList"
            anchors.fill: parent
            clip: true
            model: root.matches
            currentIndex: root.selectedIndex
            ScrollBar.vertical: OverlayScrollBar {}
            delegate: ItemDelegate {
                required property var modelData
                required property int index
                width: memberList.width
                height: 56
                focusPolicy: Qt.NoFocus
                Accessible.name: qsTr("Mention %1").arg(modelData.name)
                Accessible.role: Accessible.ListItem
                Accessible.selected: index === root.selectedIndex
                onClicked: root.choose(index)
                background: Rectangle { radius: 8; color: parent.down ? Theme.pressedRow : parent.hovered || root.selectedIndex === parent.index ? Theme.hoverRow : "transparent" }
                contentItem: RowLayout {
                    spacing: 10
                    Avatar { diameter: 36; Layout.preferredWidth: 36; Layout.preferredHeight: 36; title: modelData.name; source: Theme.fileUrl(modelData.avatar_path); Accessible.ignored: true }
                    Label { Layout.fillWidth: true; Layout.minimumWidth: 0; text: modelData.name; textFormat: Text.PlainText; elide: Text.ElideRight; color: Theme.text; font.pixelSize: 14 }
                }
                ToolTip.visible: hovered
                ToolTip.text: modelData.name
            }
        }
        ColumnLayout {
            anchors.centerIn: parent
            visible: root.matches.length === 0
            Label { text: root.client.groupInfoLoading ? qsTr("Loading group members…") : root.client.groupInfoError ? qsTr("Could not load group members") : qsTr("No matching members"); color: Theme.textMuted; font.pixelSize: 13 }
            Button { visible: Boolean(root.client.groupInfoError) && !root.client.groupInfoLoading; text: qsTr("Retry"); onClicked: root.client.refreshGroupInfo() }
        }
    }
}
