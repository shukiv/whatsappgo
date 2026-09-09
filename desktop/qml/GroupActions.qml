import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Item {
    id: root
    required property var client
    property string jid: ""
    property string mode: ""
    property var member: ({})
    property string query: ""
    property string groupName: ""
    property var selected: []
    property int muteSeconds: 0
    property bool muteChanged: false
    property string feedback: ""
    readonly property var members: client.groupInfo.participants || []
    signal notice(string message)

    function show(action, target) {
        jid = String(client.selectedChat.jid || "")
        mode = action
        member = target || ({})
        query = ""
        feedback = ""
        selected = action === "similar" ? members.filter(p => !p.is_self).map(p => p.jid) : []
        groupName = action === "similar" ? String(client.selectedChat.title || "").slice(0,25) : ""
        muteSeconds = Number(client.selectedChat.muted_until || 0) > Date.now() ? 0 : -1
        muteChanged = false
        dialog.open()
        if (action === "invite") client.requestGroupInviteLink(jid, false)
        if (action === "list") client.refreshChatLabels()
    }
    function toggle(jid) {
        const next = selected.slice()
        const index = next.indexOf(jid)
        if (index < 0) next.push(jid)
        else next.splice(index,1)
        selected = next
    }
    function candidates() {
        const aliases = {}
        for (const p of members) {
            aliases[p.jid] = true
            for (const a of p.aliases || []) aliases[a] = true
        }
        const rows = mode === "similar" ? members.filter(p => !p.is_self).map(p => ({jid:p.jid,name:p.name,phone:p.phone,avatar_path:p.avatar_path})) : []
        for (const chat of client.chats) {
            const id = String(chat.jid || "")
            if (!aliases[id] && !chat.is_group && (id.endsWith("@lid") || id.endsWith("@s.whatsapp.net"))) {
                aliases[id] = true
                rows.push({jid:id,name:chat.title,avatar_path:chat.avatar_path})
            }
        }
        return rows.filter(p => (String(p.name || "") + " " + String(p.phone || "")).toLowerCase().indexOf(query.trim().toLowerCase()) >= 0)
    }
    function confirm(action) {
        mode = action
        feedback = ""
    }
    Connections {
        target: root.client
        function onSelectedChatChanged() {
            if (root.jid && root.jid !== String(root.client.selectedChat.jid || "")) dialog.close()
        }
        function onProfileChanged() { dialog.close() }
        function onGroupActionFinished(jid, action, success) {
            if (jid !== root.jid) return
            if (!success) {
                root.feedback = root.client.groupInfoError
                root.notice(root.feedback)
            } else if (action !== "invite") {
                root.notice(action === "leave" ? qsTr("You left the group. Local history was kept.") : qsTr("Group members updated"))
            }
        }
    }

    WhatsAppDialog {
        id: dialog
        objectName: "groupActionsDialog"
        preferredWidth: 520
        preferredHeight: ["members", "add", "similar", "list"].indexOf(root.mode) >= 0 ? 570 : 0
        title: ({members:qsTr("Group members"), member:root.member.is_self ? qsTr("You") : root.member.name,
            add:qsTr("Add members"), similar:qsTr("Create similar group"), invite:qsTr("Invite via link"),
            reset:qsTr("Reset invite link?"), remove:qsTr("Remove member?"), promote:qsTr("Make group admin?"),
            demote:qsTr("Dismiss as admin?"), leave:qsTr("Exit this group?"), list:qsTr("Add to list"),
            notifications:qsTr("Notification settings"), encryption:qsTr("Encryption"), privacy:qsTr("Advanced chat privacy"),
            tags:qsTr("Group member tags"), report:qsTr("Report group")})[root.mode] || qsTr("Group info")
        showAccept: ["add", "similar", "reset", "remove", "promote", "demote", "leave", "notifications"].indexOf(root.mode) >= 0
        cancelText: showAccept ? qsTr("Cancel") : qsTr("Close")
        acceptText: root.mode === "add" ? qsTr("Add") : root.mode === "similar" ? qsTr("Create") : root.mode === "leave" ? qsTr("Exit group") : root.mode === "reset" ? qsTr("Reset link") : root.mode === "notifications" ? qsTr("Save") : qsTr("Confirm")
        destructive: ["remove", "demote", "leave", "reset"].indexOf(root.mode) >= 0
        acceptEnabled: !root.client.groupActionBusy && (root.mode !== "add" && root.mode !== "similar" || root.selected.length > 0 && root.selected.length <= 100)
            && (root.mode !== "similar" || root.groupName.trim() !== "")
        onAccepted: {
            if (root.jid !== String(root.client.selectedChat.jid || "")) return
            if (root.mode === "add") root.client.changeGroupMembers(root.jid, "add", root.selected)
            else if (root.mode === "similar") root.client.createGroup(root.groupName.trim(), root.selected)
            else if (root.mode === "leave") root.client.leaveGroup(root.jid)
            else if (root.mode === "reset") root.client.requestGroupInviteLink(root.jid, true)
            else if (root.mode === "notifications") {
                if (root.muteChanged) root.client.setChatMuted(root.jid, root.muteSeconds !== -1, Math.max(0,root.muteSeconds))
            }
            else root.client.changeGroupMembers(root.jid, root.mode, [root.member.jid])
        }
        Loader {
            Layout.fillWidth: true
            Layout.fillHeight: dialog.preferredHeight > 0
            sourceComponent: ["members", "add", "similar"].indexOf(root.mode) >= 0 ? memberListPage
                : root.mode === "member" ? memberPage : root.mode === "invite" ? invitePage
                : root.mode === "list" ? listsPage : root.mode === "notifications" ? notificationsPage : explanationPage
        }
        Label { Layout.fillWidth: true; visible: root.feedback !== ""; text: root.feedback; color: Theme.danger; wrapMode: Text.Wrap }
    }

    Component {
        id: memberListPage
        ColumnLayout {
            id: peoplePage
            readonly property var rows: root.mode === "members"
                ? root.members.filter(p => (String(p.name || "") + " " + String(p.phone || "")).toLowerCase().indexOf(root.query.trim().toLowerCase()) >= 0)
                : root.candidates()
            // Keep delegate identities and scroll position when names/photos or
            // permissions refresh. Replacing an array model resets a ListView.
            function syncRows() {
                if (!visiblePeople) return
                for (let i = 0; i < rows.length; ++i) {
                    const person = rows[i]
                    const entry = {jid: String(person.jid), memberName: String(person.name || ""),
                        phone: String(person.phone || ""), avatarPath: String(person.avatar_path || ""),
                        isSelf: Boolean(person.is_self), isAdmin: Boolean(person.is_admin), isOwner: Boolean(person.is_owner)}
                    if (i >= visiblePeople.count) visiblePeople.append(entry)
                    else {
                        if (visiblePeople.get(i).jid !== person.jid) {
                            let existing = i + 1
                            while (existing < visiblePeople.count && visiblePeople.get(existing).jid !== person.jid) ++existing
                            if (existing < visiblePeople.count) visiblePeople.move(existing, i, 1)
                            else visiblePeople.insert(i, entry)
                        }
                        visiblePeople.set(i, entry)
                    }
                }
                if (visiblePeople.count > rows.length) visiblePeople.remove(rows.length, visiblePeople.count - rows.length)
            }
            onRowsChanged: syncRows()
            Component.onCompleted: syncRows()
            ListModel { id: visiblePeople }
            spacing: 8
            DialogTextField {
                Layout.fillWidth: true
                visible: root.mode === "similar"
                placeholderText: qsTr("Group name")
                maximumLength: 25
                text: root.groupName
                onTextEdited: root.groupName = text
            }
            DialogTextField {
                id: filter
                objectName: "groupMembersSearchField"
                Layout.fillWidth: true
                search: true
                placeholderText: root.mode === "members" ? qsTr("Search members") : qsTr("Search available contacts")
                onTextEdited: root.query = text
                Component.onCompleted: forceActiveFocus()
            }
            ListView {
                id: memberList
                objectName: "groupMemberList"
                Layout.fillWidth: true
                Layout.fillHeight: true
                clip: true
                reuseItems: true
                model: visiblePeople
                delegate: GroupMemberRow {
                    required property string jid
                    required property string memberName
                    required property string phone
                    required property string avatarPath
                    required property bool isSelf
                    required property bool isAdmin
                    required property bool isOwner
                    width: memberList.width
                    member: ({jid: jid, name: memberName, phone: phone, avatar_path: avatarPath,
                        is_self: isSelf, is_admin: isAdmin, is_owner: isOwner})
                    highlighted: root.mode !== "members" && root.selected.indexOf(member.jid) >= 0
                    Accessible.checked: highlighted
                    background: Rectangle { radius: 8; color: parent.highlighted ? Theme.primaryContainer : parent.hovered ? Theme.hoverRow : "transparent"; border.width: parent.activeFocus ? 1 : 0; border.color: Theme.primary }
                    onClicked: {
                        if (root.mode === "members") { root.member = member; root.mode = "member" }
                        else root.toggle(member.jid)
                    }
                    onAvatarRequested: jid => root.client.refreshChatAvatar(jid)
                }
                ScrollBar.vertical: OverlayScrollBar {}
            }
            Label { visible: memberList.count === 0; Layout.fillWidth: true; text: qsTr("No matching people"); color: Theme.textMuted; wrapMode: Text.Wrap }
            Label { visible: root.mode !== "members"; Layout.fillWidth: true; text: qsTr("%1 selected (maximum 100). Review before confirming.").arg(root.selected.length); color: Theme.textMuted; wrapMode: Text.Wrap }
        }
    }
    Component {
        id: memberPage
        Column {
            spacing: 4
            CopyableInfoText {
                objectName: "groupMemberName"
                width: parent.width
                text: String(root.member.name || "")
                copyLabel: qsTr("Copy name")
                onCopyRequested: value => root.client.copyText(value)
            }
            CopyableInfoText {
                objectName: "groupMemberPhone"
                width: parent.width
                visible: Boolean(root.member.phone)
                text: root.member.phone ? "+" + root.member.phone : ""
                color: Theme.textMuted
                font.pixelSize: 14
                copyLabel: qsTr("Copy phone number")
                onCopyRequested: value => root.client.copyText(value)
            }
            GroupInfoRow {
                text: qsTr("Message %1").arg(root.member.name || "")
                iconSource: Qt.resolvedUrl("icons/chats.svg")
                visible: !root.member.is_self
                onClicked: { dialog.close(); root.client.openChat(root.member.jid, root.member.name) }
            }
            GroupInfoRow {
                text: root.member.is_admin ? qsTr("Dismiss as admin") : qsTr("Make group admin")
                iconSource: Qt.resolvedUrl("icons/profile.svg")
                visible: Boolean(root.client.groupInfo.can_manage) && !root.member.is_self && !root.member.is_owner
                enabled: !root.client.groupActionBusy
                onClicked: root.confirm(root.member.is_admin ? "demote" : "promote")
            }
            GroupInfoRow {
                text: qsTr("Remove from group")
                iconSource: Qt.resolvedUrl("icons/delete.svg")
                destructive: true
                visible: Boolean(root.client.groupInfo.can_manage) && !root.member.is_self && !root.member.is_owner
                enabled: !root.client.groupActionBusy
                onClicked: root.confirm("remove")
            }
            GroupInfoRow { text: qsTr("Member tags"); subtitle: qsTr("Manage in WhatsApp"); iconSource: Qt.resolvedUrl("icons/sort.svg"); onClicked: root.mode = "tags" }
        }
    }
    Component {
        id: invitePage
        ColumnLayout {
            spacing: 12
            Label { Layout.fillWidth: true; text: qsTr("Anyone with this link can request to join or join the group. Share it only with people you trust."); color: Theme.textMuted; wrapMode: Text.Wrap }
            BusyIndicator { running: root.client.groupActionBusy; visible: running; Layout.alignment: Qt.AlignHCenter }
            DialogTextField { Layout.fillWidth: true; readOnly: true; text: root.client.groupInviteLink; placeholderText: qsTr("Invite link"); selectByMouse: true }
            GroupInfoRow { Layout.fillWidth: true; text: qsTr("Copy link"); iconSource: Qt.resolvedUrl("icons/copy.svg"); enabled: root.client.groupInviteLink !== "" && !root.client.groupActionBusy; onClicked: { root.client.copyText(root.client.groupInviteLink); root.notice(qsTr("Invite link copied")) } }
            GroupInfoRow { Layout.fillWidth: true; text: qsTr("Reset link…"); iconSource: Qt.resolvedUrl("icons/reconnect.svg"); visible: Boolean(root.client.groupInfo.can_manage); enabled: !root.client.groupActionBusy; onClicked: root.confirm("reset") }
        }
    }
    Component {
        id: listsPage
        ListView {
            clip: true
            model: root.client.chatLabels
            delegate: ItemDelegate {
                required property var modelData
                width: ListView.view.width
                text: modelData.name || ""
                onClicked: { root.client.setChatLabeled(root.jid, String(modelData.id), true); dialog.close() }
            }
            Label { anchors.centerIn: parent; width: parent.width; visible: root.client.chatLabels.length === 0; text: qsTr("No lists yet. Create one with the + button beside the chat filters."); color: Theme.textMuted; wrapMode: Text.Wrap }
            ScrollBar.vertical: OverlayScrollBar {}
        }
    }
    Component {
        id: notificationsPage
        Column {
            spacing: 8
            Repeater {
                model: [{label:qsTr("Notifications on"),seconds:-1},{label:qsTr("Mute for 8 hours"),seconds:28800},{label:qsTr("Mute for 1 week"),seconds:604800},{label:qsTr("Always mute"),seconds:0}]
                DialogRadioButton { required property var modelData; text: modelData.label; checked: root.muteSeconds === modelData.seconds; onClicked: { root.muteSeconds = modelData.seconds; root.muteChanged = true } }
            }
        }
    }
    Component {
        id: explanationPage
        Label {
            width: parent ? parent.width : 400
            text: root.mode === "leave" ? qsTr("You will no longer receive new messages from this group. Your local history and media will be kept. This does not delete the chat.")
                : root.mode === "reset" ? qsTr("The current invite link will stop working. A new link will be created. Open Invite via link again to copy it.")
                : ["remove", "promote", "demote"].indexOf(root.mode) >= 0 ? qsTr("This changes %1’s membership or admin role in %2 for everyone. Continue?").arg(root.member.name).arg(root.client.selectedChat.title)
                : root.mode === "encryption" ? qsTr("WhatsApp messages are end-to-end encrypted in transit. WhatsAppGo stores decrypted local history on this computer. Group security-code verification is not available here.")
                : root.mode === "privacy" ? qsTr("Advanced chat privacy is not exposed by the linked-device library used by WhatsAppGo. Open this group in the official WhatsApp app to view or change it. Its current state is not known here.")
                : root.mode === "tags" ? qsTr("Group member tags are not exposed by the linked-device library used by WhatsAppGo. View or edit them in the official WhatsApp app.")
                : qsTr("Group reporting is not implemented in WhatsAppGo. Open this group in the official WhatsApp app and choose Report group. No report has been submitted here.")
            color: Theme.text
            font.pixelSize: 14
            wrapMode: Text.Wrap
        }
    }
}
