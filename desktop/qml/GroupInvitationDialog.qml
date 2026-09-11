import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    required property var client
    objectName: "joinGroupDialog"
    property bool submitted: false
    title: qsTr("Join a group with a link")
    preferredWidth: 460
    showAccept: false; cancelText: qsTr("Close")
    onOpened: { submitted=false; read.reset(); write.reset(); link.text=""; link.forceActiveFocus() }
    onClosed: { read.reset(); write.reset() }
    Connections { target: root.client; function onProfileChanged() { root.close() } }
    FeatureRequest { id: read; client: root.client }
    FeatureRequest {
        id: write; client: root.client
        onFinished: (result,error) => { if(!error && result.joined) { root.close(); root.client.openChat(result.jid,result.name) } }
    }
    Label { Layout.fillWidth: true; text: qsTr("Preview the group first. Nothing is joined until you confirm."); wrapMode: Text.Wrap; color: Theme.textMuted }
    DialogTextField { id: link; objectName: "joinGroupLinkField"; Layout.fillWidth: true; placeholderText: "https://chat.whatsapp.com/…"; enabled: !read.busy && !root.submitted; onTextEdited: read.reset() }
    GroupInfoRow { objectName: "groupInvitationPreview"; Layout.fillWidth: true; text: read.busy ? qsTr("Loading…") : qsTr("Preview group"); enabled: link.text.trim()!=="" && !read.busy && !root.submitted; onClicked: read.run("group.invite_preview",{link:link.text}) }
    Label { Layout.fillWidth: true; text: read.result.name || ""; textFormat: Text.PlainText; color: Theme.text; font.bold: true; wrapMode: Text.Wrap }
    Label { Layout.fillWidth: true; visible: Boolean(read.result.jid); text: qsTr("%1 participants").arg(read.result.participant_count || 0); color: Theme.textMuted }
    Label { Layout.fillWidth: true; text: read.result.description || ""; textFormat: Text.PlainText; color: Theme.text; wrapMode: Text.Wrap; maximumLineCount: 6; elide: Text.ElideRight }
    Label { Layout.fillWidth: true; text: read.error || write.error; visible: text!==""; textFormat: Text.PlainText; color: Theme.danger; wrapMode: Text.Wrap }
    Label { Layout.fillWidth: true; visible: root.submitted; text: !write.busy && !write.error ? qsTr("Request submitted. Membership is not yet confirmed; check for admin approval in WhatsApp.") : qsTr("Closing does not cancel this request. If confirmation fails, check WhatsApp before trying again."); color: Theme.textMuted; wrapMode: Text.Wrap }
    GroupInfoRow {
        objectName: "previewedGroupJoin"; Layout.fillWidth: true
        text: write.busy ? qsTr("Submitting…") : read.result.approval_required ? qsTr("Request to join") : qsTr("Join this group")
        enabled: Boolean(read.result.jid) && !read.error && !read.busy && !root.submitted
        onClicked: { root.submitted=true; write.run("group.join_previewed",{link:link.text,expected_jid:read.result.jid}) }
    }
}
