import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

WhatsAppDialog {
    id: root
    objectName: "messageDateDialog"
    title: qsTr("Go to date")
    preferredWidth: 400
    preferredHeight: 460
    showAccept: false
    cancelText: qsTr("Close")
    property date today: new Date()
    property date month: new Date(today.getFullYear(), today.getMonth(), 1)
    property date focusedDate: today
    readonly property int firstWeekday: Qt.locale().firstDayOfWeek % 7
    readonly property bool canGoForward: month.getFullYear() < today.getFullYear()
        || (month.getFullYear() === today.getFullYear() && month.getMonth() < today.getMonth())
    property string ownerChat: ""
    property string ownerProfile: ""
    signal messageChosen(string chatJid, string messageId)

    function dayStart(date) { return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime() }
    function cellDate(index) {
        const offset = (month.getDay() - firstWeekday + 7) % 7
        return new Date(month.getFullYear(), month.getMonth(), 1 + index - offset)
    }
    function changeMonth(delta) {
        if (delta > 0 && !canGoForward) return
        const next = new Date(month.getFullYear(), month.getMonth() + delta, 1)
        if (next.getFullYear() < 1970) return
        month = next
    }
    function focusDate(date) {
        if (dayStart(date) > dayStart(today) || date.getFullYear() < 1970) return
        focusedDate = date
        month = new Date(date.getFullYear(), date.getMonth(), 1)
        Qt.callLater(() => {
            for (let i = 0; i < days.count; ++i)
                if (dayStart(cellDate(i)) === dayStart(focusedDate)) days.itemAt(i).forceActiveFocus()
        })
    }
    function chooseDate(date) {
        if (backend.dateLookupBusy || !visible || ownerProfile !== backend.profile
                || ownerChat !== String(backend.selectedChat.jid || "")
                || dayStart(date) > dayStart(today) || date.getFullYear() < 1970) return
        focusedDate = date
        backend.findMessageOnDate(dayStart(date), new Date(date.getFullYear(), date.getMonth(), date.getDate() + 1).getTime())
    }
    onOpened: {
        ownerChat = String(backend.selectedChat.jid || "")
        ownerProfile = backend.profile
        today = new Date()
        backend.cancelDateLookup()
        focusDate(today)
    }
    onClosed: backend.cancelDateLookup()
    Connections {
        target: backend
        function onProfileChanged() { root.close() }
        function onSelectedChatChanged() {
            if (root.visible && root.ownerChat !== String(backend.selectedChat.jid || "")) root.close()
        }
        function onMessageDateLocated(chatJid, messageId) {
            if (!root.visible || chatJid !== root.ownerChat || root.ownerProfile !== backend.profile) return
            root.close()
            root.messageChosen(chatJid, messageId)
        }
    }

    ColumnLayout {
        Layout.fillWidth: true
        Layout.fillHeight: true
        RowLayout {
            Layout.fillWidth: true
            ThemedToolButton {
                objectName: "datePreviousMonth"
                iconSource: Qt.resolvedUrl("icons/back.svg")
                Accessible.name: qsTr("Previous month")
                onClicked: root.changeMonth(-1)
            }
            Label {
                Layout.fillWidth: true
                text: Qt.formatDate(root.month, "MMMM yyyy")
                color: Theme.text
                horizontalAlignment: Text.AlignHCenter
            }
            ThemedToolButton {
                objectName: "dateNextMonth"
                enabled: root.canGoForward
                iconSource: Qt.resolvedUrl("icons/chevron-right.svg")
                Accessible.name: qsTr("Next month")
                onClicked: root.changeMonth(1)
            }
        }
        GridLayout {
            Layout.fillWidth: true
            columns: 7
            columnSpacing: 1
            rowSpacing: 1
            Repeater {
                model: 7
                Label {
                    required property int index
                    Layout.fillWidth: true
                    Layout.preferredHeight: 24
                    text: Qt.formatDate(new Date(2026, 0, 4 + (root.firstWeekday + index) % 7), "ddd")
                    font.pixelSize: 11
                    color: Theme.textMuted
                    horizontalAlignment: Text.AlignHCenter
                }
            }
            Repeater {
                id: days
                model: 42
                AbstractButton {
                    id: day
                    required property int index
                    readonly property date date: root.cellDate(index)
                    readonly property bool isToday: root.dayStart(date) === root.dayStart(root.today)
                    objectName: "calendarDay_" + index
                    Layout.fillWidth: true
                    Layout.preferredHeight: 34
                    enabled: date.getFullYear() >= 1970 && root.dayStart(date) <= root.dayStart(root.today)
                    Accessible.name: Qt.formatDate(date, Qt.DefaultLocaleLongDate)
                    Accessible.description: isToday ? qsTr("Today") : ""
                    onClicked: root.chooseDate(date)
                    background: Rectangle {
                        radius: 17
                        color: day.isToday ? Theme.primary : day.hovered ? Theme.hoverRow : "transparent"
                        border.width: day.activeFocus ? 2 : 0
                        border.color: Theme.primary
                    }
                    contentItem: Label {
                        text: day.date.getDate()
                        color: day.isToday ? Theme.primaryText : Theme.text
                        opacity: !day.enabled ? 0.3 : day.date.getMonth() === root.month.getMonth() ? 1 : 0.55
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                    Keys.onPressed: event => {
                        let delta = 0
                        if (event.key === Qt.Key_Left) delta = -1
                        else if (event.key === Qt.Key_Right) delta = 1
                        else if (event.key === Qt.Key_Up) delta = -7
                        else if (event.key === Qt.Key_Down) delta = 7
                        else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
                            root.chooseDate(date); event.accepted = true; return
                        } else return
                        root.focusDate(new Date(date.getFullYear(), date.getMonth(), date.getDate() + delta))
                        event.accepted = true
                    }
                }
            }
        }
        BusyIndicator { Layout.alignment: Qt.AlignHCenter; visible: backend.dateLookupBusy; running: visible; implicitHeight: 28 }
        Label {
            objectName: "dateLookupFeedback"
            Layout.fillWidth: true
            visible: backend.dateLookupError !== ""
            text: backend.dateLookupError
            color: Theme.textMuted
            wrapMode: Text.Wrap
            font.pixelSize: 12
        }
        Item { Layout.fillHeight: true }
    }
}
