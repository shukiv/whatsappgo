import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "newGroupDialog"
    required property var client
    property bool reviewing: false
    property bool sending: false
    property bool loading: false
    property string targetProfile: ""
    property string requestToken: ""
    property string searchToken: ""
    property int serial: 0
    property string error: ""
    property string loadError: ""
    property var contacts: []
    property var selected: []
    property var returnFocus: null
    readonly property var selectedJids: selected.map(p => p.jid)
    readonly property string subject: nameField.text.trim()
    readonly property int nameLength: (subject.match(/[\uD800-\uDBFF][\uDC00-\uDFFF]|[\s\S]/g) || []).length
    readonly property bool validName: nameLength > 0 && nameLength <= 100
        && !/[\u0000-\u001f\u007f-\u009f\u2028\u2029]/.test(subject)
    readonly property bool sameAccount: targetProfile === client.profile
    readonly property var candidates: {
        const seen = {}
        const rows = []
        const own = String(client.status.user_jid || "").replace(/:\d+@/, "@")
        const needle = filterField.text.trim().toLowerCase()
        // Local rows stay available while the bounded address-book search runs.
        for (const p of contacts.concat(client.chats, client.archivedChats)) {
            const jid = String(p.jid || "").replace(/:\d+@/, "@")
            if (p.is_group || !/^\d+@(s\.whatsapp\.net|lid)$/.test(jid) || jid === own || seen[jid]) continue
            const name = String(p.name || p.title || p.phone || jid)
            const phone = String(p.phone || (jid.endsWith("@s.whatsapp.net") ? "+" + jid.split("@")[0] : ""))
            if ((name + " " + phone + " " + jid).toLowerCase().indexOf(needle) < 0) continue
            seen[jid] = true
            rows.push({jid: jid, name: name, phone: phone, avatar_path: String(p.avatar_path || "")})
        }
        return rows
    }
    signal created(string jid, string title)

    onSendingChanged: {
        // Disabling the focused Create button must not strand keyboard focus
        // outside the modal while the network request is pending.
        if (sending) Qt.callLater(function() { if (root.visible) backAction.forceActiveFocus(Qt.TabFocusReason) })
    }

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(520, parent ? parent.width - 32 : 520)
    height: Math.min(600, parent ? parent.height - 32 : 600)
    padding: 20
    modal: true
    focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    onAboutToShow: {
        returnFocus = parent && parent.Window.window ? parent.Window.window.activeFocusItem : null
        targetProfile = client.profile
        reviewing = false
        sending = false
        selected = []
        contacts = []
        error = ""
        loadError = ""
        requestToken = ""
        nameField.text = ""
        filterField.text = ""
    }
    onOpened: { filterField.forceActiveFocus(Qt.TabFocusReason); searchNow() }
    onClosed: {
        searchDelay.stop()
        searchToken = ""
        requestToken = ""
        Qt.callLater(function() { if (returnFocus && returnFocus.visible) returnFocus.forceActiveFocus(Qt.TabFocusReason) })
    }
    function searchNow() {
        searchDelay.stop()
        if (!visible || !sameAccount) return
        loading = true
        loadError = ""
        searchToken = "group-contacts:" + (++serial)
        client.searchGroupContacts(filterField.text.trim(), searchToken)
    }
    function toggleMember(jid) {
        if (sending) return
        const next = selected.slice()
        const index = next.findIndex(p => p.jid === jid)
        if (index >= 0) next.splice(index, 1)
        else {
            const person = candidates.find(p => p.jid === jid)
            if (!person || next.length >= 1023) return
            next.push(person)
        }
        selected = next
    }
    function back() {
        if (reviewing && !sending) {
            reviewing = false
            filterField.forceActiveFocus(Qt.TabFocusReason)
        } else close()
    }
    Shortcut { sequence: "Escape"; enabled: root.visible; autoRepeat: false; onActivated: root.back() }
    Timer { id: searchDelay; interval: 180; onTriggered: root.searchNow() }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onGroupContactsReady(token, contacts, error) {
            if (!root.visible || token !== root.searchToken || !root.sameAccount) return
            root.loading = false
            root.contacts = contacts
            root.loadError = error
        }
        function onGroupCreationFinished(token, chat, error) {
            if (!root.visible || token !== root.requestToken || !root.sameAccount) return
            root.sending = false
            root.error = error
            if (!error) {
                root.close()
                root.created(chat.jid, chat.title)
            }
        }
    }

    contentItem: ColumnLayout {
        spacing: 12
        Label {
            Layout.fillWidth: true
            text: root.reviewing ? qsTr("New group") : qsTr("Add group members")
            color: Theme.text
            font.pixelSize: 20
            font.weight: Font.Medium
        }
        Label {
            Layout.fillWidth: true
            text: root.reviewing ? qsTr("Step 2 of 2 · Name and create your group") : qsTr("Step 1 of 2 · Select people to add")
            color: Theme.textMuted
            wrapMode: Text.Wrap
        }
        Label {
            visible: root.reviewing
            text: qsTr("Group name")
            color: Theme.text
        }
        DialogTextField {
            id: nameField
            objectName: "newGroupNameField"
            Layout.fillWidth: true
            visible: root.reviewing
            enabled: !root.sending
            placeholderText: qsTr("Group name")
            Accessible.name: qsTr("Group name")
        }
        Label {
            Layout.fillWidth: true
            visible: root.reviewing
            text: qsTr("%1 / 100 characters").arg(root.nameLength)
            color: root.nameLength <= 100 ? Theme.textMuted : Theme.danger
            horizontalAlignment: Text.AlignRight
        }
        DialogTextField {
            id: filterField
            objectName: "newGroupFilter"
            Layout.fillWidth: true
            visible: !root.reviewing
            search: true
            placeholderText: qsTr("Search name or number")
            Accessible.name: qsTr("Search group members")
            onTextChanged: { root.searchToken = ""; if (root.visible) searchDelay.restart() }
        }
        Label {
            objectName: "newGroupSummaryLabel"
            Layout.fillWidth: true
            text: qsTr("%1 selected").arg(root.selected.length)
            color: Theme.textMuted
        }
        ListView {
            Layout.fillWidth: true
            Layout.preferredHeight: 40
            visible: root.selected.length > 0
            orientation: ListView.Horizontal
            spacing: 6
            clip: true
            model: root.selected
            delegate: ExpressionButton {
                required property var modelData
                width: Math.min(180, implicitWidth)
                height: 36
                text: modelData.name
                enabled: !root.sending
                Accessible.name: qsTr("Remove %1 from selected members").arg(modelData.name)
                contentItem: Label {
                    text: parent.text + " ×"
                    elide: Text.ElideMiddle
                    color: Theme.text
                    verticalAlignment: Text.AlignVCenter
                }
                onClicked: root.toggleMember(modelData.jid)
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
            }
        }
        ListView {
            id: memberList
            objectName: "newGroupMemberList"
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: !root.reviewing
            clip: true
            reuseItems: true
            model: root.candidates
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: OverlayScrollBar {}
            delegate: ItemDelegate {
                required property var modelData
                objectName: "newGroupMember_" + modelData.jid
                width: memberList.width
                height: 64
                readonly property bool selected: root.selectedJids.indexOf(modelData.jid) >= 0
                Accessible.name: modelData.name
                Accessible.role: Accessible.CheckBox
                Accessible.checkable: true
                Accessible.checked: selected
                onClicked: root.toggleMember(modelData.jid)
                background: Rectangle {
                    color: parent.selected ? Theme.selectedRow : parent.hovered ? Theme.hoverRow : "transparent"
                    border.color: Theme.primary
                    border.width: parent.activeFocus ? 2 : 0
                    radius: 8
                }
                contentItem: RowLayout {
                    spacing: 10
                    Avatar { Layout.preferredWidth: 40; Layout.preferredHeight: 40; diameter: 40; title: modelData.name; source: Theme.fileUrl(modelData.avatar_path) }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 2
                        Label { Layout.fillWidth: true; text: modelData.name; textFormat: Text.PlainText; color: Theme.text; elide: Text.ElideRight }
                        Label { Layout.fillWidth: true; text: modelData.phone; visible: text !== ""; color: Theme.textMuted; elide: Text.ElideRight; font.pixelSize: 12 }
                    }
                    TintedIcon { Layout.preferredWidth: 20; Layout.preferredHeight: 20; source: Qt.resolvedUrl(parent.parent.selected ? "icons/check.svg" : "icons/plus.svg"); tint: Theme.primary }
                }
            }
            Label {
                anchors.centerIn: parent
                width: parent.width - 24
                visible: memberList.count === 0
                text: root.loading ? qsTr("Loading contacts…") : qsTr("No matching contacts. Try another name or number.")
                color: Theme.textMuted
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
            }
        }
        Label {
            Layout.fillWidth: true
            visible: root.reviewing
            text: qsTr("The selected people will be added when you press Create. WhatsApp may restrict additions based on their privacy settings.")
            color: Theme.textMuted
            wrapMode: Text.Wrap
        }
        Item { Layout.fillHeight: true; visible: root.reviewing }
        Label {
            Layout.fillWidth: true
            visible: root.error !== "" || root.sending || root.client.groupCreationBusy || !root.client.status.connected
            text: root.error || (root.sending || root.client.groupCreationBusy ? qsTr("Creating group…") : qsTr("Reconnect to WhatsApp to create this group."))
            textFormat: Text.PlainText
            color: root.error ? Theme.danger : Theme.textMuted
            wrapMode: Text.Wrap
        }
        RowLayout {
            Layout.fillWidth: true
            visible: !root.reviewing && root.loadError !== ""
            Label { Layout.fillWidth: true; text: root.loadError; color: Theme.danger; wrapMode: Text.Wrap }
            ExpressionButton { text: qsTr("Retry"); onClicked: root.searchNow() }
        }
        RowLayout {
            Layout.fillWidth: true
            ExpressionButton {
                id: backAction
                objectName: "newGroupBackAction"
                text: root.reviewing && !root.sending ? qsTr("Back") : root.sending ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.back()
            }
            Item { Layout.fillWidth: true }
            ExpressionButton {
                objectName: "newGroupCreateAction"
                text: root.reviewing ? qsTr("Create") : qsTr("Next")
                primary: true
                enabled: root.selected.length > 0 && !root.sending && !root.client.groupCreationBusy
                    && root.sameAccount && (!root.reviewing || root.validName && root.client.status.connected)
                onClicked: {
                    if (!root.reviewing) {
                        root.reviewing = true
                        nameField.forceActiveFocus(Qt.TabFocusReason)
                        return
                    }
                    root.error = ""
                    root.sending = true
                    root.requestToken = "group-create:" + (++root.serial)
                    root.client.createGroup(root.subject, root.selectedJids, root.requestToken, root.targetProfile)
                }
            }
        }
    }
}
