import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// Reports a problem to the WhatsAppGo project in the Jabali Bugs Intake.
//
// The environment block is shown before anything is sent, not hidden behind a
// disclosure: the user can review exactly what leaves their device.
WhatsAppDialog {
    id: root

    property string subject: ""
    property string details: ""
    property string feedback: ""
    property bool sending: false
    property bool authenticatedAvailable: backend.bugReportAuthenticated
    property var openPublicForm: function () {
        return backend.openPublicBugReport();
    }

    title: qsTr("Report a problem")
    subtitle: root.authenticatedAvailable ? qsTr("Sends a report to the whatsappgo project at bugs.jabali-panel.com.") : qsTr("Report a bug at bugs.jabali-panel.com. No account or intake key is needed.")
    acceptText: !root.authenticatedAvailable ? qsTr("Open report form") : root.sending ? qsTr("Sending…") : qsTr("Send report")
    acceptName: "bugReportSendButton"
    acceptEnabled: !root.sending && (!root.authenticatedAvailable || (Boolean(root.subject.trim()) && Boolean(root.details.trim())))
    showCancel: !root.sending
    closePolicy: root.sending ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside
    preferredWidth: 520
    preferredHeight: root.authenticatedAvailable ? 680 : backend.bugReportEnvironment ? 500 : 340
    objectName: "bugReportDialog"

    signal submitRequested(string subject, string details)

    function reset() {
        subject = "";
        details = "";
        feedback = "";
        sending = false;
    }

    function finish(success, message) {
        sending = false;
        feedback = message;
        if (success) {
            subject = "";
            details = "";
            close();
        }
    }

    // The generic dialog closes immediately on accept. A report must stay
    // open until the server confirms success, retaining text on failure.
    function accept() {
        if (!acceptEnabled)
            return;
        if (!authenticatedAvailable) {
            openPublicReport();
            return;
        }
        sending = true;
        feedback = "";
        submitRequested(subject.trim(), details.trim());
    }

    function openPublicReport() {
        if (sending)
            return;
        if (openPublicForm())
            close();
        else
            feedback = qsTr("Could not open your browser. Visit https://bugs.jabali-panel.com/report and choose whatsappgo.");
    }
    onRejected: reset()

    ScrollView {
        id: formScroll
        Layout.fillWidth: true
        Layout.fillHeight: true
        contentWidth: availableWidth
        clip: true

        ColumnLayout {
            width: formScroll.availableWidth
            spacing: 10

            Label {
                objectName: "bugReportPublicInstructions"
                Layout.fillWidth: true
                visible: !root.authenticatedAvailable
                text: qsTr("In the browser, choose whatsappgo as the program, describe the problem, and complete the security check. Nothing is submitted until you send the web form.")
                textFormat: Text.PlainText
                color: Theme.text
                font.pixelSize: 14
                wrapMode: Text.Wrap
            }

            Label {
                Layout.fillWidth: true
                visible: root.authenticatedAvailable
                text: qsTr("Subject (required)")
                color: Theme.text
                font.pixelSize: 14
            }
            DialogTextField {
                objectName: "bugReportSubjectField"
                Layout.fillWidth: true
                visible: root.authenticatedAvailable
                Accessible.name: qsTr("Subject (required)")
                placeholderText: qsTr("Subject")
                text: root.subject
                enabled: !root.sending
                onTextChanged: if (root.subject !== text)
                    root.subject = text
            }

            Label {
                Layout.fillWidth: true
                visible: root.authenticatedAvailable
                text: qsTr("Description (required)")
                color: Theme.text
                font.pixelSize: 14
            }

            Rectangle {
                Layout.fillWidth: true
                visible: root.authenticatedAvailable
                Layout.preferredHeight: 150
                radius: 8
                color: Theme.surfaceMuted
                border.width: detailsArea.activeFocus ? 1 : 0
                border.color: Theme.primary

                ScrollView {
                    anchors.fill: parent
                    anchors.margins: 8
                    clip: true

                    TextArea {
                        id: detailsArea
                        objectName: "bugReportDetailsField"
                        Accessible.name: qsTr("Description (required)")
                        placeholderText: qsTr("What happened, and what did you expect instead?")
                        placeholderTextColor: Theme.textMuted
                        color: Theme.text
                        font.pixelSize: 14
                        wrapMode: TextArea.Wrap
                        selectByMouse: true
                        enabled: !root.sending
                        background: null
                        text: root.details
                        onTextChanged: if (root.details !== text)
                            root.details = text
                    }
                }
            }

            Label {
                Layout.fillWidth: true
                visible: root.authenticatedAvailable || Boolean(backend.bugReportEnvironment)
                text: root.authenticatedAvailable ? qsTr("Sent with your report:") : qsTr("Optional technical details to copy into your report:")
                color: Theme.textMuted
                font.pixelSize: 12
            }

            Rectangle {
                Layout.fillWidth: true
                visible: root.authenticatedAvailable || Boolean(backend.bugReportEnvironment)
                Layout.preferredHeight: environmentText.implicitHeight + 16
                radius: 8
                color: Theme.surfaceMuted

                Label {
                    id: environmentText
                    objectName: "bugReportEnvironmentText"
                    anchors.fill: parent
                    anchors.margins: 8
                    text: backend.bugReportEnvironment || qsTr("Collecting…")
                    textFormat: Text.PlainText
                    color: Theme.textMuted
                    font.pixelSize: 12
                    font.family: "monospace"
                    wrapMode: Text.Wrap
                }
            }

            Button {
                objectName: "bugReportCopyEnvironmentButton"
                text: qsTr("Copy technical details")
                visible: !root.authenticatedAvailable && Boolean(backend.bugReportEnvironment)
                enabled: Boolean(backend.bugReportEnvironment)
                onClicked: {
                    backend.copyText(backend.bugReportEnvironment);
                    root.feedback = qsTr("Technical details copied. Paste them into the web form if you want to include them.");
                }
            }

            Label {
                Layout.fillWidth: true
                text: root.authenticatedAvailable ? qsTr("Do not include passwords, tokens or private conversations. No chat logs or screenshots are attached automatically.") : qsTr("No app data is uploaded automatically. The website records your report, IP address and browser type. Do not include passwords, tokens or private conversations.")
                color: Theme.textMuted
                font.pixelSize: 12
                wrapMode: Text.Wrap
            }

            Button {
                objectName: "bugReportPublicFormButton"
                text: qsTr("Use the public web form instead")
                visible: root.authenticatedAvailable
                enabled: !root.sending
                onClicked: root.openPublicReport()
            }

        }
    }

    // Keep errors beside the actions, even when the form body is scrolled.
    Label {
        objectName: "bugReportFeedback"
        Layout.fillWidth: true
        visible: Boolean(root.feedback)
        text: root.feedback
        textFormat: Text.PlainText
        color: Theme.textMuted
        font.pixelSize: 12
        wrapMode: Text.Wrap
    }
}
