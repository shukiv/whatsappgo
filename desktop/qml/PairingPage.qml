import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import org.whatsappgo

Kirigami.Page {
    id: page
    title: qsTr("Link WhatsApp")

    ColumnLayout {
        anchors.centerIn: parent
        width: Math.min(parent.width - 48, 520)
        spacing: 16
        Label {
            Layout.alignment: Qt.AlignHCenter
            text: qsTr("Link your WhatsApp account")
            color: Theme.text
            font.pixelSize: 26
            font.weight: Font.DemiBold
        }
        Label {
            Layout.fillWidth: true
            text: qsTr("On your phone, open WhatsApp → Settings → Linked devices → Link a device.")
            color: Theme.textMuted
            wrapMode: Text.Wrap
            horizontalAlignment: Text.AlignHCenter
        }
        Rectangle {
            Layout.alignment: Qt.AlignHCenter
            Layout.preferredWidth: 304
            Layout.preferredHeight: 304
            color: "white"
            radius: Theme.radiusMedium
            border.color: Theme.border
            Image {
                anchors.fill: parent
                anchors.margins: 12
                source: backend.pairingQr
                fillMode: Image.PreserveAspectFit
                asynchronous: true
                Accessible.name: qsTr("WhatsApp linking QR code")
            }
            BusyIndicator {
                anchors.centerIn: parent
                running: !backend.pairingQr
                visible: running
            }
        }
        Button {
            Layout.alignment: Qt.AlignHCenter
            text: backend.pairingQr ? qsTr("Refresh QR code") : qsTr("Generate QR code")
            icon.name: "view-refresh"
            onClicked: backend.startPairing()
        }
        Kirigami.Separator { Layout.fillWidth: true }
        Label {
            Layout.fillWidth: true
            text: qsTr("Or link with your phone number")
            color: Theme.text
            font.weight: Font.DemiBold
        }
        RowLayout {
            Layout.fillWidth: true
            TextField {
                id: phoneField
                Layout.fillWidth: true
                placeholderText: qsTr("International number, e.g. 972501234567")
                inputMethodHints: Qt.ImhDialableCharactersOnly
                Accessible.name: qsTr("Phone number in international format")
                onAccepted: linkButton.clicked()
            }
            Button {
                id: linkButton
                text: qsTr("Get code")
                enabled: phoneField.text.length > 6 && !backend.busy
                onClicked: backend.pairPhone(phoneField.text)
            }
        }
        Label {
            visible: backend.pairingCode
            Layout.fillWidth: true
            text: qsTr("Enter this code on your phone: %1").arg(backend.pairingCode)
            color: Theme.primary
            font.pixelSize: 20
            font.weight: Font.Bold
            horizontalAlignment: Text.AlignHCenter
            Accessible.role: Accessible.StaticText
        }
        Label {
            Layout.fillWidth: true
            text: qsTr("Unofficial client. Calls are not supported by the underlying protocol.")
            color: Theme.textMuted
            font.pixelSize: 12
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
        }
    }
}
