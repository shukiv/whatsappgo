import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    required property var client
    objectName: "interactiveMessageDialog"
    property string chat: ""
    property string messageId: ""
    property string kind: "poll"
    property var selected: []
    property bool dirty: false
    property bool submitted: false
    title: kind === "poll" ? qsTr("Poll") : qsTr("Event")
    preferredWidth: 500; preferredHeight: 600
    showAccept: false; cancelText: qsTr("Close")
    function showMessage(message) {
        chat=String(message.chat_jid || client.selectedChat.jid || "")
        messageId=String(message.id || ""); kind=String(message.kind)
        selected=[]; dirty=false; submitted=false; read.reset(); write.reset(); open(); refresh()
    }
    function refresh() { read.run(kind+".info",{chat_jid:chat,message_id:messageId}) }
    onClosed: { read.reset(); write.reset() }
    Timer { interval: 10000; repeat: true; running: root.opened && root.kind==="poll"; onTriggered: { if(!read.busy && !write.busy) root.refresh() } }
    Connections {
        target: root.client
        function onProfileChanged() { root.close() }
        function onSelectedChatChanged() { if(root.opened && root.chat!==String(root.client.selectedChat.jid || "")) root.close() }
    }
    FeatureRequest {
        id: read; client: root.client
        onFinished: (result,error) => {
            if(!error && !root.dirty) root.selected=(result.options || []).filter(o=>o.selected).map(o=>o.name)
        }
    }
    FeatureRequest {
        id: write; client: root.client
        onFinished: (result,error) => {
            root.submitted = false
            if (!error) {
                root.dirty = false
                root.refresh()
            }
        }
    }
    Label { Layout.fillWidth: true; text: root.kind==="poll" ? (read.result.question || "") : (read.result.name || ""); textFormat: Text.PlainText; color: Theme.text; font.bold: true; wrapMode: Text.Wrap }
    BusyIndicator { running: read.busy || write.busy; visible: running; Layout.alignment: Qt.AlignHCenter }
    ScrollView {
        Layout.fillWidth: true; Layout.fillHeight: true; contentWidth: availableWidth; clip: true
        ColumnLayout {
            width: parent.width
            Repeater {
                model: root.kind==="poll" ? read.result.options || [] : []
                CheckBox {
                    id: option
                    required property var modelData
                    objectName: "pollChoice-" + modelData.name
                    Layout.fillWidth: true
                    text: modelData.name + " · " + qsTr("%1 votes").arg(modelData.count)
                    checked: root.selected.indexOf(modelData.name)>=0
                    enabled: !read.busy && !write.busy && !root.submitted
                    contentItem: Label { text: option.text; textFormat: Text.PlainText; color: Theme.text; wrapMode: Text.Wrap; leftPadding: option.indicator.width+option.spacing }
                    onClicked: {
                        const next=root.selected.slice(); const i=next.indexOf(modelData.name)
                        if(i>=0) next.splice(i,1)
                        else if(read.result.limit===1) { next.splice(0,next.length,modelData.name) }
                        else if(!read.result.limit || next.length<read.result.limit) next.push(modelData.name)
                        root.selected=next; root.dirty=true
                    }
                }
            }
            Label { visible: root.kind==="event"; Layout.fillWidth: true; text: read.result.canceled ? qsTr("Cancelled") : qsTr("Event details"); color: read.result.canceled ? Theme.danger : Theme.text }
            Label { visible: root.kind==="event" && Boolean(read.result.start); Layout.fillWidth: true; text: qsTr("Starts: %1").arg(Qt.formatDateTime(new Date(read.result.start || 0),Qt.DefaultLocaleLongDate)); color: Theme.text; wrapMode: Text.Wrap }
            Label { visible: root.kind==="event" && Boolean(read.result.end); Layout.fillWidth: true; text: qsTr("Ends: %1").arg(Qt.formatDateTime(new Date(read.result.end || 0),Qt.DefaultLocaleLongDate)); color: Theme.text; wrapMode: Text.Wrap }
            Label { visible: root.kind==="event"; Layout.fillWidth: true; text: read.result.location || ""; textFormat: Text.PlainText; color: Theme.text; wrapMode: Text.Wrap }
            Label { visible: root.kind==="event"; Layout.fillWidth: true; text: read.result.description || ""; textFormat: Text.PlainText; color: Theme.text; wrapMode: Text.Wrap }
        }
    }
    Label { Layout.fillWidth: true; text: read.error || write.error; visible: text!==""; color: Theme.danger; textFormat: Text.PlainText; wrapMode: Text.Wrap }
    Label { Layout.fillWidth: true; text: root.kind==="poll" ? (read.result.pending ? qsTr("Some votes are waiting for their decryption key. Refresh after history sync.") : qsTr("Results reflect votes received by this device. Refresh to check for updates.")) : qsTr("RSVP and event edits are not available here. Use WhatsApp on your phone."); color: Theme.textMuted; wrapMode: Text.Wrap }
    GroupInfoRow { Layout.fillWidth: true; text: qsTr("Refresh"); enabled: !read.busy && !write.busy; onClicked: root.refresh() }
    GroupInfoRow {
        objectName: "pollSaveVote"; Layout.fillWidth: true; visible: root.kind==="poll"
        text: write.busy ? qsTr("Saving…") : (root.selected.length ? qsTr("Save vote") : qsTr("Remove my vote"))
        enabled: root.dirty && !read.busy && !write.busy && !root.submitted && !read.error
        onClicked: { root.submitted=true; write.run("poll.vote",{chat_jid:root.chat,message_id:root.messageId,options:root.selected}) }
    }
}
