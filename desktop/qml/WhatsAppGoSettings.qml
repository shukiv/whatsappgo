import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Drawer {
    id: root
    objectName: "whatsappGoSettings"
    parent: Overlay.overlay
    edge: Qt.RightEdge
    width: Math.min(560, parent ? parent.width : 560)
    height: parent ? parent.height : 700
    modal: true
    focus: true
    dragMargin: 0
    padding: 0
    closePolicy: Popup.CloseOnEscape
    property string draftProvider: "none"
    property string statusText: ""
    property bool saveFailed: false
    onDraftProviderChanged: statusText = ""
    readonly property var providers: [
        {name: "GIPHY", value: "giphy", url: "https://developers.giphy.com/"},
        {name: "KLIPY", value: "klipy", url: "https://docs.klipy.com/"},
        {name: "Tenor (retired)", value: "tenor", url: "https://support.google.com/tenor/answer/10455265?hl=en"}
    ]

    onAboutToShow: {
        draftProvider = AppSettings.gifProvider
        statusText = AppSettings.error
        saveFailed = statusText !== ""
        for (let i = 0; i < keyFields.count; ++i) {
            const field = keyFields.itemAt(i)
            field.value = AppSettings.apiKey(providers[i].value)
            field.revealed = false
        }
        settingsScroll.contentItem.contentY = 0
    }
    onClosed: {
        // Do not leave credentials sitting in hidden text fields.
        for (let i = 0; i < keyFields.count; ++i) {
            keyFields.itemAt(i).value = ""
            keyFields.itemAt(i).revealed = false
        }
    }
    function save() {
        const keys = {}
        for (let i = 0; i < keyFields.count; ++i)
            keys[providers[i].value] = keyFields.itemAt(i).value
        saveFailed = !AppSettings.saveGifProviders(draftProvider, keys)
        statusText = saveFailed ? AppSettings.error : qsTr("Settings saved on this computer.")
    }
    background: Rectangle { color: Theme.surface }
    Overlay.modal: Rectangle { color: Theme.dark ? "#99000000" : "#66000000" }

    contentItem: ColumnLayout {
        spacing: 0
        RowLayout {
            Layout.fillWidth: true
            Layout.preferredHeight: 64
            Layout.leftMargin: 20
            Layout.rightMargin: 12
            Label {
                Layout.fillWidth: true
                text: qsTr("WhatsAppGo settings")
                font.pixelSize: 20
                font.weight: Font.Medium
                color: Theme.text
                elide: Text.ElideRight
            }
            ThemedToolButton {
                objectName: "appSettingsCloseButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/close.svg")
                Accessible.name: qsTr("Close settings without saving changes")
                onClicked: root.close()
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
        }
        Rectangle { Layout.fillWidth: true; height: 1; color: Theme.border }

        ScrollView {
            id: settingsScroll
            objectName: "appSettingsScroll"
            Layout.fillWidth: true
            Layout.fillHeight: true
            contentWidth: availableWidth
            clip: true
            ScrollBar.horizontal.policy: ScrollBar.AlwaysOff
            ScrollBar.vertical: OverlayScrollBar {}

            ColumnLayout {
                width: settingsScroll.availableWidth
                spacing: 18
                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.margins: 24
                    spacing: 10
                    Label {
                        text: qsTr("GIF providers")
                        color: Theme.text
                        font.pixelSize: 18
                        font.weight: Font.Medium
                    }
                    Label {
                        Layout.fillWidth: true
                        text: qsTr("Search and send GIFs from the chat emoji picker using your own API key. GIPHY includes trending GIFs; KLIPY includes featured GIFs. Both support keyword search. Saving a key does not verify it.")
                        color: Theme.textMuted
                        font.pixelSize: 14
                        wrapMode: Text.Wrap
                    }
                    Label { text: qsTr("Preferred provider"); color: Theme.text; font.pixelSize: 14 }
                    RowLayout {
                        spacing: 12
                        Repeater {
                            model: [{name: qsTr("None"), value: "none"}, {name: "GIPHY", value: "giphy"}, {name: "KLIPY", value: "klipy"}]
                            DialogRadioButton {
                                required property var modelData
                                objectName: "gifProvider_" + modelData.value
                                text: modelData.name
                                checked: root.draftProvider === modelData.value
                                onClicked: root.draftProvider = modelData.value
                            }
                        }
                    }

                    Repeater {
                        id: keyFields
                        model: root.providers
                        ColumnLayout {
                            required property var modelData
                            property alias value: keyInput.text
                            property bool revealed: false
                            Layout.fillWidth: true
                            Layout.topMargin: 14
                            spacing: 8
                            RowLayout {
                                Layout.fillWidth: true
                                Label {
                                    Layout.fillWidth: true
                                    text: modelData.name + qsTr(" API key")
                                    color: Theme.text
                                    font.pixelSize: 14
                                    font.weight: Font.Medium
                                }
                                Button {
                                    flat: true
                                    text: modelData.value === "tenor" ? qsTr("Retirement notice") : qsTr("Get a key")
                                    palette.buttonText: Theme.link
                                    onClicked: Qt.openUrlExternally(modelData.url)
                                    Accessible.name: text + " — " + modelData.name
                                }
                            }
                            RowLayout {
                                Layout.fillWidth: true
                                DialogTextField {
                                    id: keyInput
                                    objectName: "gifKey_" + modelData.value
                                    Layout.fillWidth: true
                                    Layout.minimumWidth: 0
                                    placeholderText: qsTr("Paste API key")
                                    Accessible.name: modelData.name + qsTr(" API key")
                                    echoMode: revealed ? TextInput.Normal : TextInput.Password
                                    onTextEdited: root.statusText = ""
                                    inputMethodHints: Qt.ImhSensitiveData | Qt.ImhNoPredictiveText | Qt.ImhNoAutoUppercase
                                }
                                Button {
                                    text: revealed ? qsTr("Hide") : qsTr("Show")
                                    flat: true
                                    palette.buttonText: Theme.text
                                    onClicked: revealed = !revealed
                                    Accessible.name: text + " " + modelData.name + qsTr(" API key")
                                }
                            }
                            Label {
                                Layout.fillWidth: true
                                visible: modelData.value === "tenor"
                                text: qsTr("Google retired the Tenor API on June 30, 2026. You can retain a legacy key, but Tenor cannot be selected for GIF search.")
                                color: Theme.textMuted
                                font.pixelSize: 13
                                wrapMode: Text.Wrap
                            }
                        }
                    }
                    Label {
                        Layout.fillWidth: true
                        Layout.topMargin: 14
                        text: qsTr("Keys are stored locally, not encrypted or synced to WhatsApp. On Linux, only your user can read the saved file. Clear a field and press Save to remove its key. These settings apply to all accounts on this computer.")
                        color: Theme.textMuted
                        font.pixelSize: 13
                        wrapMode: Text.Wrap
                    }
                }
            }
        }
        Rectangle { Layout.fillWidth: true; height: 1; color: Theme.border }
        Label {
            objectName: "appSettingsStatus"
            Layout.fillWidth: true
            Layout.leftMargin: 24
            Layout.rightMargin: 24
            Layout.topMargin: visible ? 12 : 0
            visible: root.statusText !== ""
            text: root.statusText
            color: root.saveFailed ? Theme.danger : Theme.primary
            wrapMode: Text.Wrap
            font.pixelSize: 14
        }
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 16
            spacing: 12
            Item { Layout.fillWidth: true }
            Button {
                text: qsTr("Close")
                flat: true
                palette.buttonText: Theme.text
                onClicked: root.close()
            }
            Button {
                id: saveButton
                objectName: "appSettingsSaveButton"
                text: qsTr("Save")
                implicitWidth: 90
                implicitHeight: 40
                onClicked: root.save()
                background: Rectangle { radius: 20; color: saveButton.down ? Qt.darker(Theme.primary, 1.12) : Theme.primary }
                contentItem: Label {
                    text: saveButton.text
                    color: Theme.primaryText
                    font.pixelSize: 14
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }
    }
}
