import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "newCommunityDialog"
    required property var client
    property string targetProfile: ""
    property string requestToken: ""
    property int serial: 0
    property bool sending: false
    property string error: ""
    property var returnFocus: null
    readonly property string subject: nameField.text.trim()
    readonly property int characterCount: (subject.match(/[\uD800-\uDBFF][\uDC00-\uDFFF]|[\s\S]/g) || []).length
    readonly property bool validName: characterCount > 0 && characterCount <= 100
        && !/[\u0000-\u001f\u007f-\u009f\u2028\u2029]/.test(subject)
    readonly property bool sameAccount: targetProfile === client.profile
    readonly property bool canSubmit: validName && !sending && !client.communityCreationBusy
        && sameAccount && client.status.connected === true
    signal created()

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(480, parent ? parent.width - 32 : 480)
    height: Math.min(440, parent ? parent.height - 32 : 440)
    padding: 20
    modal: true
    focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    onAboutToShow: {
        returnFocus = parent && parent.Window.window ? parent.Window.window.activeFocusItem : null
        targetProfile = client.profile
        requestToken = ""
        sending = false
        error = ""
        nameField.text = ""
    }
    onOpened: nameField.forceActiveFocus(Qt.TabFocusReason)
    onClosed: {
        requestToken = ""
        Qt.callLater(function() { if (returnFocus && returnFocus.visible) returnFocus.forceActiveFocus(Qt.TabFocusReason) })
    }
    function submit() {
        if (!canSubmit) return
        requestToken = "community-create:" + (++serial)
        error = ""
        sending = true
        closeAction.forceActiveFocus(Qt.TabFocusReason)
        client.createCommunity(subject, requestToken, targetProfile)
    }
    Shortcut { sequence: "Escape"; enabled: root.visible; autoRepeat: false; onActivated: root.close() }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onCommunityCreationFinished(token, community, error) {
            if (!root.visible || token !== root.requestToken || !root.sameAccount) return
            root.sending = false
            root.error = error
            if (error === "") { root.close(); root.created() }
        }
    }
    contentItem: ColumnLayout {
        spacing: 12
        Label {
            Layout.fillWidth: true
            text: qsTr("New community")
            color: Theme.text
            font.pixelSize: 20
            font.weight: Font.Medium
        }
        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: availableWidth
            clip: true
            ScrollBar.vertical: OverlayScrollBar {}
            ColumnLayout {
                width: parent.width
                spacing: 12
                Label {
                    Layout.fillWidth: true
                    text: qsTr("Bring related groups together. WhatsApp creates an announcement group for your community.")
                    color: Theme.textMuted
                    wrapMode: Text.Wrap
                }
                Label { text: qsTr("Community name"); color: Theme.text }
                DialogTextField {
                    id: nameField
                    objectName: "newCommunityNameField"
                    Layout.fillWidth: true
                    enabled: !root.sending
                    placeholderText: qsTr("Community name")
                    Accessible.name: qsTr("Community name")
                    onAccepted: root.submit()
                }
                Label {
                    Layout.fillWidth: true
                    horizontalAlignment: Text.AlignRight
                    text: qsTr("%1 / 100 characters").arg(root.characterCount)
                    color: root.characterCount <= 100 ? Theme.textMuted : Theme.danger
                }
                Label {
                    Layout.fillWidth: true
                    text: qsTr("Photo, description and group setup are not available here yet. You can manage them from WhatsApp on your phone after creating the community.")
                    color: Theme.textMuted
                    wrapMode: Text.Wrap
                }
                Label {
                    objectName: "newCommunityFeedback"
                    Layout.fillWidth: true
                    visible: text !== ""
                    text: root.error || (root.sending ? qsTr("Creating… Closing this window does not cancel creation.")
                        : root.client.communityCreationBusy ? qsTr("Another creation request is still pending. Wait for it to finish before creating another community.")
                        : root.client.status.connected !== true ? qsTr("Reconnect to WhatsApp before creating a community.") : "")
                    textFormat: Text.PlainText
                    color: root.error ? Theme.danger : Theme.textMuted
                    wrapMode: Text.Wrap
                    Accessible.name: text
                }
            }
        }
        RowLayout {
            Layout.fillWidth: true
            Item { Layout.fillWidth: true }
            ExpressionButton {
                id: closeAction
                objectName: "newCommunityCancel"
                text: root.sending ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.close()
            }
            ExpressionButton {
                objectName: "newCommunityCreate"
                text: qsTr("Create")
                primary: true
                enabled: root.canSubmit
                onClicked: root.submit()
            }
        }
    }
}
