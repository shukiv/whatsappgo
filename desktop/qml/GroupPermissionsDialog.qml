import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "groupPermissionsDialog"
    required property var client
    property string targetChat: ""
    property string targetProfile: ""
    property string field: ""
    property bool previous: false
    property bool value: false
    property bool saving: false
    property string error: ""
    property string feedback: ""
    property string saveToken: ""
    property int serial: 0
    property var returnFocus: null
    readonly property bool sameTarget: targetProfile === client.profile
        && targetChat !== "" && targetChat === String(client.selectedChat.jid || "")
    readonly property var permissions: client.groupInfo.permissions || ({})
    readonly property bool canEdit: sameTarget && Boolean(client.groupInfo.can_edit_permissions)
        && Boolean(client.groupInfo.is_member)
    readonly property var settings: [
        {key: "send_messages", title: qsTr("Send messages"), icon: "chats", help: qsTr("Choose who can send messages to this group.")},
        {key: "edit_info", title: qsTr("Edit group information"), icon: "edit", help: qsTr("Choose who can change the group name, photo and description.")},
        {key: "add_members", title: qsTr("Add other members"), icon: "user-add", help: qsTr("Choose who can add people to this group.")},
        {key: "approve_new_members", title: qsTr("Approve new members"), icon: "shield", help: qsTr("When on, admins approve requests from people who want to join the group.")}
    ]
    readonly property var setting: settings.find(entry => entry.key === field)
    signal saved(string message)

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(520, parent ? parent.width - 32 : 520)
    height: Math.min(540, parent ? parent.height - 32 : 540)
    padding: 20
    modal: true
    focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    // Keep native radio/keyboard behavior, but use our semantic colors instead
    // of a desktop control style that may ignore the app's dark palette.
    component PermissionChoice: RadioDelegate {
        id: choice
        implicitHeight: Math.max(48, contentItem.implicitHeight + topPadding + bottomPadding)
        padding: 12
        spacing: 12
        opacity: enabled ? 1 : 0.5
        contentItem: Label {
            text: choice.text
            font: choice.font
            color: Theme.text
            wrapMode: Text.Wrap
            leftPadding: choice.mirrored ? 0 : choice.indicator.width + choice.spacing
            rightPadding: choice.mirrored ? choice.indicator.width + choice.spacing : 0
        }
        indicator: Rectangle {
            width: 20; height: 20; radius: 10
            x: choice.mirrored ? choice.width - width - choice.rightPadding : choice.leftPadding
            y: (choice.height - height) / 2
            color: "transparent"
            border.width: 2
            border.color: choice.checked ? Theme.primary : Theme.icon
            Rectangle {
                anchors.centerIn: parent
                width: 10; height: 10; radius: 5
                visible: choice.checked
                color: Theme.primary
            }
        }
        background: Rectangle {
            radius: 8
            color: choice.down ? Theme.pressedRow : choice.hovered ? Theme.hoverRow : "transparent"
            border.width: choice.activeFocus ? 1 : 0
            border.color: Theme.primary
        }
    }

    function showForGroup() {
        const jid = String(client.selectedChat.jid || "")
        if (!jid.endsWith("@g.us") || client.groupInfo.jid !== jid
                || !client.groupInfo.is_member || client.groupActionBusy || client.groupInfoLoading) return
        targetChat = jid
        targetProfile = client.profile
        field = ""
        error = feedback = saveToken = ""
        saving = false
        returnFocus = parent && parent.Window.window ? parent.Window.window.activeFocusItem : null
        open()
    }
    function editField(key) {
        if (!canEdit || saving || client.groupInfoLoading || typeof permissions[key] !== "boolean") return
        field = key
        previous = value = permissions[key]
        error = feedback = ""
        Qt.callLater(function() { (root.value ? allowChoice : restrictChoice).forceActiveFocus(Qt.TabFocusReason) })
    }
    function goBack() {
        if (saving) { close(); return }
        if (field !== "") {
            const key = field
            field = ""
            error = ""
            Qt.callLater(function() {
                for (let i = 0; i < rows.count; ++i)
                    if (root.settings[i].key === key) rows.itemAt(i).forceActiveFocus(Qt.TabFocusReason)
            })
        } else close()
    }
    function save() {
        if (!saveButton.enabled) return
        saveToken = "group-permission:" + (++serial)
        saving = true
        error = ""
        client.setGroupPermission(targetChat, field, value, previous, saveToken, targetProfile)
    }
    onOpened: closeButton.forceActiveFocus(Qt.TabFocusReason)
    onClosed: {
        saveToken = ""
        Qt.callLater(function() {
            if (root.sameTarget && returnFocus && returnFocus.visible) returnFocus.forceActiveFocus(Qt.TabFocusReason)
        })
    }
    Shortcut { sequence: "Escape"; enabled: root.visible; autoRepeat: false; onActivated: root.goBack() }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onSelectedChatChanged() {
            // Read the signal source: the sameTarget binding can still hold
            // the previous value while selectedChatChanged is being delivered.
            if (root.visible && String(root.client.selectedChat.jid || "") !== root.targetChat) root.close()
        }
        function onGroupPermissionEditFinished(token, error) {
            if (!root.visible || token !== root.saveToken || !root.sameTarget) return
            root.saving = false
            root.error = error
            if (error === "") {
                root.feedback = qsTr("Group permission updated")
                root.saved(root.feedback)
                root.goBack()
            }
        }
    }

    contentItem: ColumnLayout {
        spacing: 12
        RowLayout {
            Layout.fillWidth: true
            ThemedToolButton {
                visible: root.field !== ""
                enabled: !root.saving
                iconSource: Qt.resolvedUrl("icons/back.svg")
                Accessible.name: qsTr("Back to group permissions")
                onClicked: root.goBack()
            }
            Label {
                Layout.fillWidth: true
                text: root.setting ? root.setting.title : qsTr("Group permissions")
                color: Theme.text
                font.pixelSize: 20
                font.weight: Font.Medium
                wrapMode: Text.Wrap
            }
        }
        ScrollView {
            id: scroll
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
            ScrollBar.vertical: OverlayScrollBar {}
            Column {
                width: scroll.availableWidth
                spacing: 8
                Column {
                    visible: root.field === ""
                    width: parent.width
                    Repeater {
                        id: rows
                        model: root.settings
                        GroupInfoRow {
                            required property var modelData
                            objectName: "groupPermission-" + modelData.key
                            text: modelData.title
                            subtitle: typeof root.permissions[modelData.key] !== "boolean" ? qsTr("Not supplied by WhatsApp")
                                : modelData.key === "approve_new_members" ? (root.permissions[modelData.key] ? qsTr("On") : qsTr("Off"))
                                : root.permissions[modelData.key] ? qsTr("All members") : qsTr("Only admins")
                            iconSource: Qt.resolvedUrl("icons/" + modelData.icon + ".svg")
                            enabled: root.canEdit && !root.client.groupInfoLoading && !root.client.groupActionBusy
                                && typeof root.permissions[modelData.key] === "boolean"
                            onClicked: root.editField(modelData.key)
                        }
                    }
                }
                Column {
                    visible: root.field !== ""
                    width: parent.width
                    spacing: 12
                    Label {
                        width: parent.width
                        text: root.setting ? root.setting.help : ""
                        color: Theme.textMuted
                        wrapMode: Text.Wrap
                    }
                    ButtonGroup { id: choices }
                    PermissionChoice {
                        id: allowChoice
                        objectName: "groupPermissionAllow"
                        width: parent.width
                        text: root.field === "approve_new_members" ? qsTr("On") : qsTr("All members")
                        checked: root.value
                        enabled: root.canEdit && !root.saving
                        ButtonGroup.group: choices
                        onClicked: root.value = true
                    }
                    PermissionChoice {
                        id: restrictChoice
                        objectName: "groupPermissionRestrict"
                        width: parent.width
                        text: root.field === "approve_new_members" ? qsTr("Off") : qsTr("Only admins")
                        checked: !root.value
                        enabled: root.canEdit && !root.saving
                        ButtonGroup.group: choices
                        onClicked: root.value = false
                    }
                    Label { width: parent.width; text: qsTr("This setting changes for everyone in the group after you save."); color: Theme.textMuted; wrapMode: Text.Wrap }
                }
            }
        }
        Label {
            Layout.fillWidth: true
            visible: text !== ""
            text: root.error || (root.saving ? qsTr("Saving… Closing does not cancel a submitted change.")
                : !root.canEdit ? qsTr("Only current group admins can change these permissions.")
                : root.client.status.connected !== true ? qsTr("Connect to WhatsApp to save changes.")
                : root.feedback || root.client.groupInfoError)
            textFormat: Text.PlainText
            color: root.error ? Theme.danger : Theme.textMuted
            wrapMode: Text.Wrap
            Accessible.name: text
        }
        RowLayout {
            Layout.fillWidth: true
            ExpressionButton {
                text: qsTr("Refresh")
                visible: root.field === ""
                enabled: !root.client.groupInfoLoading && !root.client.groupActionBusy && root.client.status.connected === true
                onClicked: { root.feedback = ""; root.client.refreshGroupInfo() }
            }
            Item { Layout.fillWidth: true }
            ExpressionButton {
                id: closeButton
                objectName: "groupPermissionCancel"
                text: root.field === "" || root.saving ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.goBack()
            }
            ExpressionButton {
                id: saveButton
                objectName: "groupPermissionSave"
                visible: root.field !== ""
                text: qsTr("Save")
                primary: true
                enabled: root.field !== "" && root.value !== root.previous && root.canEdit && !root.saving
                    && !root.client.groupActionBusy && root.client.status.connected === true
                onClicked: root.save()
            }
        }
    }
}
