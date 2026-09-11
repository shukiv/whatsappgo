import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import org.whatsappgo

Popup {
    id: root
    objectName: "groupPhotoDialog"
    required property var client
    property string targetChat: ""
    property string targetProfile: ""
    property string groupName: ""
    property string token: ""
    property int serial: 0
    property string preview: ""
    property string error: ""
    property bool removing: false
    property bool preparing: false
    property bool saving: false
    property bool needsReopen: false
    property var returnFocus: null
    readonly property bool sameTarget: targetChat !== "" && targetChat === String(client.selectedChat.jid || "") && targetProfile === client.profile
    readonly property bool canEdit: sameTarget && Boolean(client.groupInfo.can_edit_info) && Boolean(client.groupInfo.is_member)
    readonly property string currentPhoto: String(client.selectedChat.avatar_path || "")
    signal notice(string message)

    parent: Overlay.overlay
    anchors.centerIn: parent
    width: Math.min(460, parent ? parent.width - 32 : 460)
    height: Math.min(560, parent ? parent.height - 32 : 560)
    padding: 20; modal: true; focus: true
    closePolicy: Popup.NoAutoClose
    Overlay.modal: Rectangle { color: Theme.dark ? "#B3000000" : "#99000000" }
    background: Rectangle { color: Theme.surface; radius: 12; border.color: Theme.border }

    function showForGroup() {
        const jid = String(client.selectedChat.jid || "")
        if (!jid.endsWith("@g.us") || client.groupInfo.jid !== jid || !client.groupInfo.can_edit_info
                || !client.groupInfo.is_member || client.groupInfoLoading || client.groupActionBusy) return
        targetChat = jid; targetProfile = client.profile
        groupName = String(client.groupInfo.name || client.selectedChat.title || "")
        token = "group-photo:" + (++serial)
        preview = error = ""
        removing = preparing = saving = needsReopen = false
        returnFocus = parent && parent.Window.window ? parent.Window.window.activeFocusItem : null
        open()
    }
    function prepare(url) {
        if (!visible || !canEdit || saving || preparing || needsReopen) return
        client.discardGroupPhoto(token)
        token = "group-photo:" + (++serial)
        removing = false; preview = error = ""; preparing = true
        client.prepareGroupPhoto(targetChat, url, token, targetProfile)
    }
    function back() {
        if (removing && !saving) { removing = false; cancelButton.forceActiveFocus(Qt.TabFocusReason) }
        else close()
    }
    onOpened: cancelButton.forceActiveFocus(Qt.TabFocusReason)
    onClosed: {
        filePicker.close()
        client.discardGroupPhoto(token)
        token = ""; preview = ""; preparing = false
        Qt.callLater(function() { if (root.sameTarget && returnFocus && returnFocus.visible) returnFocus.forceActiveFocus(Qt.TabFocusReason) })
    }
    Shortcut { sequence: "Escape"; enabled: root.visible && !filePicker.visible; autoRepeat: false; onActivated: root.back() }
    FileDialog {
        id: filePicker
        objectName: "groupPhotoFilePicker"
        title: qsTr("Choose a group photo")
        fileMode: FileDialog.OpenFile
        nameFilters: [qsTr("Photos (*.jpg *.jpeg *.png)")]
        onAccepted: if (root.visible && root.sameTarget) root.prepare(selectedFile.toString())
    }
    Connections {
        target: root.client
        function onProfileChanged() { if (root.visible) root.close() }
        function onSelectedChatChanged() {
            if (root.visible && root.targetChat !== String(root.client.selectedChat.jid || "")) root.close()
        }
        function onGroupPhotoPrepared(token, preview, error) {
            if (!root.visible || token !== root.token || !root.sameTarget) return
            root.preparing = false; root.preview = preview; root.error = error
        }
        function onGroupPhotoSaved(token, error) {
            if (!root.visible || token !== root.token || !root.sameTarget) return
            root.saving = false; root.error = error
            if (error !== "") root.needsReopen = true
            else {
                root.notice(root.removing ? qsTr("Group photo removed") : qsTr("Group photo updated"))
                root.close()
            }
        }
    }
    contentItem: ColumnLayout {
        spacing: 12
        Label {
            Layout.fillWidth: true
            text: root.removing ? qsTr("Remove group photo?") : qsTr("Edit group photo")
            font.pixelSize: 20; font.weight: Font.Medium; color: Theme.text
        }
        Label {
            Layout.fillWidth: true
            text: root.removing ? qsTr("Remove the photo for %1? This changes it for everyone in the group.").arg(root.groupName)
                : qsTr("Choose a JPEG or PNG, then review the centered square crop before saving. Your original file stays unchanged.")
            textFormat: Text.PlainText; wrapMode: Text.Wrap; color: Theme.textMuted
        }
        Item {
            Layout.fillWidth: true; Layout.fillHeight: true
            Image {
                id: photo
                objectName: "groupPhotoPreview"
                anchors.centerIn: parent
                width: Math.min(220, parent.width, parent.height); height: width
                source: root.removing || !root.preview ? Theme.fileUrl(root.currentPhoto) : root.preview
                sourceSize.width: 640; sourceSize.height: 640
                fillMode: Image.PreserveAspectFit; asynchronous: true
                Accessible.role: Accessible.Graphic
                Accessible.name: root.preview && !root.removing ? qsTr("New group photo preview") : qsTr("Current group photo")
            }
            Label {
                anchors.centerIn: parent; color: Theme.textMuted
                visible: !photo.source.toString() && !root.preparing
                text: qsTr("No group photo")
            }
            BusyIndicator { anchors.centerIn: parent; running: root.preparing; visible: running }
        }
        Label {
            objectName: "groupPhotoError"
            Layout.fillWidth: true
            text: root.error || (!root.canEdit ? qsTr("You no longer have permission to edit this group's photo.")
                : root.saving ? qsTr("Saving… Closing does not cancel this change.")
                : root.client.status.connected !== true ? qsTr("Connect to WhatsApp before saving.") : "")
            visible: text !== ""; textFormat: Text.PlainText; wrapMode: Text.Wrap
            color: root.error ? Theme.danger : Theme.textMuted
        }
        RowLayout {
            visible: !root.removing
            Layout.fillWidth: true
            ExpressionButton {
                objectName: "groupPhotoChoose"
                text: root.preview ? qsTr("Choose another") : qsTr("Choose photo")
                enabled: root.canEdit && !root.saving && !root.preparing && !root.needsReopen
                onClicked: filePicker.open()
            }
            ExpressionButton {
                objectName: "groupPhotoRemove"
                text: qsTr("Remove photo")
                visible: root.currentPhoto !== ""
                enabled: root.canEdit && !root.saving && !root.preparing && !root.needsReopen
                onClicked: { root.removing = true; cancelButton.forceActiveFocus(Qt.TabFocusReason) }
            }
        }
        RowLayout {
            Layout.fillWidth: true
            Item { Layout.fillWidth: true }
            ExpressionButton {
                id: cancelButton
                objectName: "groupPhotoCancel"
                text: root.saving || root.needsReopen ? qsTr("Close") : qsTr("Cancel")
                onClicked: root.needsReopen ? root.close() : root.back()
            }
            ExpressionButton {
                id: saveButton
                objectName: "groupPhotoSave"
                text: root.removing ? qsTr("Remove") : qsTr("Save")
                enabled: root.canEdit && !root.saving && !root.preparing && !root.needsReopen
                    && !root.client.groupActionBusy && root.client.status.connected === true
                    && (root.removing || (root.preview !== "" && photo.status === Image.Ready))
                contentItem: Label {
                    text: saveButton.text; font: saveButton.font
                    color: root.removing ? Theme.danger : Theme.primaryText
                    horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    radius: 22
                    color: root.removing ? (saveButton.hovered ? Theme.hoverRow : "transparent") : Theme.primary
                    border.color: root.removing ? Theme.danger : Theme.primary
                    border.width: saveButton.activeFocus ? 2 : 1
                }
                onClicked: {
                    root.saving = true; root.error = ""
                    root.client.saveGroupPhoto(root.targetChat, root.removing, root.token, root.targetProfile)
                }
            }
        }
    }
}
