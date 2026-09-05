import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// WhatsApp Web answers a sidebar query with three labelled groups - Chats,
// Contacts and Messages - in place of the chat list, rather than filtering the
// list in situ. One ListView carries all three so the whole result set scrolls
// as a single column, with headers injected between the groups.
Item {
    id: root

    property string query: ""
    property var chatHits: []
    property var contactHits: []
    property var messageHits: []

    signal chatChosen(string jid, string title)
    signal messageChosen(string chatJid, string chatTitle, string messageId)

    readonly property int resultCount: (chatHits ? chatHits.length : 0)
        + (contactHits ? contactHits.length : 0)
        + (messageHits ? messageHits.length : 0)

    // A header only exists when its group has hits, so an empty group leaves no
    // orphaned label behind.
    readonly property var rows: {
        const rows = []
        const chats = root.chatHits || []
        if (chats.length > 0) {
            rows.push({ kind: "header", label: qsTr("Chats") })
            for (let i = 0; i < chats.length; ++i)
                rows.push({ kind: "chat", item: chats[i] })
        }
        const contacts = root.contactHits || []
        if (contacts.length > 0) {
            rows.push({ kind: "header", label: qsTr("Contacts") })
            for (let j = 0; j < contacts.length; ++j)
                rows.push({ kind: "contact", item: contacts[j] })
        }
        const messages = root.messageHits || []
        if (messages.length > 0) {
            rows.push({ kind: "header", label: qsTr("Messages") })
            for (let k = 0; k < messages.length; ++k)
                rows.push({ kind: "message", item: messages[k] })
        }
        return rows
    }

    ListView {
        id: resultList
        objectName: "searchResultsList"
        anchors.fill: parent
        anchors.leftMargin: 7
        anchors.rightMargin: 8
        model: root.rows
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: OverlayScrollBar {}

        delegate: Loader {
            required property var modelData
            width: ListView.view.width
            sourceComponent: {
                switch (String(modelData.kind)) {
                case "header": return sectionHeader
                case "chat": return chatResult
                case "contact": return contactResult
                default: return messageResult
                }
            }
            onLoaded: item.entry = modelData
        }
    }

    Label {
        objectName: "searchResultsEmpty"
        anchors.centerIn: parent
        width: parent.width - 48
        visible: root.resultCount === 0
        horizontalAlignment: Text.AlignHCenter
        wrapMode: Text.Wrap
        color: Theme.textMuted
        font.pixelSize: 14
        text: qsTr("No chats, contacts or messages found")
        Accessible.name: text
    }

    Component {
        id: sectionHeader
        Item {
            property var entry: ({})
            height: 44
            Label {
                anchors.left: parent.left
                anchors.leftMargin: 12
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 6
                text: String(entry.label || "")
                color: Theme.textMuted
                font.pixelSize: 13
            }
        }
    }

    Component {
        id: chatResult
        ChatListDelegate {
            property var entry: ({})
            modelData: entry.item || ({})
            current: backend.selectedChat.jid === (entry.item ? entry.item.jid : "")
            highlightQuery: root.query
            onChosen: (jid, title) => root.chatChosen(jid, title)
            onAvatarRequested: jid => backend.refreshChatAvatar(jid)
        }
    }

    // A contact has no conversation yet, so the row carries a name and nothing
    // else - no preview, no timestamp, exactly as WhatsApp Web shows it.
    Component {
        id: contactResult
        ItemDelegate {
            id: contactRow
            property var entry: ({})
            readonly property var item: entry.item || ({})
            readonly property string contactName: String(item.name || item.phone || "")
            height: 72
            padding: 0
            leftPadding: 12
            rightPadding: 12
            hoverEnabled: true
            Accessible.name: qsTr("Start a chat with %1").arg(contactName)
            onClicked: root.chatChosen(String(item.jid || ""), contactName)
            background: Rectangle {
                radius: 8
                color: contactRow.hovered ? Theme.hoverRow : "transparent"
            }
            contentItem: RowLayout {
                spacing: 13
                Avatar {
                    Layout.preferredWidth: 49
                    Layout.preferredHeight: 49
                    diameter: 49
                    title: contactRow.contactName
                    source: Theme.fileUrl(item.avatar_path)
                }
                Label {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    text: SearchHighlight.markup(contactRow.contactName, root.query, Theme.primary)
                    textFormat: Text.StyledText
                    color: Theme.text
                    font.pixelSize: 16
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }
    }

    // A message hit is indented to the text column with no avatar: the chat
    // name is the heading and the matched line sits under it.
    Component {
        id: messageResult
        ItemDelegate {
            id: messageRow
            property var entry: ({})
            readonly property var item: entry.item || ({})
            readonly property string chatLabel: String(item.chat_title || item.chat_jid || "")
            height: 78
            padding: 0
            leftPadding: 12
            rightPadding: 12
            hoverEnabled: true
            Accessible.name: qsTr("%1: %2").arg(chatLabel).arg(String(item.body || ""))
            onClicked: root.messageChosen(String(item.chat_jid || ""), chatLabel, String(item.id || ""))
            background: Rectangle {
                radius: 8
                color: messageRow.hovered ? Theme.hoverRow : "transparent"
            }
            contentItem: ColumnLayout {
                spacing: 4
                Item { Layout.fillHeight: true }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: 8
                    Label {
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        text: messageRow.chatLabel
                        color: Theme.text
                        font.pixelSize: 15
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                    Label {
                        text: RowTime.label(item.timestamp)
                        color: Theme.textMuted
                        font.pixelSize: 12
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: 4
                    ReadReceipt {
                        visible: Boolean(item.from_me) && String(item.status || "") !== ""
                        Layout.alignment: Qt.AlignVCenter
                        status: String(item.status || "")
                    }
                    Label {
                        objectName: "searchMessageBody"
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                        // Message bodies come from other people and are drawn as
                        // rich text, so the markup helper escapes them first.
                        text: SearchHighlight.markup(item.body, root.query, Theme.primary)
                        textFormat: Text.StyledText
                        color: Theme.textMuted
                        font.pixelSize: 14
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                }
                Item { Layout.fillHeight: true }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }
    }
}
