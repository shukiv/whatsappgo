import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import org.whatsappgo

WhatsAppDialog {
    id: root
    objectName: "profilePhotoDialog"
    property bool removing: false
    property string pickerProfile: ""
    title: removing ? qsTr("Remove profile photo?") : qsTr("Change profile photo")
    subtitle: removing
        ? qsTr("Remove the photo from your WhatsApp account (%1)?").arg(backend.status.user_name || backend.profile)
        : qsTr("Choose a JPEG or PNG. Review the centered square crop before saving. Your original file stays unchanged.")
    preferredWidth: 440
    showAccept: false
    showCancel: false

    function choosePhoto() {
        if (backend.profilePhotoSaving) return
        removing = false
        backend.clearProfilePhoto()
        open()
        chooseFile()
    }
    function chooseFile() {
        pickerProfile = backend.profile
        filePicker.open()
    }
    function confirmRemoval() {
        if (backend.profilePhotoSaving) return
        backend.clearProfilePhoto()
        removing = true
        open()
    }
    onOpened: cancelButton.forceActiveFocus()
    onClosed: {
        filePicker.close()
        backend.clearProfilePhoto()
    }

    FileDialog {
        id: filePicker
        objectName: "profilePhotoFilePicker"
        title: qsTr("Choose a profile photo")
        fileMode: FileDialog.OpenFile
        nameFilters: [qsTr("Photos (*.jpg *.jpeg *.png)")]
        onAccepted: {
            if (root.visible && root.pickerProfile === backend.profile)
                backend.prepareProfilePhoto(selectedFile.toString())
        }
        onRejected: if (!backend.profilePhotoPreview && !backend.profilePhotoError) root.close()
    }
    Connections {
        target: backend
        function onProfileChanged() { filePicker.close(); root.close() }
        function onStatusChanged() { if (!backend.loggedIn) { filePicker.close(); root.close() } }
        function onProfilePhotoSaved() { root.close() }
    }

    Image {
        objectName: "profilePhotoPreview"
        Layout.alignment: Qt.AlignHCenter
        Layout.preferredWidth: Layout.preferredHeight
        Layout.preferredHeight: Math.max(64, Math.min(220, root.parent ? root.parent.height - 340 : 220))
        visible: source.toString() !== ""
        source: root.removing ? Theme.fileUrl(backend.ownProfile.avatar_path || "") : backend.profilePhotoPreview
        sourceSize.width: 440
        sourceSize.height: 440
        fillMode: Image.PreserveAspectFit
        asynchronous: true
        Accessible.role: Accessible.Graphic
        Accessible.name: root.removing ? qsTr("Current profile photo") : qsTr("New profile photo preview")
    }
    Label {
        Layout.fillWidth: true
        visible: backend.profilePhotoPreparing || backend.profilePhotoSaving
        text: backend.profilePhotoPreparing ? qsTr("Preparing preview…") : qsTr("Saving to WhatsApp…")
        color: Theme.textMuted
        wrapMode: Text.Wrap
    }
    Label {
        objectName: "profilePhotoError"
        Layout.fillWidth: true
        visible: backend.profilePhotoError !== ""
        text: backend.profilePhotoError
        textFormat: Text.PlainText
        color: Theme.danger
        wrapMode: Text.Wrap
    }
    SettingsRow {
        objectName: "profilePhotoChooseButton"
        Layout.fillWidth: true
        visible: !root.removing
        text: backend.profilePhotoPreview ? qsTr("Choose another photo") : qsTr("Choose photo")
        iconSource: Qt.resolvedUrl("icons/gallery.svg")
        enabled: !backend.profilePhotoSaving && !backend.profilePhotoPreparing
        onClicked: root.chooseFile()
    }
    RowLayout {
        Layout.fillWidth: true
        Layout.bottomMargin: 22
        spacing: 12
        Item { Layout.fillWidth: true }
        AbstractButton {
            id: cancelButton
            objectName: "profilePhotoCancelButton"
            implicitWidth: cancelText.implicitWidth + 32
            implicitHeight: 40
            Accessible.name: cancelText.text
            onClicked: root.close()
            background: Rectangle {
                radius: 20
                color: cancelButton.hovered ? Theme.hoverRow : "transparent"
                border.width: cancelButton.visualFocus ? 2 : 1
                border.color: cancelButton.visualFocus ? Theme.primary : Theme.border
            }
            contentItem: Label {
                id: cancelText
                text: backend.profilePhotoSaving ? qsTr("Close") : qsTr("Cancel")
                color: Theme.text
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
        }
        AbstractButton {
            id: saveButton
            objectName: "profilePhotoSaveButton"
            implicitWidth: saveText.implicitWidth + 36
            implicitHeight: 40
            enabled: !backend.profilePhotoSaving && !backend.profilePhotoPreparing
                && backend.status.connected === true && (root.removing || backend.profilePhotoPreview !== "")
            Accessible.name: root.removing ? qsTr("Remove profile photo") : qsTr("Save profile photo")
            onClicked: backend.saveProfilePhoto(root.removing)
            background: Rectangle {
                radius: 20
                color: !saveButton.enabled ? Theme.surfaceMuted : root.removing ? Theme.danger : Theme.primary
                border.width: saveButton.visualFocus ? 2 : 0
                border.color: Theme.text
            }
            contentItem: Label {
                id: saveText
                text: root.removing ? qsTr("Remove") : qsTr("Save")
                color: saveButton.enabled ? Theme.primaryText : Theme.textMuted
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
        }
    }
}
