import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    required property var client
    property string jid: ""
    objectName: "groupInviteQRDialog"
    title: qsTr("Group invite QR code")
    subtitle: qsTr("Anyone who scans this code can join or request to join. Share only with people you trust.")
    preferredWidth: 460
    showAccept: false; cancelText: qsTr("Close")
    function showFor(jid) { root.jid=String(jid); request.reset(); open(); request.run("group.invite_qr",{chat_jid:root.jid}) }
    onClosed: request.reset()
    Connections {
        target: root.client
        function onProfileChanged() { root.close() }
        function onSelectedChatChanged() { if(root.opened && root.jid!==String(root.client.selectedChat.jid || "")) root.close() }
    }
    FeatureRequest { id: request; client: root.client }
    BusyIndicator { running: request.busy; visible: running; Layout.alignment: Qt.AlignHCenter }
    Image {
        objectName: "groupInviteQRImage"
        Layout.alignment: Qt.AlignHCenter
        Layout.preferredWidth: 280; Layout.preferredHeight: 280
        source: request.result.image || ""; fillMode: Image.PreserveAspectFit
        visible: source!==""; smooth: false
        Accessible.name: qsTr("Scannable invitation to this group")
    }
    Label { Layout.fillWidth: true; text: request.error; visible: text!==""; color: Theme.danger; textFormat: Text.PlainText; wrapMode: Text.Wrap }
    GroupInfoRow { Layout.fillWidth: true; text: qsTr("Copy invite link"); enabled: Boolean(request.result.link) && !request.busy; onClicked: root.client.copyText(request.result.link) }
    GroupInfoRow { Layout.fillWidth: true; text: qsTr("Refresh code"); enabled: !request.busy; onClicked: {request.reset();request.run("group.invite_qr",{chat_jid:root.jid})} }
}
