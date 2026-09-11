pragma Singleton
import QtQuick

QtObject {
    function body(item) {
        if (item.revoked) return qsTr("This message was deleted")
        if (item.kind === "view_once") return qsTr("View once message")
        if (item.body) return String(item.body)
        if (item.media_name) return String(item.media_name)
        switch (String(item.kind || "")) {
        case "image": return qsTr("Photo")
        case "video": return item.gif_playback ? qsTr("GIF") : qsTr("Video")
        case "audio": return qsTr("Audio")
        case "document": return qsTr("Document")
        case "sticker": return qsTr("Sticker")
        case "poll": return qsTr("Poll")
        case "contact": return qsTr("Contact")
        case "location": return qsTr("Location")
        default: return qsTr("Message")
        }
    }

    function sender(item) {
        if (item.from_me) return qsTr("You")
        return String(item.sender_name || item.sender_jid || "")
    }
}
