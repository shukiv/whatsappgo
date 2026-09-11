import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ColumnLayout {
    id: root
    property var items: []
    property string scope: ""
    readonly property bool allStarred: items.length > 0 && items.every(item => Boolean(item.starred))
    readonly property bool downloadable: items.some(item => ["image", "video", "document", "audio", "sticker"].indexOf(item.kind) >= 0)
    readonly property real selectedBytes: items.reduce((total, item) => total + Math.max(0, Number(item.media_size || 0)), 0)
    readonly property bool ownBatch: backend.mediaBatchScope === scope
    function perform(action) {
        if (!visible || items.length === 0 || backend.mediaBatchBusy) return
        const selected = action === "download" ? items.filter(item => ["image", "video", "document", "audio", "sticker"].indexOf(item.kind) >= 0) : items
        backend.actOnMediaSelection(selected, scope, action, backend.profile)
    }
    onVisibleChanged: if (!visible && ownBatch) backend.cancelMediaBatch(scope)
    Connections { target: backend; function onProfileChanged() { if (root.ownBatch) backend.cancelMediaBatch(root.scope) } }
    spacing: 2
    RowLayout {
        Layout.fillWidth: true
        Label {
            Layout.fillWidth: true
            text: root.selectedBytes > 0 ? qsTr("%1 selected · %2").arg(root.items.length).arg(
                root.selectedBytes >= 1048576 ? (root.selectedBytes / 1048576).toFixed(1) + " MB"
                : root.selectedBytes >= 1024 ? (root.selectedBytes / 1024).toFixed(1) + " kB"
                : root.selectedBytes + " B") : qsTr("%1 selected").arg(root.items.length)
            color: Theme.text
            elide: Text.ElideRight
            font.pixelSize: 13
        }
        ThemedToolButton {
            objectName: root.scope + "BulkStar"
            Layout.preferredWidth: 40
            Layout.preferredHeight: 40
            enabled: root.items.length > 0 && !backend.mediaBatchBusy
            iconSource: Qt.resolvedUrl("icons/star.svg")
            iconTint: root.allStarred ? Theme.primary : Theme.icon
            Accessible.name: root.allStarred ? qsTr("Unstar selected") : qsTr("Star selected")
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            onClicked: root.perform(root.allStarred ? "unstar" : "star")
        }
        ThemedToolButton {
            objectName: root.scope + "BulkDownload"
            Layout.preferredWidth: 40
            Layout.preferredHeight: 40
            enabled: root.downloadable && !backend.mediaBatchBusy
            iconSource: Qt.resolvedUrl("icons/download.svg")
            Accessible.name: qsTr("Download selected")
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            onClicked: root.perform("download")
        }
    }
    Label {
        objectName: root.scope + "BulkFeedback"
        Layout.fillWidth: true
        visible: root.ownBatch && backend.mediaBatchSummary !== ""
        text: visible ? backend.mediaBatchSummary : ""
        color: Theme.textMuted
        wrapMode: Text.Wrap
        font.pixelSize: 12
    }
}
