import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// Reports a problem as a GitHub issue on the project's repository.
//
// The environment block is shown before anything is sent, not hidden behind a
// disclosure: an issue is public, and somebody about to publish should be able
// to read what they are publishing.
WhatsAppDialog {
    id: root

    property string subject: ""
    property string details: ""
    property string feedback: ""
    property bool sending: false

    title: qsTr("Report a problem")
    subtitle: qsTr("This opens a public issue on the project's GitHub repository.")
    acceptText: root.sending ? qsTr("Sending…") : qsTr("Send report")
    acceptName: "bugReportSendButton"
    acceptEnabled: !root.sending
        && Boolean(root.subject.trim())
        && Boolean(root.details.trim())
    preferredWidth: 520
    objectName: "bugReportDialog"

    signal submitRequested(string subject, string details)

    function reset() {
        subject = ""
        details = ""
        feedback = ""
        sending = false
    }

    function finish(success, message) {
        sending = false
        feedback = message
        if (success)
            close()
    }

    onAccepted: {
        if (!acceptEnabled)
            return
        sending = true
        feedback = ""
        submitRequested(subject.trim(), details.trim())
    }
    onRejected: reset()

    ColumnLayout {
        Layout.fillWidth: true
        spacing: 10

        DialogTextField {
            objectName: "bugReportSubjectField"
            Layout.fillWidth: true
            placeholderText: qsTr("Subject")
            text: root.subject
            enabled: !root.sending
            onTextChanged: if (root.subject !== text) root.subject = text
        }

        Rectangle {
            Layout.fillWidth: true
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
                    placeholderText: qsTr("What happened, and what did you expect instead?")
                    placeholderTextColor: Theme.textMuted
                    color: Theme.text
                    font.pixelSize: 14
                    wrapMode: TextArea.Wrap
                    selectByMouse: true
                    enabled: !root.sending
                    background: null
                    text: root.details
                    onTextChanged: if (root.details !== text) root.details = text
                }
            }
        }

        Label {
            Layout.fillWidth: true
            text: qsTr("Sent with your report:")
            color: Theme.textMuted
            font.pixelSize: 12
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: environmentText.implicitHeight + 16
            radius: 8
            color: Theme.surfaceMuted

            Label {
                id: environmentText
                objectName: "bugReportEnvironmentText"
                anchors.fill: parent
                anchors.margins: 8
                text: backend.bugReportEnvironment || qsTr("Collecting…")
                color: Theme.textMuted
                font.pixelSize: 12
                font.family: "monospace"
                wrapMode: Text.Wrap
            }
        }

        Label {
            objectName: "bugReportFeedback"
            Layout.fillWidth: true
            visible: Boolean(root.feedback)
            text: root.feedback
            color: Theme.textMuted
            font.pixelSize: 12
            wrapMode: Text.Wrap
        }
    }
}
