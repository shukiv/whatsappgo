import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    required property var client
    objectName: "statusAudienceContactsDialog"
    title: qsTr("Status audience contacts")
    preferredWidth: 460; preferredHeight: 540
    showAccept: false; cancelText: qsTr("Close")
    onOpened: { request.reset(); search.clear(); request.run("status.audience_contacts",{}) }
    onClosed: request.reset()
    Connections { target: root.client; function onProfileChanged() { root.close() } }
    FeatureRequest { id: request; client: root.client }
    Label {
        Layout.fillWidth: true; wrapMode: Text.Wrap; color: Theme.textMuted
        text: request.result.type==="whitelist" ? qsTr("Only these contacts can see your status.")
            : request.result.type==="blacklist" ? qsTr("These contacts are excluded from your status audience.")
            : qsTr("Your audience is My contacts. WhatsApp does not return an individual inclusion list for this setting.")
    }
    DialogTextField { id: search; Layout.fillWidth: true; placeholderText: qsTr("Search contacts"); Accessible.name: placeholderText }
    BusyIndicator { running: request.busy; visible: running; Layout.alignment: Qt.AlignHCenter }
    ListView {
        Layout.fillWidth: true; Layout.fillHeight: true; clip: true
        model: (request.result.people || []).filter(p=>(String(p.name || "")+" "+String(p.phone || "")).toLocaleLowerCase().indexOf(search.text.trim().toLocaleLowerCase())>=0)
        ScrollBar.vertical: OverlayScrollBar {}
        delegate: ItemDelegate {
            required property var modelData
            width: ListView.view.width; height: 64
            contentItem: RowLayout {
                Avatar { Layout.preferredWidth: 40; Layout.preferredHeight: 40; diameter: 40; title: modelData.name || "?"; source: Theme.fileUrl(modelData.avatar_path) }
                Label { Layout.fillWidth: true; text: modelData.name || modelData.phone || qsTr("Contact"); color: Theme.text; textFormat: Text.PlainText; elide: Text.ElideRight }
            }
            Accessible.name: modelData.name || modelData.phone || qsTr("Contact")
        }
    }
    Label { Layout.fillWidth: true; visible: request.error!==""; text: request.error; textFormat: Text.PlainText; color: Theme.danger; wrapMode: Text.Wrap }
    Label { Layout.fillWidth: true; text: qsTr("Change this audience in WhatsApp on your phone."); color: Theme.textMuted; wrapMode: Text.Wrap }
    GroupInfoRow { Layout.fillWidth: true; text: qsTr("Refresh"); enabled: !request.busy; onClicked: request.run("status.audience_contacts",{}) }
}
