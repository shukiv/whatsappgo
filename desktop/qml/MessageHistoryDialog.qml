import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// Every version one message has had, oldest first. WhatsApp Web only marks a
// message as edited; the versions themselves are kept by the daemon, and a
// deleted message keeps its text nowhere else, so this reads them from there
// rather than from the conversation, which holds only what is current.
WhatsAppDialog {
    id: root
    objectName: "messageHistoryDialog"
    title: qsTr("Edit history")
    preferredWidth: 460
    preferredHeight: 520
    showAccept: false
    cancelText: qsTr("Close")

    property string chatJid: ""
    property var message: ({})
    property var revisions: []
    property string errorText: ""
    property bool loading: false
    property string loadToken: ""
    property string ownerProfile: ""
    property int serial: 0

    // recorded_at says when a version stopped being current, so each version
    // was written when the one before it was replaced, and the first was
    // written when the message itself was sent.
    function buildEntries() {
        const entries = []
        const revisions = root.revisions || []
        const sent = Number(root.message.timestamp || 0)
        for (let i = 0; i < revisions.length; ++i) {
            entries.push({
                label: i === 0 ? qsTr("Original") : qsTr("Edited"),
                body: String(revisions[i].body || ""),
                at: i === 0 ? sent : Number(revisions[i - 1].recorded_at || 0),
                deleted: false
            })
        }
        const lastChange = revisions.length > 0
            ? Number(revisions[revisions.length - 1].recorded_at || 0) : sent
        if (Boolean(root.message.revoked)) {
            entries.push({label: qsTr("Deleted"), body: "", at: lastChange, deleted: true})
        } else {
            entries.push({
                // A message corrected before these versions were kept has none
                // stored, but what stands is still not what was first written.
                label: revisions.length > 0 || Boolean(root.message.edited)
                    ? qsTr("Current") : qsTr("Original"),
                body: String(root.message.body || ""),
                at: lastChange,
                deleted: false
            })
        }
        return entries
    }

    readonly property var entries: root.buildEntries()

    function formatTime(at) {
        return at > 0 ? new Date(at).toLocaleString(Qt.locale(), Locale.ShortFormat) : ""
    }

    function show(chatJid, message) {
        root.chatJid = String(chatJid || "")
        root.message = message || ({})
        root.revisions = []
        root.errorText = ""
        root.ownerProfile = backend.profile
        root.serial += 1
        root.loadToken = "history-" + root.serial
        root.loading = true
        backend.loadMessageRevisions(root.chatJid, String(root.message.id || ""), root.loadToken)
        root.open()
    }

    onClosed: {
        // A late answer must not repopulate a dialog the reader has dismissed.
        root.loadToken = ""
        root.loading = false
    }

    Connections {
        target: backend
        function onProfileChanged() { root.close() }
        function onMessageRevisionsLoaded(token, revisions, error) {
            if (!root.visible || token === "" || token !== root.loadToken) return
            root.loading = false
            root.errorText = error
            root.revisions = error === "" ? revisions : []
        }
    }

    Label {
        objectName: "messageHistoryStatus"
        Layout.fillWidth: true
        visible: text !== ""
        wrapMode: Text.Wrap
        color: root.errorText !== "" ? Theme.danger : Theme.textMuted
        text: root.loading ? qsTr("Loading earlier versions…")
            : root.errorText !== "" ? root.errorText
            : (root.revisions || []).length === 0
                ? qsTr("No earlier version of this message was kept. Only versions seen since this feature arrived are stored.")
                : ""
    }

    ListView {
        id: versions
        objectName: "messageHistoryList"
        Layout.fillWidth: true
        Layout.fillHeight: true
        clip: true
        spacing: 8
        model: root.entries
        ScrollBar.vertical: OverlayScrollBar {}

        delegate: Rectangle {
            id: version
            required property var modelData
            required property int index
            objectName: "messageHistoryEntry" + index
            width: versions.width
            height: entry.implicitHeight + 24
            radius: 10
            color: Theme.surfaceMuted

            ColumnLayout {
                id: entry
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: 12 }
                spacing: 4

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 8
                    Label {
                        objectName: "messageHistoryLabel" + version.index
                        text: version.modelData.label
                        color: version.modelData.deleted ? Theme.danger : Theme.primary
                        font.pixelSize: 12
                        font.weight: Font.Medium
                    }
                    Label {
                        objectName: "messageHistoryTime" + version.index
                        Layout.fillWidth: true
                        horizontalAlignment: Text.AlignRight
                        text: root.formatTime(Number(version.modelData.at || 0))
                        color: Theme.textMuted
                        font.pixelSize: 12
                        elide: Text.ElideRight
                    }
                    ThemedToolButton {
                        objectName: "messageHistoryCopy" + version.index
                        Layout.preferredWidth: 24
                        Layout.preferredHeight: 24
                        visible: String(version.modelData.body || "") !== ""
                        iconSize: 13
                        iconSource: Qt.resolvedUrl("icons/copy.svg")
                        Accessible.name: qsTr("Copy this version")
                        ToolTip.visible: hovered
                        ToolTip.text: Accessible.name
                        onClicked: backend.copyText(String(version.modelData.body || ""))
                    }
                }

                TextEdit {
                    objectName: "messageHistoryBody" + version.index
                    Layout.fillWidth: true
                    visible: !version.modelData.deleted
                    text: String(version.modelData.body || "")
                    textFormat: TextEdit.PlainText
                    readOnly: true
                    selectByMouse: true
                    selectByKeyboard: true
                    wrapMode: TextEdit.Wrap
                    color: Theme.text
                    font.pixelSize: 14
                    selectionColor: Theme.primary
                    selectedTextColor: Theme.primaryText
                    Accessible.name: qsTr("%1 version of the message").arg(version.modelData.label)
                }

                Label {
                    objectName: "messageHistoryTombstone" + version.index
                    Layout.fillWidth: true
                    visible: version.modelData.deleted
                    text: qsTr("This message was deleted")
                    color: Theme.textMuted
                    font.pixelSize: 14
                    font.italic: true
                    wrapMode: Text.Wrap
                }
            }
        }
    }
}
