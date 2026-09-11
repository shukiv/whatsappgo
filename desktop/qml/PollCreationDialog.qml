import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    required property var client
    objectName: "pollCreationDialog"
    property string chat: ""
    property bool submitted: false
    title: qsTr("Create poll")
    preferredWidth: 500
    preferredHeight: 620
    showAccept: false
    cancelText: qsTr("Close")
    onOpened: {
        chat = String(client.selectedChat.jid || "")
        submitted = false; request.reset(); question.text = ""; multiple.checked = true
        options.clear(); options.append({name:""}); options.append({name:""})
        question.forceActiveFocus()
    }
    onClosed: request.reset()
    Connections {
        target: root.client
        function onProfileChanged() { root.close() }
        function onSelectedChatChanged() { if (root.opened && root.chat !== String(root.client.selectedChat.jid || "")) root.close() }
    }
    FeatureRequest {
        id: request; client: root.client
        onFinished: (result,error) => { if (!error) root.close() }
    }
    ListModel { id: options }
    Label { Layout.fillWidth: true; text: qsTr("Question"); color: Theme.text }
    DialogTextField { id: question; objectName: "pollQuestion"; Layout.fillWidth: true; placeholderText: qsTr("Ask a question"); maximumLength: 510; enabled: !request.busy }
    Label { text: qsTr("Options (2–12)"); color: Theme.text }
    ScrollView {
        Layout.fillWidth: true; Layout.fillHeight: true; clip: true; contentWidth: availableWidth
        ColumnLayout {
            width: parent.width
            Repeater {
                model: options
                RowLayout {
                    required property int index
                    required property string name
                    Layout.fillWidth: true
                    DialogTextField {
                        objectName: "pollOption" + index
                        Layout.fillWidth: true; text: name
                        placeholderText: qsTr("Option %1").arg(index+1)
                        maximumLength: 200; enabled: !request.busy
                        onTextEdited: options.setProperty(index,"name",text)
                    }
                    ThemedToolButton {
                        iconSource: Qt.resolvedUrl("icons/delete.svg"); iconSize: 18
                        Accessible.name: qsTr("Remove option %1").arg(index+1)
                        enabled: options.count > 2 && !request.busy
                        onClicked: options.remove(index)
                    }
                }
            }
            GroupInfoRow { Layout.fillWidth: true; text: qsTr("Add option"); enabled: options.count < 12 && !request.busy; onClicked: options.append({name:""}) }
        }
    }
    CheckBox {
        id: multiple; text: qsTr("Allow multiple answers"); enabled: !request.busy
        contentItem: Label { text: multiple.text; color: Theme.text; leftPadding: multiple.indicator.width+multiple.spacing; verticalAlignment: Text.AlignVCenter }
    }
    Label { Layout.fillWidth: true; visible: request.error !== ""; text: request.error; textFormat: Text.PlainText; color: Theme.danger; wrapMode: Text.Wrap }
    Label { Layout.fillWidth: true; visible: root.submitted; text: qsTr("Submission cannot be cancelled. If confirmation fails, check the chat before creating another poll."); color: Theme.textMuted; wrapMode: Text.Wrap }
    GroupInfoRow {
        objectName: "pollCreateSend"; Layout.fillWidth: true
        text: request.busy ? qsTr("Sending…") : qsTr("Send poll")
        enabled: !root.submitted && root.chat !== "" && question.text.trim() !== ""
        onClicked: {
            const names=[]; for(let i=0;i<options.count;i++) names.push(options.get(i).name)
            if(Array.from(question.text).length>255 || names.some(n=>Array.from(n).length>100)) {request.error=qsTr("Use at most 255 characters for the question and 100 for each option.");return}
            if(names.some(n=>!n.trim()) || new Set(names.map(n=>n.trim().toLowerCase())).size!==names.length) { request.error=qsTr("Enter distinct, nonempty options."); return }
            root.submitted=true
            request.run("poll.create",{chat_jid:root.chat,question:question.text,options:names,multiple:multiple.checked})
        }
    }
}
