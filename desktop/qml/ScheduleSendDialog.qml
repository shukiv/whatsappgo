import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// Picking a moment for a message that is already written. The quick choices
// cover what people actually pick - later today, tomorrow morning, next week -
// and the two fields underneath are there for everything else.
//
// The dialog says plainly that the app has to be running, because it does: a
// message whose time passes with the computer off goes out at the next start,
// and somebody planning a message for midnight deserves to know that before
// they rely on it.
WhatsAppDialog {
    id: root
    objectName: "scheduleSendDialog"

    // What is being scheduled, shown back to the reader so a dialog opened by
    // accident is obvious.
    property string messageText: ""
    property string chatTitle: ""
    // The moment currently chosen, in milliseconds since the epoch. It is a
    // real rather than an int because an int cannot hold one.
    property real sendAt: 0
    // clock is the present, named so a test can hold it still.
    property var clock: function () { return new Date() }

    signal scheduleRequested(real sendAt)

    title: qsTr("Schedule message")
    acceptText: qsTr("Schedule")
    acceptName: "scheduleConfirmButton"
    acceptEnabled: root.chosenIsFuture && root.messageText.trim() !== ""
    chatScoped: true

    readonly property date chosen: new Date(root.sendAt)
    readonly property bool chosenIsFuture: root.sendAt > root.clock().getTime()

    onAccepted: root.scheduleRequested(root.sendAt)

    function openFor(text, chat) {
        root.messageText = String(text || "")
        root.chatTitle = String(chat || "")
        root.sendAt = root.defaultTime().getTime()
        root.open()
        return true
    }

    // An hour from now, to the minute: a starting point nobody has to correct
    // before the dialog makes sense.
    function defaultTime() {
        const when = new Date(root.clock().getTime() + 60 * 60 * 1000)
        when.setSeconds(0, 0)
        return when
    }

    function atHour(dayOffset, hour) {
        const when = new Date(root.clock().getTime())
        when.setDate(when.getDate() + dayOffset)
        when.setHours(hour, 0, 0, 0)
        return when
    }

    // The one mistake this dialog could make on its own: "tonight at 20:00"
    // chosen at 21:00 means tomorrow, not an hour ago.
    function chooseHour(dayOffset, hour) {
        let when = root.atHour(dayOffset, hour)
        if (when.getTime() <= root.clock().getTime())
            when = root.atHour(dayOffset + 1, hour)
        root.sendAt = when.getTime()
        return root.sendAt
    }

    function applyFields(dateText, timeText) {
        const parts = String(dateText).split("-")
        const clockParts = String(timeText).split(":")
        if (parts.length !== 3 || clockParts.length !== 2)
            return false
        const when = new Date(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2]),
                              Number(clockParts[0]), Number(clockParts[1]), 0, 0)
        if (isNaN(when.getTime()))
            return false
        root.sendAt = when.getTime()
        return true
    }

    function two(value) { return (value < 10 ? "0" : "") + value }
    function dateField() {
        return root.chosen.getFullYear() + "-" + root.two(root.chosen.getMonth() + 1)
            + "-" + root.two(root.chosen.getDate())
    }
    function timeField() {
        return root.two(root.chosen.getHours()) + ":" + root.two(root.chosen.getMinutes())
    }

    Label {
        Layout.fillWidth: true
        visible: root.chatTitle !== ""
        text: qsTr("To %1").arg(root.chatTitle)
        color: Theme.textMuted
        font.pixelSize: 13
        elide: Text.ElideRight
    }

    Rectangle {
        Layout.fillWidth: true
        radius: 8
        color: Theme.surfaceMuted
        visible: root.messageText !== ""
        implicitHeight: preview.implicitHeight + 20
        Label {
            id: preview
            objectName: "scheduleMessagePreview"
            anchors.fill: parent
            anchors.margins: 10
            text: root.messageText
            color: Theme.text
            wrapMode: Text.Wrap
            maximumLineCount: 3
            elide: Text.ElideRight
            font.pixelSize: 13
        }
    }

    Flow {
        Layout.fillWidth: true
        spacing: 8
        Repeater {
            model: [
                {label: qsTr("In an hour"), day: 0, hour: -1},
                {label: qsTr("Tonight, 20:00"), day: 0, hour: 20},
                {label: qsTr("Tomorrow, 09:00"), day: 1, hour: 9},
                {label: qsTr("Next week"), day: 7, hour: 9},
            ]
            delegate: AbstractButton {
                objectName: "schedulePreset" + index
                implicitWidth: presetLabel.implicitWidth + 24
                implicitHeight: 32
                Accessible.name: modelData.label
                onClicked: {
                    if (modelData.hour < 0)
                        root.sendAt = root.defaultTime().getTime()
                    else
                        root.chooseHour(modelData.day, modelData.hour)
                }
                background: Rectangle {
                    radius: 16
                    color: parent.hovered ? Theme.hoverRow : "transparent"
                    border.color: Theme.border
                    border.width: 1
                }
                contentItem: Label {
                    id: presetLabel
                    text: modelData.label
                    color: Theme.text
                    font.pixelSize: 13
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }
    }

    RowLayout {
        Layout.fillWidth: true
        spacing: 8
        TextField {
            id: dateInput
            objectName: "scheduleDateField"
            Layout.fillWidth: true
            text: root.dateField()
            inputMask: "9999-99-99"
            color: Theme.text
            Accessible.name: qsTr("Date, year first")
            onEditingFinished: root.applyFields(text, timeInput.text)
        }
        TextField {
            id: timeInput
            objectName: "scheduleTimeField"
            Layout.preferredWidth: 90
            text: root.timeField()
            inputMask: "99:99"
            color: Theme.text
            Accessible.name: qsTr("Time of day")
            onEditingFinished: root.applyFields(dateInput.text, text)
        }
    }

    Label {
        objectName: "scheduleWhenLabel"
        Layout.fillWidth: true
        text: root.chosenIsFuture
            ? qsTr("Sends %1").arg(Qt.formatDateTime(root.chosen, "dddd d MMMM, HH:mm"))
            : qsTr("Pick a time in the future.")
        color: root.chosenIsFuture ? Theme.text : Theme.danger
        font.pixelSize: 13
        wrapMode: Text.Wrap
    }

    Label {
        Layout.fillWidth: true
        text: qsTr("WhatsAppGo has to be running then. If it is not, the message goes out the next time you open it.")
        color: Theme.textMuted
        font.pixelSize: 12
        wrapMode: Text.Wrap
    }
}
