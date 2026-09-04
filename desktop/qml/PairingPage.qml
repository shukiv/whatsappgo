import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Item {
    id: page

    // Linking is a dead end without a way back: an account that is signed out,
    // or one that was only just added, would otherwise trap the window on this
    // screen. The header offers the other accounts and a way back to the one
    // that was open before.
    property string previousProfile: ""
    // Only offered when there is somewhere real to go: a profile that still
    // exists, and is not the one being linked.
    readonly property bool canGoBack: String(page.previousProfile || "") !== ""
        && String(page.previousProfile) !== backend.profile
        && backend.profiles.indexOf(String(page.previousProfile)) >= 0
    signal switchRequested(string profile)
    signal renameRequested(string profile)
    signal removeRequested(string profile)

    RowLayout {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.margins: 12
        spacing: 8

        ThemedToolButton {
            objectName: "pairingBackButton"
            Layout.preferredWidth: 40
            Layout.preferredHeight: 40
            visible: page.canGoBack
            iconSource: Qt.resolvedUrl("icons/back.svg")
            iconSize: 20
            Accessible.name: qsTr("Back to the previous account")
            onClicked: page.switchRequested(page.previousProfile)
            background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
        }

        Item { Layout.fillWidth: true }

        AccountSwitcherButton {
            objectName: "pairingAccountSwitcher"
            Layout.preferredWidth: 40
            Layout.preferredHeight: 40
            profiles: backend.profiles
            currentProfile: backend.profile
            displayNames: backend.profileDisplayNames
            unreadCounts: backend.profileUnreadCounts
            onSwitchRequested: profile => page.switchRequested(profile)
            onRenameRequested: profile => page.renameRequested(profile)
            onRemoveRequested: profile => page.removeRequested(profile)
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
        }
    }

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
        AbstractButton {
            id: refreshButton
            objectName: "pairingRefreshButton"
            Layout.alignment: Qt.AlignHCenter
            implicitWidth: refreshLabel.implicitWidth + 40
            implicitHeight: 38
            Accessible.name: refreshLabel.text
            onClicked: backend.startPairing()
            background: Rectangle {
                radius: 19
                color: refreshButton.hovered ? Qt.darker(Theme.primary, 1.1) : Theme.primary
            }
            contentItem: Label {
                id: refreshLabel
                text: backend.pairingQr ? qsTr("Refresh QR code") : qsTr("Generate QR code")
                color: Theme.primaryText
                font.pixelSize: 14
                font.weight: Font.Medium
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
            }
        }
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 1
            color: Theme.border
        }
        Label {
            Layout.fillWidth: true
            text: qsTr("Or link with your phone number")
            color: Theme.text
            font.weight: Font.DemiBold
        }
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            DialogTextField {
                id: phoneField
                objectName: "pairingPhoneField"
                Layout.fillWidth: true
                placeholderText: qsTr("International number, e.g. 972501234567")
                inputMethodHints: Qt.ImhDialableCharactersOnly
                Accessible.name: qsTr("Phone number in international format")
                onAccepted: if (linkButton.enabled) backend.pairPhone(phoneField.text)
            }
            AbstractButton {
                id: linkButton
                objectName: "pairingCodeButton"
                implicitWidth: linkLabel.implicitWidth + 36
                implicitHeight: 40
                enabled: phoneField.text.length > 6 && !backend.busy
                Accessible.name: linkLabel.text
                onClicked: backend.pairPhone(phoneField.text)
                background: Rectangle {
                    radius: 20
                    color: !linkButton.enabled
                        ? Theme.surfaceMuted
                        : linkButton.hovered ? Qt.darker(Theme.primary, 1.1) : Theme.primary
                }
                contentItem: Label {
                    id: linkLabel
                    text: qsTr("Get code")
                    color: linkButton.enabled ? Theme.primaryText : Theme.textMuted
                    font.pixelSize: 14
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
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
