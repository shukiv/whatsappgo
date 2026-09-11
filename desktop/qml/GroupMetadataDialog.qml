import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "groupMetadataDialog"
    required property var client
    property string field: "name"
    property string targetProfile: ""
    property string targetChat: ""
    property string previous: ""
    property string error: ""
    property string saveToken: ""
    property int serial: 0
    property bool saving: false
    property var returnFocus: null
    readonly property bool isName: field === "name"
    readonly property string value: isName ? nameField.text.trim() : descriptionField.text
    // Count Unicode code points, not UTF-16 halves of emoji, matching Go.
    readonly property int characterCount: (value.match(/[\uD800-\uDBFF][\uDC00-\uDFFF]|[\s\S]/g) || []).length
    readonly property int limit: isName ? 100 : 2048
    readonly property bool valid: characterCount <= limit && (!isName || value !== "")
        && !(isName ? /[\u0000-\u001f\u007f-\u009f\u2028\u2029]/ : /[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/).test(value)
    readonly property bool sameTarget: targetProfile === client.profile
        && targetChat !== "" && targetChat === String(client.selectedChat.jid || "")
    signal saved(string message)

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(500, parent ? parent.width - 32 : 500)
    height: Math.min(isName ? (error ? 360 : 300) : 480, parent ? parent.height - 32 : 480)
    padding: 20
    modal: true
    focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    function showFor(which) {
        if (!client.groupInfo.can_edit_info || client.groupActionBusy || client.groupInfoLoading) return
        field = which
        targetProfile = client.profile
        targetChat = String(client.selectedChat.jid || "")
        previous = String(client.groupInfo[which] || "")
        nameField.text = isName ? previous : ""
        descriptionField.text = isName ? "" : previous
        error = ""
        saving = false
        saveToken = ""
        returnFocus = parent && parent.Window.window ? parent.Window.window.activeFocusItem : null
        open()
    }
    onOpened: {
        // ScrollView's focus scope must be visible before focusing its editor.
        if (root.isName) nameField.forceActiveFocus(Qt.TabFocusReason)
        else descriptionField.forceActiveFocus(Qt.TabFocusReason)
    }
    onClosed: {
        saveToken = ""
        Qt.callLater(function() { if (returnFocus && returnFocus.visible) returnFocus.forceActiveFocus(Qt.TabFocusReason) })
    }
    Shortcut { sequence: "Escape"; enabled: root.visible; autoRepeat: false; onActivated: root.close() }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onSelectedChatChanged() {
            if (root.visible && root.targetChat !== String(root.client.selectedChat.jid || "")) root.close()
        }
        function onGroupInfoEditFinished(token, error) {
            if (!root.visible || token !== root.saveToken || !root.sameTarget) return
            root.saving = false
            root.error = error
            if (error === "") {
                root.saved(root.isName ? qsTr("Group name updated") : qsTr("Group description updated"))
                root.close()
            }
        }
    }
    contentItem: ColumnLayout {
        spacing: 12
        Label {
            Layout.fillWidth: true
            text: root.isName ? qsTr("Edit group name") : qsTr("Edit group description")
            color: Theme.text
            font.pixelSize: 20
            font.weight: Font.Medium
        }
        Label {
            Layout.fillWidth: true
            text: qsTr("Changes will be visible to everyone in the group after you save.")
            color: Theme.textMuted
            wrapMode: Text.Wrap
        }
        DialogTextField {
            id: nameField
            objectName: "groupMetadataName"
            Layout.fillWidth: true
            visible: root.isName
            enabled: !root.saving
            Accessible.name: qsTr("Group name")
            placeholderText: qsTr("Group name")
        }
        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: !root.isName
            clip: true
            ScrollBar.vertical: OverlayScrollBar {}
            TextArea {
                id: descriptionField
                objectName: "groupMetadataDescription"
                enabled: !root.saving
                textFormat: TextEdit.PlainText
                wrapMode: TextEdit.Wrap
                selectByMouse: true
                color: Theme.text
                selectionColor: Theme.primary
                selectedTextColor: Theme.primaryText
                placeholderText: qsTr("Add a description (leave empty to remove it)")
                placeholderTextColor: Theme.textMuted
                Accessible.name: qsTr("Group description")
                background: Rectangle { color: Theme.surfaceMuted; radius: 8; border.color: descriptionField.activeFocus ? Theme.primary : Theme.border }
            }
        }
        Label {
            Layout.fillWidth: true
            horizontalAlignment: Text.AlignRight
            text: qsTr("%1 / %2 characters").arg(root.characterCount).arg(root.limit)
            color: root.valid ? Theme.textMuted : Theme.danger
        }
        Label {
            Layout.fillWidth: true
            visible: root.error !== "" || root.saving || !root.client.groupInfo.can_edit_info
            text: root.error || (root.saving ? qsTr("Saving…") : qsTr("You no longer have permission to edit group information."))
            textFormat: Text.PlainText
            color: root.error ? Theme.danger : Theme.textMuted
            wrapMode: Text.Wrap
        }
        Item { Layout.fillHeight: true; visible: root.isName }
        RowLayout {
            Layout.fillWidth: true
            Item { Layout.fillWidth: true }
            ExpressionButton {
                objectName: "groupMetadataCancel"
                text: root.saving ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.close()
            }
            ExpressionButton {
                objectName: "groupMetadataSave"
                text: qsTr("Save")
                primary: true
                enabled: root.valid && root.value !== root.previous && !root.saving
                    && !root.client.groupActionBusy && root.client.groupInfo.can_edit_info
                    && root.client.status.connected === true && root.sameTarget
                onClicked: {
                    root.saveToken = "group-edit:" + (++root.serial)
                    root.error = ""
                    root.saving = true
                    root.client.setGroupInfo(root.targetChat, root.field, root.value, root.previous,
                        root.saveToken, root.targetProfile)
                }
            }
        }
    }
}
