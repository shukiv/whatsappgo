import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "groupJoinRequestsDialog"
    required property var client
    property string targetChat: ""
    property string targetProfile: ""
    property var requests: []
    property var selected: null
    property string action: ""
    property string query: ""
    property string error: ""
    property string feedback: ""
    property string loadToken: ""
    property string reviewToken: ""
    property int serial: 0
    property bool loading: false
    property bool saving: false
    property bool needsReload: false
    property var returnFocus: null
    readonly property bool canReview: targetProfile === client.profile
        && targetChat !== "" && targetChat === String(client.selectedChat.jid || "")
        && Boolean(client.groupInfo.can_edit_permissions) && Boolean(client.groupInfo.is_member)
    readonly property var filtered: requests.filter(row =>
        (String(row.member.name || "") + " " + String(row.member.phone || "")).toLowerCase().includes(query.trim().toLowerCase()))
    signal notice(string message)

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(560, parent ? parent.width - 32 : 560)
    height: Math.min(selected ? 340 : 600, parent ? parent.height - 32 : 600)
    padding: 20
    modal: true
    focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    function showForGroup() {
        const jid = String(client.selectedChat.jid || "")
        if (!jid.endsWith("@g.us") || client.groupInfo.jid !== jid || !client.groupInfo.can_edit_permissions
                || !client.groupInfo.is_member || client.groupActionBusy || client.groupInfoLoading) return
        targetChat = jid
        targetProfile = client.profile
        requests = []
        selected = null
        action = query = error = feedback = ""
        saving = needsReload = false
        returnFocus = parent && parent.Window.window ? parent.Window.window.activeFocusItem : null
        open()
        reload()
    }
    function reload() {
        if (!canReview || saving || loading || client.status.connected !== true) return
        selected = null
        action = error = ""
        requests = []
        needsReload = false
        loading = true
        loadToken = "group-requests:" + (++serial)
        client.loadGroupJoinRequests(targetChat, loadToken, targetProfile)
    }
    function choose(row, decision) {
        if (!canReview || loading || saving || needsReload || client.groupActionBusy
                || client.status.connected !== true || Number(row.requested_at) <= 0) return
        selected = row
        action = decision
        error = feedback = ""
        Qt.callLater(function() { cancelButton.forceActiveFocus(Qt.TabFocusReason) })
    }
    function back() {
        if (saving || !selected) { close(); return }
        selected = null
        action = ""
        Qt.callLater(function() { search.forceActiveFocus(Qt.TabFocusReason) })
    }
    function submit() {
        if (!confirmButton.enabled) return
        saving = true
        error = ""
        reviewToken = "group-review:" + (++serial)
        client.reviewGroupJoinRequest(targetChat, String(selected.member.jid), Number(selected.requested_at), action, reviewToken, targetProfile)
    }
    onOpened: search.forceActiveFocus(Qt.TabFocusReason)
    onClosed: {
        loadToken = reviewToken = ""
        loading = false
        requests = []
        selected = null
        Qt.callLater(function() {
            if (root.targetProfile === root.client.profile && root.targetChat === String(root.client.selectedChat.jid || "")
                    && returnFocus && returnFocus.visible) returnFocus.forceActiveFocus(Qt.TabFocusReason)
        })
    }
    Shortcut { sequence: "Escape"; enabled: root.visible; autoRepeat: false; onActivated: root.back() }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onSelectedChatChanged() {
            if (root.visible && String(root.client.selectedChat.jid || "") !== root.targetChat) root.close()
        }
        function onGroupInfoChanged() {
            if (root.visible && !root.client.groupInfo.can_edit_permissions) {
                root.requests = []
                if (!root.saving) root.selected = null
                root.needsReload = true
            }
        }
        function onGroupJoinRequestsLoaded(token, requests, error) {
            if (!root.visible || token !== root.loadToken) return
            root.loading = false
            root.error = error
            root.requests = root.canReview && error === "" ? requests : []
        }
        function onGroupJoinRequestReviewed(token, error) {
            if (!root.visible || token !== root.reviewToken) return
            root.saving = false
            root.error = error
            if (error !== "") {
                root.needsReload = true
                root.requests = []
            } else {
                root.feedback = root.action === "approve" ? qsTr("Join request approved") : qsTr("Join request rejected")
                root.notice(root.feedback)
                root.reload()
            }
        }
    }

    contentItem: ColumnLayout {
        spacing: 12
        Label {
            Layout.fillWidth: true
            text: root.selected ? (root.action === "approve" ? qsTr("Approve join request?") : qsTr("Reject join request?")) : qsTr("Pending join requests")
            color: Theme.text; font.pixelSize: 20; font.weight: Font.Medium; wrapMode: Text.Wrap
        }
        TextField {
            id: search
            objectName: "groupRequestsSearch"
            Layout.fillWidth: true
            implicitHeight: 44
            leftPadding: 12; rightPadding: 12
            visible: !root.selected
            placeholderText: qsTr("Search name or number")
            Accessible.name: qsTr("Search pending join requests")
            text: root.query
            color: Theme.text
            placeholderTextColor: Theme.textMuted
            onTextEdited: root.query = text
            background: Rectangle { color: Theme.input; radius: 22; border.color: search.activeFocus ? Theme.primary : Theme.border }
        }
        Label {
            visible: !root.selected && !root.loading && !root.error && root.canReview && !root.needsReload
            text: qsTr("%1 pending requests").arg(root.requests.length)
            color: Theme.textMuted
        }
        ListView {
            id: list
            objectName: "groupRequestsList"
            Layout.fillWidth: true; Layout.fillHeight: true
            visible: !root.selected
            clip: true; spacing: 8
            model: root.filtered
            ScrollBar.vertical: OverlayScrollBar {}
            delegate: Rectangle {
                required property var modelData
                objectName: "groupRequest-" + modelData.member.jid
                width: list.width
                height: content.implicitHeight + 24
                color: Theme.surfaceMuted; radius: 10
                ColumnLayout {
                    id: content
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: 12 }
                    spacing: 6
                    Label { Layout.fillWidth: true; text: modelData.member.name || qsTr("WhatsApp member"); textFormat: Text.PlainText; color: Theme.text; font.weight: Font.Medium; wrapMode: Text.Wrap }
                    Label { Layout.fillWidth: true; visible: modelData.member.phone !== undefined && modelData.member.phone !== ""; text: "+" + (modelData.member.phone || ""); textFormat: Text.PlainText; color: Theme.textMuted }
                    Label {
                        Layout.fillWidth: true; color: Theme.textMuted; wrapMode: Text.Wrap
                        text: Number(modelData.requested_at) > 0 ? qsTr("Requested %1").arg(new Date(Number(modelData.requested_at)).toLocaleString(Qt.locale(), Locale.ShortFormat)) : qsTr("Request time unavailable; refresh before reviewing")
                    }
                    RowLayout {
                        Item { Layout.fillWidth: true }
                        ExpressionButton {
                            objectName: "groupRequestReject"
                            text: qsTr("Reject")
                            enabled: root.canReview && !root.needsReload && !root.loading && !root.client.groupActionBusy && root.client.status.connected === true && Number(modelData.requested_at) > 0
                            Accessible.name: qsTr("Review rejection for %1").arg(modelData.member.name || qsTr("WhatsApp member"))
                            onClicked: root.choose(modelData, "reject")
                        }
                        ExpressionButton {
                            objectName: "groupRequestApprove"
                            text: qsTr("Approve")
                            enabled: root.canReview && !root.needsReload && !root.loading && !root.client.groupActionBusy && root.client.status.connected === true && Number(modelData.requested_at) > 0
                            Accessible.name: qsTr("Review approval for %1").arg(modelData.member.name || qsTr("WhatsApp member"))
                            onClicked: root.choose(modelData, "approve")
                        }
                    }
                }
            }
            Label {
                anchors.centerIn: parent; width: parent.width - 16
                horizontalAlignment: Text.AlignHCenter; wrapMode: Text.Wrap; color: Theme.textMuted
                visible: list.count === 0 && !root.error && root.canReview
                text: root.loading ? qsTr("Loading join requests…") : root.needsReload ? qsTr("Refresh requests before making another decision.") : root.query.trim() ? qsTr("No matching requests") : qsTr("No pending join requests")
            }
        }
        ScrollView {
            id: confirmation
            Layout.fillWidth: true; Layout.fillHeight: true
            visible: Boolean(root.selected); clip: true
            contentWidth: availableWidth
            Label {
                width: confirmation.availableWidth
                text: root.selected ? (root.action === "approve"
                    ? qsTr("Add %1 to this group? They will become a group member.")
                    : qsTr("Reject %1’s request to join this group? This dismisses their current request, but does not block them.")).arg(root.selected.member.name || qsTr("WhatsApp member")) : ""
                textFormat: Text.PlainText; color: Theme.text; wrapMode: Text.Wrap
            }
        }
        Label {
            Layout.fillWidth: true
            text: root.error || (!root.canReview ? qsTr("Only current admins of supported groups can review requests.")
                : root.saving ? qsTr("Submitting… Closing does not cancel this decision.")
                : root.client.status.connected !== true ? qsTr("Connect to WhatsApp to review requests.") : root.feedback)
            visible: text !== ""; textFormat: Text.PlainText; wrapMode: Text.Wrap
            color: root.error ? Theme.danger : Theme.textMuted
        }
        RowLayout {
            Layout.fillWidth: true
            ExpressionButton {
                objectName: "groupRequestsRefresh"
                text: qsTr("Refresh requests")
                enabled: root.canReview && !root.loading && !root.saving && root.client.status.connected === true
                onClicked: root.reload()
            }
            Item { Layout.fillWidth: true }
            ExpressionButton {
                id: cancelButton
                objectName: "groupRequestsCancel"
                text: !root.selected || root.saving ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.back()
            }
            ExpressionButton {
                id: confirmButton
                objectName: "groupRequestsConfirm"
                text: root.action === "approve" ? qsTr("Approve") : qsTr("Reject")
                visible: Boolean(root.selected)
                primary: true
                contentItem: Label {
                    text: confirmButton.text; font: confirmButton.font
                    color: root.action === "reject" ? Theme.danger : Theme.primaryText
                    horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 22
                    color: root.action === "reject" ? (confirmButton.hovered ? Theme.hoverRow : "transparent") : Theme.primary
                    border.color: root.action === "reject" ? Theme.danger : Theme.primary
                    border.width: confirmButton.activeFocus ? 2 : 1
                }
                enabled: root.canReview && Boolean(root.selected) && !root.loading && !root.saving && !root.needsReload
                    && !root.client.groupActionBusy && root.client.status.connected === true
                onClicked: root.submit()
            }
        }
    }
}
