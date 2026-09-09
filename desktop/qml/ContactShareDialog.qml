import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "contactShareDialog"
    required property var client
    property string targetProfile: ""
    property string targetChat: ""
    property string targetTitle: ""
    property string targetReply: ""
    property bool reviewing: false
    property bool loading: false
    property bool sending: false
    property string loadError: ""
    property string sendError: ""
    property var contacts: []
    property int serial: 0
    property string searchToken: ""
    property string sendToken: ""
    readonly property string phone: phoneField.text.trim().replace(/[ ().-]/g, "")
    readonly property bool validCard: nameField.text.trim().length > 0
        && !/[\u0000-\u001f\u007f-\u009f\u2028\u2029]/.test(nameField.text)
        && /^\+[1-9][0-9]{6,14}$/.test(phone)
    readonly property bool sameTarget: targetProfile === client.profile
        && targetChat !== "" && targetChat === String(client.selectedChat.jid || "")

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(480, parent ? parent.width - 32 : 480)
    height: Math.min(590, parent ? parent.height - 32 : 590)
    padding: 20
    modal: true
    focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    function showFor(replyTo) {
        targetProfile = client.profile
        targetChat = String(client.selectedChat.jid || "")
        targetTitle = String(client.selectedChat.title || targetChat)
        targetReply = replyTo
        reviewing = false
        sending = false
        sendError = ""
        loadError = ""
        searchField.text = ""
        contacts = []
        open()
        searchNow()
        searchField.forceActiveFocus(Qt.TabFocusReason)
    }
    function searchNow() {
        searchDelay.stop()
        if (!visible || !sameTarget) return
        searchToken = "contact-search:" + (++serial)
        loading = true
        loadError = ""
        client.searchShareContacts(searchField.text.trim(), searchToken)
    }
    function review(name, number) {
        nameField.text = name
        phoneField.text = number
        sendError = ""
        reviewing = true
        nameField.forceActiveFocus(Qt.TabFocusReason)
    }
    function back() {
        if (reviewing && !sending) {
            reviewing = false
            searchField.forceActiveFocus(Qt.TabFocusReason)
        } else close()
    }
    onClosed: {
        searchDelay.stop()
        searchToken = ""
        sendToken = ""
        contacts = []
        nameField.text = ""
        phoneField.text = ""
    }
    Shortcut { sequence: "Escape"; enabled: root.visible; autoRepeat: false; onActivated: root.back() }
    Timer { id: searchDelay; interval: 180; onTriggered: root.searchNow() }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onSelectedChatChanged() {
            // Read the source property here: the derived binding may update
            // after this signal handler, leaving an old review visible.
            if (root.visible && root.targetChat !== String(root.client.selectedChat.jid || "")) root.close()
        }
        function onShareContactsReady(token, contacts, error) {
            if (!root.visible || token !== root.searchToken || !root.sameTarget) return
            root.loading = false
            root.contacts = contacts
            root.loadError = error
        }
        function onExpressionSendFinished(token, success) {
            if (!root.visible || token !== root.sendToken) return
            root.sending = false
            if (success) root.close()
            else root.sendError = qsTr("The contact could not be sent. Check the connection and try again.")
        }
    }

    contentItem: ColumnLayout {
        spacing: 12
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            ThemedToolButton {
                id: backButton
                objectName: "contactShareBack"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/back.svg")
                iconSize: 22
                Accessible.name: root.reviewing ? qsTr("Back to contacts") : qsTr("Back to chat")
                onClicked: root.back()
                background: Rectangle {
                    radius: 20
                    color: backButton.hovered ? Theme.hoverRow : "transparent"
                    border.width: backButton.visualFocus ? 2 : 0
                    border.color: Theme.primary
                }
            }
            Label {
                Layout.fillWidth: true
                text: root.reviewing ? qsTr("Review contact") : qsTr("Share a contact")
                font.pixelSize: 20
                font.weight: Font.Medium
                color: Theme.text
            }
        }
        Label {
            Layout.fillWidth: true
            text: qsTr("To: %1").arg(root.targetTitle)
            textFormat: Text.PlainText
            elide: Text.ElideRight
            color: Theme.textMuted
        }
        DialogTextField {
            id: searchField
            objectName: "contactShareSearch"
            Layout.fillWidth: true
            visible: !root.reviewing
            search: true
            placeholderText: qsTr("Search name or number")
            maximumLength: 100
            onTextChanged: {
                root.searchToken = "" // Ignore the previous query immediately, even during debounce.
                if (root.visible) { root.loading = true; searchDelay.restart() }
            }
        }
        ExpressionButton {
            objectName: "contactShareManual"
            Layout.fillWidth: true
            visible: !root.reviewing
            text: qsTr("Enter a name and number")
            onClicked: root.review("", "")
        }
        Label {
            Layout.fillWidth: true
            visible: !root.reviewing && (root.loading || root.loadError !== "" || root.contacts.length === 0)
            text: root.loading ? qsTr("Loading contacts…") : root.loadError !== "" ? root.loadError
                : qsTr("No matching contacts with a known phone number. You can enter one above.")
            color: root.loadError !== "" ? Theme.danger : Theme.textMuted
            wrapMode: Text.Wrap
        }
        ExpressionButton {
            visible: !root.reviewing && root.loadError !== ""
            text: qsTr("Retry")
            onClicked: root.searchNow()
        }
        ListView {
            id: contactList
            objectName: "contactShareList"
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: !root.reviewing
            model: root.loading ? [] : root.contacts
            clip: true
            reuseItems: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: OverlayScrollBar {}
            delegate: ItemDelegate {
                required property var modelData
                width: contactList.width
                height: 66
                Accessible.name: String(modelData.name) + ", " + String(modelData.phone)
                onClicked: root.review(String(modelData.name), String(modelData.phone))
                background: Rectangle {
                    radius: 8
                    color: parent.hovered || parent.activeFocus ? Theme.hoverRow : "transparent"
                    border.width: parent.visualFocus ? 2 : 0
                    border.color: Theme.primary
                }
                contentItem: RowLayout {
                    spacing: 12
                    TintedIcon {
                        Layout.preferredWidth: 24
                        Layout.preferredHeight: 24
                        source: Qt.resolvedUrl("icons/contact.svg")
                        tint: Theme.icon
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 4
                        Label {
                            Layout.fillWidth: true
                            text: modelData.name
                            textFormat: Text.PlainText
                            elide: Text.ElideRight
                            color: Theme.text
                            font.pixelSize: 16
                        }
                        Label {
                            Layout.fillWidth: true
                            text: modelData.phone
                            textFormat: Text.PlainText
                            elide: Text.ElideRight
                            horizontalAlignment: Text.AlignLeft
                            color: Theme.textMuted
                        }
                    }
                }
            }
        }
        Label {
            Layout.fillWidth: true
            visible: !root.reviewing && root.contacts.length >= 100
            text: qsTr("Showing the first 100 contacts. Search to narrow the list.")
            color: Theme.textMuted
            wrapMode: Text.Wrap
        }
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: root.reviewing
            spacing: 12
            Label { text: qsTr("Name"); color: Theme.textMuted }
            DialogTextField {
                id: nameField
                objectName: "contactShareName"
                Layout.fillWidth: true
                placeholderText: qsTr("Contact name")
                maximumLength: 100
                enabled: !root.sending
            }
            Label { text: qsTr("Phone number"); color: Theme.textMuted }
            DialogTextField {
                id: phoneField
                objectName: "contactSharePhone"
                Layout.fillWidth: true
                placeholderText: qsTr("+ country code and phone number")
                maximumLength: 40
                inputMethodHints: Qt.ImhDialableCharactersOnly
                horizontalAlignment: Text.AlignLeft
                enabled: !root.sending
            }
            Label {
                Layout.fillWidth: true
                text: qsTr("Only this name and phone number will be shared. Nothing is sent until you press Send contact.")
                color: Theme.textMuted
                wrapMode: Text.Wrap
            }
            Label {
                Layout.fillWidth: true
                visible: !root.validCard && (nameField.text !== "" || phoneField.text !== "")
                text: qsTr("Enter a name and a valid international number starting with + (7–15 digits).")
                color: Theme.textMuted
                wrapMode: Text.Wrap
            }
            Label {
                Layout.fillWidth: true
                visible: root.sendError !== "" || root.sending || root.client.status.connected !== true
                text: root.sending ? qsTr("Sending contact…") : root.sendError !== "" ? root.sendError
                    : qsTr("Reconnect to WhatsApp before sending.")
                color: root.sendError !== "" ? Theme.danger : Theme.textMuted
                wrapMode: Text.Wrap
            }
            Item { Layout.fillHeight: true }
        }
        RowLayout {
            Layout.fillWidth: true
            Item { Layout.fillWidth: true }
            ExpressionButton {
                objectName: "contactShareCancel"
                text: root.sending ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.close()
            }
            ExpressionButton {
                objectName: "contactShareSend"
                visible: root.reviewing
                text: qsTr("Send contact")
                primary: true
                enabled: root.validCard && !root.sending && !root.client.busy
                    && root.client.status.connected === true && root.sameTarget
                onClicked: {
                    root.sendToken = "contact-send:" + (++root.serial)
                    root.sendError = ""
                    root.sending = true
                    root.client.sendContactCard(nameField.text.trim(), root.phone, root.sendToken,
                        root.targetProfile, root.targetChat, root.targetReply)
                }
            }
        }
    }
}
