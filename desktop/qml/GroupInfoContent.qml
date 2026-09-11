import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Column {
    id: root
    property var groupInfo: ({})
    property var chat: ({})
    property bool loading: false
    property bool busy: false
    property string error: ""
    readonly property var members: groupInfo.participants || []
    signal actionRequested(string action, var member)
    signal avatarRequested(string jid)

    GroupInfoRow { objectName: "groupStarredRow"; text: qsTr("Starred messages"); iconSource: Qt.resolvedUrl("icons/star.svg"); onClicked: root.actionRequested("starred", {}) }
    GroupInfoRow { objectName: "groupNotificationsRow"; text: qsTr("Notification settings"); subtitle: Number(root.chat.muted_until || 0) > Date.now() ? qsTr("Muted") : qsTr("On"); iconSource: Qt.resolvedUrl("icons/bell.svg"); onClicked: root.actionRequested("notifications", {}) }
    GroupInfoRow { text: qsTr("Encryption"); subtitle: qsTr("Messages are end-to-end encrypted. Click to learn more."); iconSource: Qt.resolvedUrl("icons/lock.svg"); onClicked: root.actionRequested("encryption", {}) }
    GroupInfoRow {
        text: qsTr("Disappearing messages")
        readonly property int seconds: Number(root.chat.disappearing_seconds || 0)
        subtitle: seconds === 0 ? qsTr("Off") : seconds === 86400 ? qsTr("24 hours") : seconds === 604800 ? qsTr("7 days") : seconds === 7776000 ? qsTr("90 days") : qsTr("On")
        iconSource: Qt.resolvedUrl("icons/status.svg")
        onClicked: root.actionRequested("disappearing", {})
    }
    GroupInfoRow { text: qsTr("Advanced chat privacy"); subtitle: qsTr("Manage in WhatsApp; not supported here yet"); iconSource: Qt.resolvedUrl("icons/shield.svg"); onClicked: root.actionRequested("privacy", {}) }
    GroupInfoRow {
        objectName: "groupPermissionsRow"
        text: qsTr("Group permissions")
        subtitle: root.groupInfo.can_edit_permissions ? qsTr("Manage what members can do") : qsTr("Only admins can change these settings")
        iconSource: Qt.resolvedUrl("icons/settings.svg")
        enabled: Boolean(root.groupInfo.is_member) && Boolean(root.groupInfo.permissions) && !root.loading && !root.busy
        onClicked: {
            forceActiveFocus(Qt.MouseFocusReason)
            root.actionRequested("permissions", {})
        }
    }
    Rectangle { width: parent.width - 40; x: 20; height: 1; color: Theme.border }
    GroupInfoRow {
        objectName: "groupJoinRequestsRow"
        text: qsTr("Pending join requests")
        subtitle: qsTr("Review people asking to join")
        iconSource: Qt.resolvedUrl("icons/user-add.svg")
        visible: Boolean(root.groupInfo.can_edit_permissions)
        enabled: !root.loading && !root.busy
        onClicked: {
            forceActiveFocus(Qt.MouseFocusReason)
            root.actionRequested("join_requests", {})
        }
    }
    GroupInfoRow {
        objectName: "groupSimilarRow"
        text: qsTr("Create similar group")
        subtitle: qsTr("Start with the same members, then add or remove people.")
        iconSource: Qt.resolvedUrl("icons/group-add.svg")
        enabled: root.members.length > 0 && !root.busy
        onClicked: root.actionRequested("similar", {})
    }

    Column {
        objectName: "groupMembersSection"
        width: parent.width
        RowLayout {
            width: parent.width
            height: 56
            Label {
                Layout.leftMargin: 24
                Layout.fillWidth: true
                text: root.loading && root.members.length === 0 ? qsTr("Loading members…")
                    : root.groupInfo.jid ? qsTr("%1 members").arg(root.groupInfo.participant_count || root.members.length) : qsTr("Members")
                color: Theme.textMuted
                font.pixelSize: 14
                font.weight: Font.Medium
            }
            ThemedToolButton {
                objectName: "groupMemberSearchButton"
                Layout.rightMargin: 16
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/search.svg")
                Accessible.name: qsTr("Search group members")
                enabled: root.members.length > 0
                onClicked: root.actionRequested("members", {})
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
        }
        Label {
            width: parent.width - 48
            x: 24
            visible: root.error !== ""
            text: root.error
            color: Theme.danger
            font.pixelSize: 13
            wrapMode: Text.Wrap
        }
        GroupInfoRow { visible: root.error !== ""; text: qsTr("Retry loading members"); iconSource: Qt.resolvedUrl("icons/reconnect.svg"); enabled: !root.loading; onClicked: root.actionRequested("retry", {}) }
        GroupInfoRow {
            objectName: "groupAddMembersRow"
            text: qsTr("Add member")
            subtitle: !root.groupInfo.can_add && root.groupInfo.jid ? qsTr("Only permitted members can add people") : ""
            enabled: Boolean(root.groupInfo.can_add) && !root.busy
            iconSource: Qt.resolvedUrl("icons/user-add.svg")
            onClicked: root.actionRequested("add", {})
        }
        GroupInfoRow {
            objectName: "groupInviteRow"
            text: qsTr("Invite to group via link")
            enabled: Boolean(root.groupInfo.can_invite) && !root.busy
            iconSource: Qt.resolvedUrl("icons/link.svg")
            onClicked: root.actionRequested("invite", {})
        }
        Repeater {
            // Bound inline construction; the full searchable list is virtualized.
            model: root.members.slice(0, 8)
            GroupMemberRow {
                required property var modelData
                member: modelData
                onClicked: root.actionRequested("member", member)
                onAvatarRequested: jid => root.avatarRequested(jid)
            }
        }
        GroupInfoRow { visible: root.members.length > 8; text: qsTr("View all %1 members").arg(root.members.length); iconSource: Qt.resolvedUrl("icons/communities.svg"); onClicked: root.actionRequested("members", {}) }
        GroupInfoRow { text: qsTr("Group member tags"); subtitle: qsTr("View and edit tags in WhatsApp"); iconSource: Qt.resolvedUrl("icons/sort.svg"); onClicked: root.actionRequested("tags", {}) }
    }
    Rectangle { width: parent.width - 40; x: 20; height: 1; color: Theme.border }
    GroupInfoRow { text: root.chat.favorite ? qsTr("Remove from Favorites") : qsTr("Add to Favorites"); iconSource: Qt.resolvedUrl("icons/heart.svg"); onClicked: root.actionRequested("favorite", {}) }
    GroupInfoRow { objectName: "groupAddToListRow"; text: qsTr("Add to list"); iconSource: Qt.resolvedUrl("icons/sort.svg"); onClicked: root.actionRequested("list", {}) }
    GroupInfoRow { text: root.chat.archived ? qsTr("Restore from archive") : qsTr("Archive chat"); iconSource: Qt.resolvedUrl("icons/archive.svg"); onClicked: root.actionRequested("archive", {}) }
    GroupInfoRow { text: qsTr("Export chat"); iconSource: Qt.resolvedUrl("icons/download.svg"); onClicked: root.actionRequested("export", {}) }
    GroupInfoRow { text: qsTr("Clear chat"); iconSource: Qt.resolvedUrl("icons/block.svg"); destructive: true; onClicked: root.actionRequested("clear", {}) }
    GroupInfoRow { objectName: "groupExitRow"; text: qsTr("Exit group"); subtitle: qsTr("Keep local history on this computer"); iconSource: Qt.resolvedUrl("icons/logout.svg"); destructive: true; enabled: Boolean(root.groupInfo.is_member) && !root.busy; onClicked: root.actionRequested("leave", {}) }
    GroupInfoRow { visible: Boolean(root.groupInfo.jid) && !root.groupInfo.is_member; text: qsTr("Delete chat"); iconSource: Qt.resolvedUrl("icons/delete.svg"); destructive: true; onClicked: root.actionRequested("delete", {}) }
    GroupInfoRow { text: qsTr("Report group"); subtitle: qsTr("Use WhatsApp to submit a report"); iconSource: Qt.resolvedUrl("icons/flag.svg"); destructive: true; onClicked: root.actionRequested("report", {}) }
    Label {
        width: parent.width - 48
        x: 24
        topPadding: 16
        bottomPadding: 24
        visible: Number(root.groupInfo.created_at || 0) > 0
        text: qsTr("Group created %1 by %2").arg(new Date(Number(root.groupInfo.created_at || 0)).toLocaleString(Qt.locale(), Locale.ShortFormat))
            .arg(root.groupInfo.creator && root.groupInfo.creator.name || qsTr("an unknown member"))
        color: Theme.textMuted
        font.pixelSize: 13
        wrapMode: Text.Wrap
    }
}
