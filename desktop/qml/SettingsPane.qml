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

    ProfilePhotoDialog { id: profilePhotoDialog }

    // "" is the top level; otherwise the open section. A command-line argument
    // can open one directly, which is what makes each page photographable.
    property string openSection: {
        const allowed = ["profile", "account", "security", "privacy", "default_timer", "status_audience", "chats", "blocked", "notifications", "notification_messages", "notification_groups", "notification_statuses", "notification_calls", "auto_download", "help"]
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
    readonly property alias replaceEmoticons: composerSettings.replaceEmoticons
    readonly property alias showWallpaper: composerSettings.showWallpaper
    readonly property alias photoQuality: composerSettings.photoQuality
    readonly property alias spellChecking: composerSettings.spellChecking
    readonly property alias spellLanguage: composerSettings.spellLanguage
    property var spellingDictionaries: []
    property string spellingError: ""

    Settings {
        id: composerSettings
        category: "composer"
        property bool enterIsSend: true
        property bool replaceEmoticons: true
        property bool showWallpaper: true
        property string photoQuality: "original"
        property bool spellChecking: false
        property string spellLanguage: "en_US"
    }

    signal logoutRequested()
    signal shortcutsRequested()
    signal appearanceRequested()
    signal bugReportRequested()
    signal backRequested()

    function goBack() {
        if (openSection === "blocked") openSection = "privacy"
        else if (openSection === "default_timer" || openSection === "status_audience") openSection = "privacy"
        else if (openSection === "security") openSection = "account"
        else if (openSection === "auto_download") openSection = "chats"
        else if (openSection.indexOf("notification_") === 0) openSection = "notifications"
        else if (openSection !== "") openSection = ""
        else if (settingsSearch.text !== "") settingsSearch.clear()
        else backRequested()
    }
    function explain(title, description) {
        informationDialog.title = title
        informationDialog.subtitle = description
        informationDialog.open()
    }
    readonly property string ownPhone: {
        const jid = String(backend.status.user_jid || "")
        const local = jid.split("@")[0].split(":")[0]
        return jid.endsWith("@s.whatsapp.net") && /^[0-9]+$/.test(local) ? "+" + local : ""
    }
    WhatsAppDialog {
        id: informationDialog
        showCancel: false
    }
    property string pendingTimer: ""
    property bool aboutEdited: false
    WhatsAppDialog {
        id: timerConfirmation
        title: qsTr("Change default message timer?")
        subtitle: qsTr("Applies to new chats on your WhatsApp account. Existing chat timers are unchanged. WhatsAppGo still retains its local history; this does not erase local copies.")
        acceptText: qsTr("Apply")
        acceptEnabled: root.pendingTimer !== "" && !backend.defaultTimerBusy && backend.status.connected === true
        onAccepted: backend.setDefaultMessageTimer(Number(root.pendingTimer))
    }
    WhatsAppDialog {
        id: profileNameDialog
        title: qsTr("Edit profile name")
        subtitle: qsTr("This changes your WhatsApp name, not the local account label.")
        acceptText: qsTr("Save")
        acceptEnabled: backend.status.connected === true && profileNameField.text.trim() !== ""
        onOpened: { profileNameField.text = backend.status.user_name || ""; profileNameField.forceActiveFocus() }
        onAccepted: backend.setProfileName(profileNameField.text)
        DialogTextField {
            id: profileNameField
            objectName: "settingsProfileNameField"
            Layout.fillWidth: true
            maximumLength: 25
            Accessible.name: qsTr("Profile name")
            onAccepted: if (profileNameDialog.acceptEnabled) profileNameDialog.accept()
        }
    }

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
        { key: "last_seen", label: qsTr("Last seen"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "online", label: qsTr("Online"),
          choices: ["all", "match_last_seen"] },
        { key: "profile_photo", label: qsTr("Profile photo"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "about", label: qsTr("About"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "read_receipts", label: qsTr("Read receipts"),
          choices: ["all", "none"] },
        { key: "group_add", label: qsTr("Groups"),
          choices: ["all", "contacts", "contact_blacklist", "none"] },
        { key: "call_add", label: qsTr("Who can call me"), choices: ["all", "known"] },
        { key: "messages", label: qsTr("Who can message me"), choices: ["all", "contacts"] }
    ]

    function choiceLabel(value) {
        switch (String(value)) {
        case "all": return qsTr("Everyone")
        case "contacts": return qsTr("My contacts")
        case "contact_blacklist": return qsTr("My contacts except…")
        case "none": return qsTr("Nobody")
        case "match_last_seen": return qsTr("Same as last seen")
        case "known": return qsTr("Known contacts")
        default: return String(value)
        }
    }

    function refreshSettings() {
        if (!visible || !backend.daemonConnected) return
        if (openSection === "blocked" || openSection === "privacy") backend.refreshBlockedContacts()
        backend.refreshPrivacySettings()
        backend.refreshNotificationSettings()
        backend.refreshLocalSettings()
        if (openSection === "profile") backend.refreshOwnProfile()
        if (openSection === "status_audience") backend.refreshStatusAudience()
    }
    onVisibleChanged: refreshSettings()
    onOpenSectionChanged: {
        if (settingsScroll) settingsScroll.contentY = 0
        pendingTimer = ""
        refreshSettings()
    }
    Connections {
        target: backend
        function onDaemonConnectedChanged() { root.refreshSettings() }
        function onProfileChanged() { root.aboutEdited = false; aboutField.clear(); root.refreshSettings() }
        function onStatusChanged() { root.refreshSettings() }
        function onOwnProfileChanged() {
            const saved = String(backend.ownProfile.about || "")
            if (!root.aboutEdited || aboutField.text.trim() === saved) {
                aboutField.text = saved
                root.aboutEdited = false
            }
        }
        function onDefaultTimerSaved() { root.pendingTimer = "" }
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
                iconSource: Qt.resolvedUrl("icons/back.svg")
                iconSize: 20
                Accessible.name: root.openSection === "" ? qsTr("Back to chats") : qsTr("Back")
                onClicked: root.goBack()
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
        case "security": return qsTr("Security notifications")
        case "privacy": return qsTr("Privacy")
        case "default_timer": return qsTr("Default message timer")
        case "status_audience": return qsTr("Status audience")
        case "chats": return qsTr("Chats")
        case "notifications": return qsTr("Notifications")
        case "notification_messages": return qsTr("Message notifications")
        case "notification_groups": return qsTr("Group notifications")
        case "notification_statuses": return qsTr("Status notifications")
        case "notification_calls": return qsTr("Call notifications")
        case "auto_download": return qsTr("Media auto-download")
        case "blocked": return qsTr("Blocked contacts")
        case "help": return qsTr("Help")
        default: return qsTr("Settings")
        }
    }

    DialogTextField {
        id: settingsSearch
        objectName: "settingsSearch"
        Layout.fillWidth: true
        Layout.margins: 20
        visible: root.openSection === ""
        placeholderText: qsTr("Search settings")
        Accessible.name: qsTr("Search settings")
    }

    Flickable {
        id: settingsScroll
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
                            text: root.ownPhone
                            color: Theme.textMuted
                            font.pixelSize: 14
                        }
                    }
                }

                Repeater {
                    model: [
                        { key: "profile", label: qsTr("Profile"), description: qsTr("Name, phone number and About"), icon: "user.svg" },
                        { key: "account", label: qsTr("Account"), description: qsTr("Security notifications and account information"), icon: "lock.svg" },
                        { key: "privacy", label: qsTr("Privacy"), description: qsTr("Personal information, blocked contacts and disappearing messages"), icon: "shield.svg" },
                        { key: "chats", label: qsTr("Chats"), description: qsTr("Theme, wallpaper and composer preferences"), icon: "chats.svg" },
                        { key: "notifications", label: qsTr("Notifications"), description: qsTr("Messages, groups, previews and sounds"), icon: "bell.svg" },
                        { key: "shortcuts", label: qsTr("Keyboard shortcuts"), description: qsTr("Quick actions"), icon: "edit.svg" },
                        { key: "help", label: qsTr("Help and feedback"), description: qsTr("Updates and reporting a problem"), icon: "info.svg" }
                    ].filter(item => (item.label + " " + item.description).toLowerCase().indexOf(settingsSearch.text.toLowerCase().trim()) >= 0)
                    SettingsRow {
                        required property var modelData
                        objectName: "settingsRow_" + modelData.key
                        Layout.fillWidth: true
                        text: modelData.label
                        description: modelData.description
                        showChevron: true
                        iconSource: Qt.resolvedUrl("icons/" + modelData.icon)
                        onClicked: modelData.key === "shortcuts" ? root.shortcutsRequested() : root.openSection = modelData.key
                    }
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
                Avatar {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.topMargin: 22
                    Layout.bottomMargin: 28
                    diameter: 120
                    Layout.preferredWidth: 120
                    Layout.preferredHeight: 120
                    title: backend.status.user_name || backend.profile
                    source: Theme.fileUrl(backend.ownProfile.avatar_path || "")
                }
                SettingsRow {
                    objectName: "settingsChangeProfilePhoto"
                    Layout.fillWidth: true
                    text: backend.profilePhotoSaving ? qsTr("Saving profile photo…") : qsTr("Change profile photo")
                    iconSource: Qt.resolvedUrl("icons/camera.svg")
                    enabled: backend.status.connected === true && !backend.profilePhotoSaving
                    onClicked: profilePhotoDialog.choosePhoto()
                }
                SettingsRow {
                    objectName: "settingsRemoveProfilePhoto"
                    Layout.fillWidth: true
                    visible: !!backend.ownProfile.avatar_path
                    text: qsTr("Remove profile photo")
                    iconSource: Qt.resolvedUrl("icons/delete.svg")
                    destructive: true
                    enabled: backend.status.connected === true && !backend.profilePhotoSaving && !backend.ownProfileLoading
                    onClicked: profilePhotoDialog.confirmRemoval()
                }
                Label {
                    text: qsTr("Your name")
                    color: Theme.textMuted
                    font.pixelSize: 13
                }
                CopyableInfoText {
                    objectName: "settingsProfileName"
                    Layout.fillWidth: true
                    text: backend.status.user_name || qsTr("Not set")
                    color: Theme.text
                    font.pixelSize: 16
                    copyLabel: qsTr("Copy name")
                    onCopyRequested: value => backend.copyText(value)
                }
                Label {
                    text: qsTr("Photo, name and About changes made here sync to your WhatsApp account.")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                SettingsRow {
                    Layout.fillWidth: true
                    text: qsTr("Edit profile name")
                    iconSource: Qt.resolvedUrl("icons/edit.svg")
                    enabled: backend.status.connected === true
                    onClicked: profileNameDialog.open()
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
                        enabled: backend.status.connected === true && !backend.ownProfileLoading
                        Accessible.name: qsTr("About")
                        onTextEdited: root.aboutEdited = true
                        onAccepted: if (aboutSave.enabled) backend.setAbout(text)
                    }
                    AbstractButton {
                        id: aboutSave
                        objectName: "settingsAboutSaveButton"
                        implicitWidth: aboutSaveLabel.implicitWidth + 36
                        implicitHeight: 40
                        enabled: backend.status.connected === true && !backend.ownProfileLoading && aboutField.text.trim() !== "" && aboutField.text.trim() !== String(backend.ownProfile.about || "")
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
                Label {
                    Layout.fillWidth: true
                    visible: backend.ownProfileLoading || backend.ownProfileError !== ""
                    text: backend.ownProfileLoading ? qsTr("Loading your profile…") : backend.ownProfileError
                    color: backend.ownProfileError !== "" ? Theme.danger : Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                    textFormat: Text.PlainText
                }
                SettingsRow {
                    Layout.fillWidth: true
                    visible: backend.ownProfileError !== ""
                    text: qsTr("Retry loading profile")
                    enabled: backend.status.connected === true && !backend.ownProfileLoading
                    onClicked: backend.refreshOwnProfile()
                }
                Label {
                    Layout.topMargin: 16
                    text: qsTr("Phone number")
                    color: Theme.textMuted
                    font.pixelSize: 13
                }
                CopyableInfoText {
                    Layout.fillWidth: true
                    text: root.ownPhone || qsTr("Not available")
                    copyEnabled: root.ownPhone !== ""
                    font.pixelSize: 16
                    copyLabel: qsTr("Copy phone number")
                    onCopyRequested: value => backend.copyText(value)
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
                CopyableInfoText {
                    objectName: "settingsAccountNumber"
                    Layout.fillWidth: true
                    text: root.ownPhone || qsTr("Not available")
                    copyEnabled: root.ownPhone !== ""
                    color: Theme.text
                    font.pixelSize: 16
                    copyLabel: qsTr("Copy phone number")
                    onCopyRequested: value => backend.copyText(value)
                }
                SettingsRow {
                    Layout.fillWidth: true
                    text: qsTr("Security notifications")
                    description: qsTr("Security-code change alerts on this computer")
                    iconSource: Qt.resolvedUrl("icons/shield.svg")
                    showChevron: true
                    onClicked: root.openSection = "security"
                }
                Repeater {
                    model: [
                        {title: qsTr("Request account information"), icon: "document.svg", detail: qsTr("Request your WhatsApp account report on your phone under Settings → Account → Request account info. This is separate from exporting a chat in WhatsAppGo.")},
                        {title: qsTr("How to delete my account"), icon: "info.svg", detail: qsTr("Account deletion must be done in the official WhatsApp app under Settings → Account → Delete my account. Logging out here only unlinks this device; it does not delete your WhatsApp account.")}
                    ]
                    SettingsRow {
                        required property var modelData
                        Layout.fillWidth: true
                        text: modelData.title
                        description: qsTr("Manage in the official WhatsApp app")
                        iconSource: Qt.resolvedUrl("icons/" + modelData.icon)
                        showChevron: true
                        onClicked: root.explain(modelData.title, modelData.detail)
                    }
                }
                Label {
                    Layout.topMargin: 12
                    Layout.fillWidth: true
                    text: qsTr("Requesting your account information and deleting your account are handled in WhatsApp on your phone.")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "security"
                spacing: 0
                SettingsToggleRow {
                    objectName: "settingsSecurityNotifications"
                    Layout.fillWidth: true
                    text: qsTr("Show security notifications on this computer")
                    description: qsTr("Receive an alert when a contact's security code changes. Muted chats stay quiet. Alerts are silent on Linux; other desktops use their system sound settings. This preference applies only to this account on this computer.")
                    on: backend.notificationSettings.security === true
                    enabled: backend.daemonConnected && !backend.notificationSettingsBusy && backend.notificationSettings.security !== undefined
                    onSwitched: value => backend.setNotificationSetting("security", value)
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    text: qsTr("Security codes can change when someone reinstalls WhatsApp or changes phones. A change does not by itself mean your conversation is compromised. Verify codes with your contact in the official WhatsApp app. Encryption remains enabled regardless of this switch.")
                    color: Theme.textMuted
                    font.pixelSize: 14
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
                        choiceLabels: modelData.key === "read_receipts" ? [qsTr("On"), qsTr("Off")] : modelData.choices.map(root.choiceLabel)
                        value: String(backend.privacySettings[modelData.key] || "")
                        enabled: backend.status.connected === true && value !== ""
                        onChoiceSelected: choice => {
                            if (choice === "contact_blacklist")
                                root.explain(qsTr("Privacy exceptions"), qsTr("Choose the excluded contacts in WhatsApp on your phone. This list cannot yet be edited here; your current choice has not been changed."))
                            else backend.setPrivacySetting(modelData.key, choice)
                        }
                    }
                }

                SettingsRow {
                    Layout.fillWidth: true
                    text: qsTr("Status audience")
                    description: qsTr("Who receives your 24-hour status updates")
                    showChevron: true
                    onClicked: root.openSection = "status_audience"
                }
                SettingsRow {
                    Layout.fillWidth: true
                    text: qsTr("Default message timer")
                    description: qsTr("Choose a timer for new chats")
                    showChevron: true
                    onClicked: root.openSection = "default_timer"
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
                SettingsToggleRow {
                    Layout.fillWidth: true
                    text: qsTr("Block high volumes from unknown accounts")
                    description: qsTr("WhatsApp's advanced protection against unusually high message volumes from unknown accounts. This is not a block on every unknown sender.")
                    on: backend.privacySettings.defense === "on_standard"
                    enabled: backend.status.connected === true && String(backend.privacySettings.defense || "") !== ""
                    onSwitched: value => backend.setPrivacySetting("defense", value ? "on_standard" : "off")
                }
                SettingsToggleRow {
                    objectName: "settingsDisableLinkPreviews"
                    Layout.fillWidth: true
                    text: qsTr("Disable link previews")
                    description: qsTr("Stop new website requests for composer previews and higher-resolution chat previews on this account and computer. Existing cached previews remain visible; requests already running may finish.")
                    on: backend.localSettings.disable_link_previews === true
                    enabled: backend.daemonConnected && !backend.localSettingsBusy && backend.localSettings.disable_link_previews !== undefined
                    onSwitched: value => backend.setLocalSetting("disable_link_previews", value)
                }
                Repeater {
                    model: [
                        {title: qsTr("App lock"), detail: qsTr("WhatsAppGo does not currently have an app-specific password lock. Use your desktop's screen lock to protect this session.")},
                        {title: qsTr("Protect IP address in calls"), detail: qsTr("WhatsAppGo cannot place or answer calls. Configure call privacy in the official WhatsApp app.")}
                    ]
                    SettingsRow {
                        required property var modelData
                        Layout.fillWidth: true
                        text: modelData.title
                        description: qsTr("Availability and details")
                        iconSource: Qt.resolvedUrl("icons/info.svg")
                        showChevron: true
                        onClicked: root.explain(modelData.title, modelData.detail)
                    }
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "default_timer"
                spacing: 0
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    text: qsTr("Your current account default cannot be read by this linked-device library. Choose a duration below and Apply to change it. Per-chat timers remain available in Contact or Group info.")
                    color: Theme.textMuted
                    font.pixelSize: 14
                    wrapMode: Text.Wrap
                }
                SettingsChoiceRow {
                    objectName: "settingsDefaultTimerChoice"
                    Layout.fillWidth: true
                    text: qsTr("New default")
                    emptyValueLabel: qsTr("Choose duration")
                    value: root.pendingTimer
                    choices: ["86400", "604800", "7776000", "0"]
                    choiceLabels: [qsTr("24 hours"), qsTr("7 days"), qsTr("90 days"), qsTr("Off")]
                    enabled: backend.status.connected === true && !backend.defaultTimerBusy
                    onChoiceSelected: value => root.pendingTimer = value
                }
                SettingsRow {
                    Layout.fillWidth: true
                    text: backend.defaultTimerBusy ? qsTr("Applying…") : qsTr("Apply default timer")
                    enabled: root.pendingTimer !== "" && backend.status.connected === true && !backend.defaultTimerBusy
                    onClicked: timerConfirmation.open()
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "status_audience"
                spacing: 0
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    text: backend.statusAudienceLoading ? qsTr("Loading status audience…")
                        : backend.statusAudienceError !== "" ? backend.statusAudienceError
                        : backend.statusAudience.type === "contacts" ? qsTr("My contacts")
                        : backend.statusAudience.type === "blacklist" ? qsTr("My contacts except %1 contacts").arg((backend.statusAudience.jids || []).length)
                        : backend.statusAudience.type === "whitelist" ? qsTr("Only share with %1 contacts").arg((backend.statusAudience.jids || []).length)
                        : qsTr("Connect to WhatsApp to read your status audience.")
                    color: backend.statusAudienceError !== "" ? Theme.danger : Theme.text
                    font.pixelSize: 16
                    wrapMode: Text.Wrap
                    textFormat: Text.PlainText
                }
                SettingsRow {
                    Layout.fillWidth: true
                    text: qsTr("Refresh audience")
                    enabled: backend.status.connected === true && !backend.statusAudienceLoading
                    onClicked: backend.refreshStatusAudience()
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    text: qsTr("This is the default audience returned by WhatsApp for status broadcasts, separate from About visibility. Edit the included or excluded contacts in WhatsApp on your phone. WhatsAppGo uses that audience when posting a status.")
                    color: Theme.textMuted
                    font.pixelSize: 14
                    wrapMode: Text.Wrap
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
                SettingsChoiceRow {
                    Layout.fillWidth: true
                    text: qsTr("Appearance")
                    value: Theme.preferredMode
                    choices: ["system", "light", "dark"]
                    choiceLabels: [qsTr("System default"), qsTr("Light"), qsTr("Dark")]
                    onChoiceSelected: value => Theme.preferredMode = value
                }
                SettingsToggleRow {
                    Layout.fillWidth: true
                    text: qsTr("Chat wallpaper")
                    description: qsTr("Show the doodle background in conversations.")
                    on: composerSettings.showWallpaper
                    onSwitched: value => composerSettings.showWallpaper = value
                }
                SettingsToggleRow {
                    Layout.fillWidth: true
                    text: qsTr("Replace text with emoji")
                    description: qsTr("Convert emoticons such as :-) when you finish a word or send.")
                    on: composerSettings.replaceEmoticons
                    onSwitched: value => composerSettings.replaceEmoticons = value
                }
                SettingsToggleRow {
                    objectName: "settingsRow_enterIsSend"
                    Layout.fillWidth: true
                    text: qsTr("Enter is send")
                    description: qsTr("Enter sends the message. Turn this off to start a new line instead.")
                    on: composerSettings.enterIsSend
                    onSwitched: value => composerSettings.enterIsSend = value
                }
                SettingsChoiceRow {
                    objectName: "settingsPhotoQuality"
                    Layout.fillWidth: true
                    text: qsTr("Photo upload quality")
                    value: composerSettings.photoQuality
                    choices: ["standard", "hd", "original"]
                    choiceLabels: [qsTr("Standard"), qsTr("HD"), qsTr("Original")]
                    onChoiceSelected: value => composerSettings.photoQuality = value
                }
                Label {
                    Layout.fillWidth: true
                    Layout.leftMargin: 22
                    Layout.rightMargin: 22
                    text: qsTr("Standard resizes JPEG/PNG photos to at most 1600 px; HD to 3840 px. Converted photos use JPEG with a white background. Original sends unchanged bytes, including metadata. Applies to this desktop, including pasted images. Documents, animations and videos stay unchanged.")
                    color: Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                }
                SettingsToggleRow {
                    objectName: "settingsSpellChecking"
                    Layout.fillWidth: true
                    text: qsTr("Spell check")
                    description: qsTr("Underline misspelled words as you type. Right-click a word for suggestions. Uses local Aspell dictionaries; your draft is never sent to a spelling service.")
                    on: composerSettings.spellChecking
                    enabled: root.spellingDictionaries.length > 0
                    onSwitched: value => composerSettings.spellChecking = value
                }
                SettingsChoiceRow {
                    Layout.fillWidth: true
                    text: qsTr("Spelling dictionary")
                    value: composerSettings.spellLanguage
                    choices: root.spellingDictionaries
                    choiceLabels: root.spellingDictionaries
                    enabled: root.spellingDictionaries.length > 0
                    onChoiceSelected: value => composerSettings.spellLanguage = value
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    visible: root.spellingError !== ""
                    text: root.spellingError
                    textFormat: Text.PlainText
                    color: Theme.textMuted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                }
                SettingsRow {
                    Layout.fillWidth: true
                    text: qsTr("Media auto-download")
                    description: qsTr("Choose which attachment types download automatically")
                    showChevron: true
                    onClicked: root.openSection = "auto_download"
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "auto_download"
                spacing: 0
                Repeater {
                    model: [
                        {key: "image", label: qsTr("Photos")}, {key: "video", label: qsTr("Videos and GIFs")},
                        {key: "audio", label: qsTr("Audio and voice messages")}, {key: "document", label: qsTr("Documents")},
                        {key: "sticker", label: qsTr("Stickers")}
                    ]
                    SettingsToggleRow {
                        required property var modelData
                        readonly property string preference: "download_" + modelData.key
                        objectName: "settingsAutoDownload_" + modelData.key
                        Layout.fillWidth: true
                        text: modelData.label
                        on: backend.localSettings[preference] === true
                        enabled: backend.daemonConnected && !backend.localSettingsBusy && backend.localSettings[preference] !== undefined
                        onSwitched: value => backend.setLocalSetting(preference, value)
                    }
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    text: qsTr("Applies to incoming attachments and background history caching. Already downloaded files are kept. You can still download any attachment by clicking it. Transfers already running may finish; files over 50 MiB require a manual download.")
                    font.pixelSize: 13
                    color: Theme.textMuted
                    wrapMode: Text.Wrap
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                visible: root.openSection === "notifications"
                spacing: 0
                Repeater {
                    model: [
                        {key: "messages", label: qsTr("Messages"), icon: "chats.svg"},
                        {key: "groups", label: qsTr("Groups"), icon: "group-add.svg"},
                        {key: "statuses", label: qsTr("Status"), icon: "status.svg"},
                        {key: "calls", label: qsTr("Calls"), icon: "phone.svg"}
                    ]
                    SettingsRow {
                        required property var modelData
                        Layout.fillWidth: true
                        text: modelData.label
                        description: backend.notificationSettings[modelData.key] === undefined ? qsTr("Loading…")
                            : backend.notificationSettings[modelData.key] ? qsTr("On") : qsTr("Off")
                        iconSource: Qt.resolvedUrl("icons/" + modelData.icon)
                        showChevron: true
                        onClicked: root.openSection = "notification_" + modelData.key
                    }
                }
                Repeater {
                    model: [
                        {key: "previews", label: qsTr("Show message previews"), detail: qsTr("Include message text in desktop notifications. Sender names remain visible.")},
                        {key: "sounds", label: qsTr("Allow incoming sounds"), detail: Qt.platform.os === "linux" ? qsTr("Master switch for incoming alert sounds. Choose sounds separately under Messages, Groups, Status and Calls. Desktop sound settings also apply.") : qsTr("Manage notification sounds in your operating system settings.")},
                        {key: "outgoing_sound", label: qsTr("Play a sound when sending messages"), detail: qsTr("Play the desktop message sound after a text, attachment or forwarded message sends successfully.")}
                    ]
                    SettingsToggleRow {
                        required property var modelData
                        Layout.fillWidth: true
                        text: modelData.label
                        description: modelData.detail
                        on: backend.notificationSettings[modelData.key] === true
                        enabled: backend.daemonConnected && !backend.notificationSettingsBusy
                            && backend.notificationSettings[modelData.key] !== undefined
                            && (modelData.key === "previews" || Qt.platform.os === "linux")
                        onSwitched: value => backend.setNotificationSetting(modelData.key, value)
                    }
                }
                Repeater {
                    model: [
                        {title: qsTr("Background sync"), detail: qsTr("Linked profiles keep syncing while WhatsAppGo is running, including when its window is hidden. Quitting the app stops its background services. No separate background-sync switch is required.")}
                    ]
                    SettingsRow {
                        required property var modelData
                        Layout.fillWidth: true
                        text: modelData.title
                        iconSource: Qt.resolvedUrl("icons/info.svg")
                        showChevron: true
                        onClicked: root.explain(modelData.title, modelData.detail)
                    }
                }
                SettingsRow {
                    objectName: "settingsTestIncomingSound"
                    Layout.fillWidth: true
                    text: qsTr("Test incoming sound")
                    description: qsTr("Preview the desktop sound without sending a message.")
                    enabled: backend.daemonConnected && Qt.platform.os === "linux"
                    onClicked: backend.testNotificationSound("incoming")
                }
                SettingsRow {
                    objectName: "settingsTestOutgoingSound"
                    Layout.fillWidth: true
                    text: qsTr("Test outgoing sound")
                    enabled: backend.daemonConnected && Qt.platform.os === "linux"
                    onClicked: backend.testNotificationSound("outgoing")
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    text: qsTr("Saved for this account on this computer. Muted chats remain muted. Your desktop must also allow notifications for WhatsAppGo.")
                    font.pixelSize: 13
                    color: Theme.textMuted
                    wrapMode: Text.Wrap
                }
            }
            ColumnLayout {
                id: categoryNotifications
                Layout.fillWidth: true
                visible: root.openSection.indexOf("notification_") === 0
                readonly property string category: root.openSection.slice("notification_".length)
                spacing: 0
                Repeater {
                    model: [
                        {suffix: "", label: qsTr("Show notifications"), detail: qsTr("Show desktop alerts for this category.")},
                        {suffix: "_reactions", label: qsTr("Show reaction notifications"), detail: qsTr("Notify when someone reacts to a message you sent. Removed reactions and history sync stay quiet.")},
                        {suffix: "_sound", label: qsTr("Play sound"), detail: qsTr("Play a sound with alerts in this category. The incoming-sound master switch must also be on.")}
                    ]
                    SettingsToggleRow {
                        required property var modelData
                        readonly property string preference: categoryNotifications.category + modelData.suffix
                        objectName: "settingsNotification_" + preference
                        Layout.fillWidth: true
                        visible: modelData.suffix !== "_reactions" || ["messages", "groups"].indexOf(categoryNotifications.category) >= 0
                        text: modelData.label
                        description: modelData.detail
                        on: backend.notificationSettings[preference] === true
                        enabled: backend.daemonConnected && !backend.notificationSettingsBusy
                            && backend.notificationSettings[preference] !== undefined
                            && (modelData.suffix !== "_sound" || Qt.platform.os === "linux")
                        onSwitched: value => backend.setNotificationSetting(preference, value)
                    }
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    visible: categoryNotifications.category === "statuses"
                    text: qsTr("Optional alerts for newly received status updates. Muted contacts, your own updates and history sync stay quiet. Message-preview settings also apply. Clicking an alert opens Status. Likes and mentions are not supported yet.")
                    font.pixelSize: 13
                    color: Theme.textMuted
                    wrapMode: Text.Wrap
                }
                Label {
                    Layout.fillWidth: true
                    Layout.margins: 22
                    visible: categoryNotifications.category === "calls"
                    text: qsTr("Receive a one-time alert when WhatsApp delivers an incoming call event. Answer on your phone; voice/video calling is not implemented in WhatsAppGo.")
                    font.pixelSize: 13
                    color: Theme.textMuted
                    wrapMode: Text.Wrap
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
