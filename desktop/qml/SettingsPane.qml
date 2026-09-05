import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtCore
import org.whatsappgo

// WhatsApp Web's settings tree: a list of sections that drills into one at a
// time. Only the settings this client can actually carry out are offered; a row
// that would do nothing is worse than no row.
ColumnLayout {
    id: root
    spacing: 0

    // "" is the top level; otherwise the open section. A command-line argument
    // can open one directly, which is what makes each page photographable.
    property string openSection: {
        const allowed = ["profile", "account", "privacy", "chats", "blocked", "help"]
        const args = Qt.application.arguments
        for (let i = 0; i < args.length - 1; ++i) {
            if (args[i] === "--settings-section" && allowed.indexOf(args[i + 1]) >= 0)
                return args[i + 1]
        }
        return ""
    }

    // Local preferences live with the pane that owns them; the composer reads
    // them through this pane rather than keeping a second copy.
    readonly property alias enterIsSend: composerSettings.enterIsSend

    Settings {
        id: composerSettings
        category: "composer"
        property bool enterIsSend: true
    }

    signal logoutRequested()
    signal shortcutsRequested()
    signal appearanceRequested()
    signal bugReportRequested()

    // A build that was not stamped with a release version is somebody's own,
    // and it is never behind a release - the daemon compares the same way. A
    // working copy is stamped with its commit instead, which is worth showing
    // but is not a version to compare. Saying so is more use than "up to date".
    readonly property bool stampedBuild:
        /^v?\d+\.\d+(\.\d+)?([-+].*)?$/.test(String(backend.updateStatus.current || ""))
    readonly property bool updateBusy: backend.updateStatus.downloading === true
    readonly property bool updateReady: String(backend.updateStatus.downloaded || "") !== ""
    readonly property string versionText: {
        const current = String(backend.updateStatus.current || "")
        if (root.stampedBuild)
            return qsTr("WhatsAppGo %1").arg(current)
        return current === "" || current === "dev"
            ? qsTr("WhatsAppGo, built from source")
            : qsTr("WhatsAppGo, built from source (%1)").arg(current)
    }
    readonly property string updateText: {
        const status = backend.updateStatus
        if (String(status.error || "") !== "")
            return String(status.error)
        if (root.updateBusy)
            return qsTr("Downloading version %1…").arg(String(status.latest || ""))
        if (root.updateReady)
            return qsTr("Version %1 is downloaded and ready to install.").arg(String(status.latest || ""))
        if (status.available === true)
            return qsTr("Version %1 is out.").arg(String(status.latest || ""))
        if (!root.stampedBuild)
            return qsTr("A build from source is not compared against the releases.")
        if (String(status.checked_at || "") === "")
            return qsTr("Not checked yet.")
        return qsTr("This is the newest release.")
    }
    readonly property string updateButtonText: {
        if (root.updateBusy)
            return qsTr("Downloading…")
        if (root.updateReady)
            return qsTr("Install and restart")
        if (backend.updateStatus.available === true)
            return backend.updateInstallable() ? qsTr("Download the update") : qsTr("Open the release page")
        return qsTr("Check for updates")
    }

    function updateAction() {
        if (root.updateReady) {
            backend.installUpdate()
            return
        }
        if (backend.updateStatus.available === true) {
            if (backend.updateInstallable())
                backend.downloadUpdate()
            else
                backend.openReleasePage()
            return
        }
        backend.checkForUpdates()
    }

    readonly property var privacyRows: [
        { key: "last_seen", label: qsTr("Last seen and online"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "profile_photo", label: qsTr("Profile photo"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "status", label: qsTr("Status"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "read_receipts", label: qsTr("Read receipts"),
          choices: ["all", "none"] },
        { key: "group_add", label: qsTr("Groups"),
          choices: ["all", "contacts", "contact_blacklist", "none"] }
    ]

    function choiceLabel(value) {
        switch (String(value)) {
        case "all": return qsTr("Everyone")
        case "contacts": return qsTr("My contacts")
        case "contact_blacklist": return qsTr("My contacts except…")
        case "none": return qsTr("Nobody")
        case "match_last_seen": return qsTr("Same as last seen")
        default: return String(value)
        }
    }

    Component.onCompleted: {
        backend.refreshPrivacySettings()
        backend.refreshBlockedContacts()
    }

    Rectangle {
        Layout.fillWidth: true
        Layout.preferredHeight: 64
        color: Theme.surface
        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 16
            anchors.rightMargin: 16
            spacing: 12
            ThemedToolButton {
                objectName: "settingsBackButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                visible: root.openSection !== ""
                iconSource: Qt.resolvedUrl("icons/back.svg")
                iconSize: 20
                Accessible.name: qsTr("Back to settings")
                onClicked: root.openSection = ""
                background: Rectangle { radius: 20; color: parent.hovered ? Theme.hoverRow : "transparent" }
            }
            Label {
                objectName: "settingsTitle"
                Layout.fillWidth: true
                text: root.openSection === "" ? qsTr("Settings") : root.sectionTitle(root.openSection)
                color: Theme.text
                font.pixelSize: 19
                font.weight: Font.Medium
                elide: Text.ElideRight
            }
        }
    }

    function sectionTitle(key) {
        switch (key) {
        case "profile": return qsTr("Profile")
        case "account": return qsTr("Account")
        case "privacy": return qsTr("Privacy")
        case "chats": return qsTr("Chats")
        case "blocked": return qsTr("Blocked contacts")
        case "help": return qsTr("Help")
        default: return qsTr("Settings")
        }
    }

    Flickable {
        Layout.fillWidth: true
        Layout.fillHeight: true
        clip: true
        contentWidth: width
        contentHeight: pages.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: OverlayScrollBar {}

        ColumnLayout {
            id: pages
            width: parent.width
            spacing: 0

            // Top level
            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === ""
                spacing: 0

                RowLayout {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    spacing: 16
                    Avatar {
                        Layout.preferredWidth: 64
                        Layout.preferredHeight: 64
                        diameter: 64
                        title: backend.status.user_name || backend.profile
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 2
                        Label {
                            objectName: "settingsAccountName"
                            text: backend.status.user_name || backend.profile
                            color: Theme.text
                            font.pixelSize: 17
                            font.weight: Font.Medium
                        }
                        Label {
                            text: String(backend.status.user_jid || "").split("@")[0].split(":")[0]
                            color: Theme.textMuted
                            font.pixelSize: 14
                        }
                    }
                }

                Repeater {
                    model: [
                        { key: "profile", label: qsTr("Profile"), icon: "user.svg" },
                        { key: "account", label: qsTr("Account"), icon: "lock.svg" },
                        { key: "privacy", label: qsTr("Privacy"), icon: "block.svg" },
                        { key: "chats", label: qsTr("Chats"), icon: "chats.svg" },
                        { key: "help", label: qsTr("Help"), icon: "info.svg" }
                    ]
                    SettingsRow {
                        required property var modelData
                        objectName: "settingsRow_" + modelData.key
                        Layout.fillWidth: true
                        text: modelData.label
                        iconSource: Qt.resolvedUrl("icons/" + modelData.icon)
                        onClicked: root.openSection = modelData.key
                    }
                }
                SettingsRow {
                    objectName: "settingsRow_shortcuts"
                    Layout.fillWidth: true
                    text: qsTr("Keyboard shortcuts")
                    iconSource: Qt.resolvedUrl("icons/edit.svg")
                    onClicked: root.shortcutsRequested()
                }
                SettingsRow {
                    objectName: "settingsRow_logout"
                    Layout.fillWidth: true
                    text: qsTr("Log out")
                    destructive: true
                    iconSource: Qt.resolvedUrl("icons/logout.svg")
                    onClicked: root.logoutRequested()
                }
            }

            // Profile
            ColumnLayout {
                Layout.fillWidth: true
                Layout.margins: 22
                visible: root.openSection === "profile"
                spacing: 12
                Label {
                    text: qsTr("Your name")
                    color: Theme.textMuted
                    font.pixelSize: 13
                }
                Label {
                    objectName: "settingsProfileName"
                    text: backend.status.user_name || qsTr("Not set")
                    color: Theme.text
                    font.pixelSize: 16
                }
                Label {
                    // The name belongs to the phone; nothing here can change it,
                    // and pretending otherwise would be worse than saying so.
                    text: qsTr("Your name comes from your phone. Change it there and it follows here.")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                Rectangle {
                    Layout.fillWidth: true
                    Layout.topMargin: 8
                    Layout.preferredHeight: 1
                    color: Theme.border
                }
                Label {
                    Layout.topMargin: 8
                    text: qsTr("About")
                    color: Theme.textMuted
                    font.pixelSize: 13
                }
                RowLayout {
                    Layout.fillWidth: true
                    spacing: 8
                    DialogTextField {
                        id: aboutField
                        objectName: "settingsAboutField"
                        Layout.fillWidth: true
                        maximumLength: 139
                        placeholderText: qsTr("Tell people about yourself")
                        onAccepted: if (text.trim()) backend.setAbout(text)
                    }
                    AbstractButton {
                        id: aboutSave
                        objectName: "settingsAboutSaveButton"
                        implicitWidth: aboutSaveLabel.implicitWidth + 36
                        implicitHeight: 40
                        enabled: aboutField.text.trim() !== ""
                        Accessible.name: aboutSaveLabel.text
                        onClicked: backend.setAbout(aboutField.text)
                        background: Rectangle {
                            radius: 20
                            color: !aboutSave.enabled ? Theme.surfaceMuted
                                : aboutSave.hovered ? Qt.darker(Theme.primary, 1.1) : Theme.primary
                        }
                        contentItem: Label {
                            id: aboutSaveLabel
                            text: qsTr("Save")
                            color: aboutSave.enabled ? Theme.primaryText : Theme.textMuted
                            font.pixelSize: 14
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                        }
                    }
                }
            }

            // Account
            ColumnLayout {
                Layout.fillWidth: true
                Layout.margins: 22
                visible: root.openSection === "account"
                spacing: 8
                Label {
                    text: qsTr("Phone number")
                    color: Theme.textMuted
                    font.pixelSize: 13
                }
                Label {
                    objectName: "settingsAccountNumber"
                    text: "+" + String(backend.status.user_jid || "").split("@")[0].split(":")[0]
                    color: Theme.text
                    font.pixelSize: 16
                }
                Label {
                    Layout.topMargin: 12
                    Layout.fillWidth: true
                    text: qsTr("Requesting your account information, deleting your account and security notifications are handled in WhatsApp on your phone.")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                }
            }

            // Privacy
            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "privacy"
                spacing: 0

                Repeater {
                    model: root.privacyRows
                    SettingsChoiceRow {
                        required property var modelData
                        objectName: "settingsPrivacy_" + modelData.key
                        Layout.fillWidth: true
                        text: modelData.label
                        choices: modelData.choices
                        choiceLabels: modelData.choices.map(root.choiceLabel)
                        value: String(backend.privacySettings[modelData.key] || "")
                        onChoiceSelected: choice => backend.setPrivacySetting(modelData.key, choice)
                    }
                }

                SettingsRow {
                    objectName: "settingsRow_blocked"
                    Layout.fillWidth: true
                    text: qsTr("Blocked contacts")
                    trailingText: backend.blockedContacts.length > 0
                        ? String(backend.blockedContacts.length) : ""
                    iconSource: Qt.resolvedUrl("icons/block.svg")
                    onClicked: root.openSection = "blocked"
                }
            }

            // Blocked contacts
            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "blocked"
                spacing: 0
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    visible: backend.blockedContacts.length === 0
                    text: qsTr("Nobody is blocked.")
                    color: Theme.textMuted
                    font.pixelSize: 14
                }
                Repeater {
                    model: backend.blockedContacts
                    SettingsRow {
                        required property string modelData
                        objectName: "settingsBlockedRow"
                        Layout.fillWidth: true
                        text: "+" + modelData.split("@")[0]
                        iconSource: Qt.resolvedUrl("icons/user.svg")
                        actionText: qsTr("Unblock")
                        onActionClicked: backend.setContactBlocked(modelData, false)
                    }
                }
            }

            // Chats
            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "chats"
                spacing: 0
                SettingsRow {
                    objectName: "settingsRow_theme"
                    Layout.fillWidth: true
                    text: qsTr("Theme")
                    trailingText: Theme.dark ? qsTr("Dark") : qsTr("Light")
                    iconSource: Qt.resolvedUrl(Theme.dark ? "icons/moon.svg" : "icons/sun.svg")
                    onClicked: root.appearanceRequested()
                }
                SettingsToggleRow {
                    objectName: "settingsRow_enterIsSend"
                    Layout.fillWidth: true
                    text: qsTr("Enter is send")
                    description: qsTr("Enter sends the message. Turn this off to start a new line instead.")
                    on: composerSettings.enterIsSend
                    onSwitched: value => composerSettings.enterIsSend = value
                }

            }

            // Help: what this copy is, and whether a newer one exists.
            ColumnLayout {
                Layout.fillWidth: true
                Layout.margins: 22
                visible: root.openSection === "help"
                spacing: 10

                Label {
                    objectName: "settingsVersionLabel"
                    Layout.fillWidth: true
                    text: root.versionText
                    color: Theme.text
                    font.pixelSize: 15
                    wrapMode: Text.WordWrap
                }
                Label {
                    objectName: "settingsUpdateStatusLabel"
                    Layout.fillWidth: true
                    text: root.updateText
                    color: Theme.textMuted
                    font.pixelSize: 14
                    wrapMode: Text.WordWrap
                }

                RowLayout {
                    Layout.fillWidth: true
                    Layout.topMargin: 4
                    spacing: 8

                    AbstractButton {
                        id: updateButton
                        objectName: "settingsUpdateButton"
                        implicitWidth: updateButtonLabel.implicitWidth + 36
                        implicitHeight: 40
                        enabled: !root.updateBusy
                        Accessible.name: updateButtonLabel.text
                        onClicked: root.updateAction()
                        background: Rectangle {
                            radius: 20
                            color: !updateButton.enabled ? Theme.surfaceMuted
                                : updateButton.hovered ? Qt.darker(Theme.primary, 1.1) : Theme.primary
                        }
                        contentItem: Label {
                            id: updateButtonLabel
                            text: root.updateButtonText
                            color: updateButton.enabled ? Theme.primaryText : Theme.textMuted
                            font.pixelSize: 14
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                        }
                    }

                    AbstractButton {
                        id: releasePageButton
                        objectName: "settingsReleasePageButton"
                        visible: String(backend.updateStatus.url || "") !== ""
                        implicitWidth: releasePageLabel.implicitWidth + 32
                        implicitHeight: 40
                        Accessible.name: releasePageLabel.text
                        onClicked: backend.openReleasePage()
                        background: Rectangle {
                            radius: 20
                            color: releasePageButton.hovered ? Theme.hoverRow : "transparent"
                            border.color: Theme.border
                            border.width: 1
                        }
                        contentItem: Label {
                            id: releasePageLabel
                            text: qsTr("What is new")
                            color: Theme.text
                            font.pixelSize: 14
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                        }
                    }
                }

                SettingsRow {
                    objectName: "settingsRow_reportProblem"
                    Layout.fillWidth: true
                    Layout.topMargin: 8
                    text: qsTr("Report a problem")
                    iconSource: Qt.resolvedUrl("icons/bug.svg")
                    onClicked: root.bugReportRequested()
                }
            }
        }
    }
}
