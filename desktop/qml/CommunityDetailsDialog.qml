import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    required property var client
    property string jid: ""
    property string previousDescription: ""
    property bool submitted: false
    property var pendingChange: ({})
    objectName: "communityDetailsDialog"
    title: qsTr("Community details")
    preferredWidth: 520; preferredHeight: 650
    showAccept: false; cancelText: qsTr("Close")
    function showFor(jid) {
        root.jid=String(jid); submitted=false; read.reset(); write.reset()
        description.clear(); open(); read.run("community.info",{chat_jid:root.jid})
    }
    onClosed: { read.reset(); write.reset(); confirmation.close() }
    Connections { target: root.client; function onProfileChanged() { root.close() } }
    FeatureRequest {
        id: read; client: root.client
        onFinished: (result,error) => {
            if(!error) {root.previousDescription=String(result.description || ""); description.text=root.previousDescription}
        }
    }
    FeatureRequest {
        id: write; client: root.client
        onFinished: (result,error) => {
            if(!error) {root.submitted=false; read.run("community.info",{chat_jid:root.jid})}
        }
    }
    Label { Layout.fillWidth: true; text: read.result.name || ""; textFormat: Text.PlainText; color: Theme.text; wrapMode: Text.Wrap; font.bold: true }
    BusyIndicator { running: read.busy || write.busy; visible: running; Layout.alignment: Qt.AlignHCenter }
    ScrollView {
        Layout.fillWidth: true; Layout.fillHeight: true; contentWidth: availableWidth; clip: true
        ColumnLayout {
            width: parent.width
            Label { text: qsTr("Description"); color: Theme.textMuted }
            TextArea {
                id: description; objectName: "communityDescriptionEditor"
                Layout.fillWidth: true
                readOnly: !read.result.can_manage || read.busy || root.submitted
                textFormat: TextEdit.PlainText; wrapMode: TextEdit.Wrap; selectByMouse: true
                color: Theme.text; placeholderText: qsTr("Community description")
                Accessible.name: placeholderText
                background: Rectangle { color: Theme.surfaceMuted; radius: 8 }
            }
            GroupInfoRow {
                objectName: "communitySaveDescription"
                Layout.fillWidth: true; visible: Boolean(read.result.can_manage)
                text: qsTr("Save description…")
                enabled: !read.busy && !root.submitted && description.text!==root.previousDescription && Array.from(description.text).length<=2048
                onClicked: { root.pendingChange={method:"community.description",params:{chat_jid:root.jid,description:description.text,previous:root.previousDescription},label:qsTr("Change this community’s description for everyone?")}; confirmation.open() }
            }
            Label { Layout.fillWidth: true; text: qsTr("Linked groups"); color: Theme.textMuted }
            Repeater {
                model: read.result.linked || []
                GroupInfoRow {
                    required property var modelData
                    objectName: "communityUnlinkGroup"
                    Layout.fillWidth: true; text: modelData.name || qsTr("Group")
                    subtitle: modelData.announcement ? qsTr("Announcements · cannot unlink") : read.result.can_manage ? qsTr("Unlink from community…") : ""
                    enabled: Boolean(read.result.can_manage) && !modelData.announcement && !root.submitted && !read.busy
                    onClicked: { root.pendingChange={method:"community.link",params:{chat_jid:root.jid,child_jid:modelData.jid,action:"unlink"},label:qsTr("Unlink %1 from this community? The group itself will not be deleted.").arg(modelData.name)}; confirmation.open() }
                }
            }
            Label { Layout.fillWidth: true; visible: Boolean(read.result.can_manage); text: qsTr("Available groups you administer"); color: Theme.textMuted; wrapMode: Text.Wrap }
            Repeater {
                model: read.result.can_manage ? read.result.available || [] : []
                GroupInfoRow {
                    required property var modelData
                    objectName: "communityLinkGroup"
                    Layout.fillWidth: true; text: modelData.name || qsTr("Group"); subtitle: qsTr("Link to community…")
                    enabled: !root.submitted && !read.busy
                    onClicked: { root.pendingChange={method:"community.link",params:{chat_jid:root.jid,child_jid:modelData.jid,action:"link"},label:qsTr("Link %1 to this community? This changes the group’s community membership.").arg(modelData.name)}; confirmation.open() }
                }
            }
        }
    }
    Label { Layout.fillWidth: true; text: read.error || write.error; visible: text!==""; textFormat: Text.PlainText; color: Theme.danger; wrapMode: Text.Wrap }
    Label { Layout.fillWidth: true; visible: root.submitted && !write.busy; text: qsTr("The request may have reached WhatsApp. Close and reopen to check its result before trying again."); color: Theme.textMuted; wrapMode: Text.Wrap }
    GroupInfoRow { Layout.fillWidth: true; text: qsTr("Refresh"); enabled: !read.busy && !write.busy && !root.submitted; onClicked: read.run("community.info",{chat_jid:root.jid}) }
    WhatsAppDialog {
        id: confirmation
        objectName: "communityChangeConfirmation"
        title: qsTr("Confirm community change")
        subtitle: root.pendingChange.label || ""
        acceptText: qsTr("Confirm")
        onAccepted: { root.submitted=true; write.run(root.pendingChange.method,root.pendingChange.params) }
    }
}
