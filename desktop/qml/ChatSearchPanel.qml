import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// WhatsApp Web searches inside the open conversation from a panel on the right
// of the window, not from a dialog over it. The conversation narrows to make
// room, and each hit is a timestamp above the matched line.
Rectangle {
    id: root

    property string query: ""
    property var results: []
    property bool loading: false
    property string errorText: ""
    property bool showHeader: true
    property bool showChat: false
    property string searchLabel: qsTr("Search this chat")
    property string emptyText: qsTr("Search for messages in this chat")
    readonly property int currentIndex: hitList.currentIndex

    signal closeRequested()
    signal queryEdited(string text)
    signal messageChosen(string messageId)
    signal messageActivated(var message)
    signal retryRequested()
    signal dateRequested()

    function activateResult(index) {
        if (loading || index < 0 || index >= results.length) return
        const item = results[index]
        if (!item.id) return
        hitList.currentIndex = index
        messageChosen(String(item.id))
        messageActivated(item)
    }

    function moveResult(delta) {
        if (loading || results.length === 0) return
        hitList.currentIndex = Math.max(0, Math.min(results.length - 1,
            hitList.currentIndex < 0 ? 0 : hitList.currentIndex + delta))
        hitList.positionViewAtIndex(hitList.currentIndex, ListView.Contain)
    }

    // ListView selects row zero when it adopts a new model. Reset after that
    // adoption so the first Down key selects, rather than skips, the first hit.
    onResultsChanged: Qt.callLater(() => { hitList.currentIndex = -1 })

    color: Theme.surface
    Rectangle {
        anchors.left: parent.left
        width: 1
        height: parent.height
        color: Theme.border
    }

    function focusField() {
        panelField.forceActiveFocus()
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        RowLayout {
            visible: root.showHeader
            Layout.fillWidth: true
            Layout.preferredHeight: 60
            Layout.leftMargin: 12
            Layout.rightMargin: 12
            spacing: 12

            ThemedToolButton {
                objectName: "chatSearchPanelClose"
                Layout.preferredWidth: 36
                Layout.preferredHeight: 36
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconTint: Theme.icon
                iconSize: 18
                Accessible.name: qsTr("Close search")
                onClicked: root.closeRequested()
                background: Item {}
            }
            Label {
                Layout.fillWidth: true
                text: qsTr("Search messages")
                color: Theme.text
                font.pixelSize: 16
            }
            ThemedToolButton {
                objectName: "chatSearchDateButton"
                Layout.preferredWidth: 40
                Layout.preferredHeight: 40
                iconSource: Qt.resolvedUrl("icons/calendar.svg")
                Accessible.name: qsTr("Go to date")
                onClicked: root.dateRequested()
            }
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 40
            Layout.leftMargin: 16
            Layout.rightMargin: 16
            Layout.bottomMargin: 10
            radius: height / 2
            color: Theme.surfaceMuted
            border.width: panelField.activeFocus ? 2 : 0
            border.color: Theme.primary

            TintedIcon {
                anchors.left: parent.left
                anchors.leftMargin: 12
                anchors.verticalCenter: parent.verticalCenter
                width: 16
                height: 16
                source: Qt.resolvedUrl("icons/search.svg")
                tint: Theme.icon
            }
            TextField {
                id: panelField
                objectName: "chatSearchPanelField"
                anchors.fill: parent
                leftPadding: 38
                rightPadding: 36
                placeholderText: qsTr("Search…")
                color: Theme.text
                font.pixelSize: 14
                Accessible.name: root.searchLabel
                text: root.query
                onTextEdited: root.queryEdited(text)
                Keys.onEscapePressed: root.closeRequested()
                Keys.onDownPressed: root.moveResult(1)
                Keys.onUpPressed: root.moveResult(-1)
                onAccepted: root.activateResult(hitList.currentIndex < 0 ? 0 : hitList.currentIndex)
                background: Item {}
            }
            ThemedToolButton {
                objectName: "chatSearchPanelClear"
                anchors.right: parent.right
                anchors.rightMargin: 6
                anchors.verticalCenter: parent.verticalCenter
                visible: panelField.text.length > 0
                width: 26
                height: 26
                iconSource: Qt.resolvedUrl("icons/close.svg")
                iconTint: Theme.icon
                iconSize: 14
                Accessible.name: qsTr("Clear search")
                onClicked: { root.queryEdited(""); panelField.forceActiveFocus() }
                background: Item {}
            }
        }

        Label {
            objectName: "chatSearchPanelHint"
            Layout.fillWidth: true
            Layout.margins: 20
            visible: root.results.length === 0 && !root.loading && root.errorText === ""
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
            color: Theme.textMuted
            font.pixelSize: 13
            // Before anything is typed the panel explains itself rather than
            // claiming there is nothing to find.
            text: root.query.trim().length === 0
                ? root.emptyText
                : qsTr("No messages found")
            Accessible.name: text
        }

        BusyIndicator {
            objectName: "searchResultsBusy"
            Layout.alignment: Qt.AlignHCenter
            visible: root.loading
            running: visible
            Accessible.name: qsTr("Searching messages")
        }
        Label {
            objectName: "searchResultsError"
            Layout.fillWidth: true
            Layout.margins: 16
            visible: root.errorText !== ""
            text: root.errorText
            color: Theme.textMuted
            wrapMode: Text.Wrap
        }
        Button {
            objectName: "searchResultsRetry"
            Layout.alignment: Qt.AlignHCenter
            visible: root.errorText !== "" && !root.loading
            text: qsTr("Try again")
            onClicked: root.retryRequested()
        }
        Label {
            objectName: "searchResultsCount"
            Layout.leftMargin: 16
            visible: !root.loading && root.results.length > 0
            text: qsTr("%n result(s)", "", root.results.length)
            color: Theme.textMuted
            font.pixelSize: 12
        }

        ListView {
            id: hitList
            objectName: "chatSearchPanelResults"
            Layout.fillWidth: true
            Layout.fillHeight: true
            visible: root.results.length > 0
            model: root.results
            clip: true
            reuseItems: true
            boundsBehavior: Flickable.StopAtBounds
            currentIndex: -1
            Keys.onDownPressed: root.moveResult(1)
            Keys.onUpPressed: root.moveResult(-1)
            Keys.onReturnPressed: root.activateResult(currentIndex)
            Keys.onEnterPressed: root.activateResult(currentIndex)
            Keys.onEscapePressed: root.closeRequested()
            ScrollBar.vertical: OverlayScrollBar {}

            delegate: ItemDelegate {
                id: hitRow
                required property var modelData
                required property int index
                width: ListView.view.width
                height: root.showChat ? 96 : 82
                padding: 0
                leftPadding: 16
                rightPadding: 16
                hoverEnabled: true
                // A document is found by its filename, so that is what the row
                // shows when the message carries no text.
                readonly property string hitText: MessageSummary.body(modelData)
                readonly property string senderText: MessageSummary.sender(modelData)
                Accessible.name: senderText + ": " + hitText + ", " + RowTime.label(modelData.timestamp)
                onClicked: root.activateResult(index)
                onActiveFocusChanged: { if (activeFocus) hitList.currentIndex = index }
                Keys.onDownPressed: { root.moveResult(1); hitList.forceActiveFocus() }
                Keys.onUpPressed: { root.moveResult(-1); hitList.forceActiveFocus() }
                background: Rectangle {
                    color: hitRow.hovered || hitList.currentIndex === hitRow.index ? Theme.hoverRow : "transparent"
                    border.width: hitRow.activeFocus ? 1 : 0
                    border.color: Theme.primary
                }
                contentItem: ColumnLayout {
                    spacing: 4
                    Item { Layout.fillHeight: true }
                    Label {
                        visible: root.showChat
                        Layout.fillWidth: true
                        text: String(hitRow.modelData.chat_title || hitRow.modelData.chat_jid || "")
                        textFormat: Text.PlainText
                        color: Theme.text
                        elide: Text.ElideRight
                        font.weight: Font.DemiBold
                    }
                    Label {
                        Layout.fillWidth: true
                        text: (hitRow.senderText ? hitRow.senderText + " · " : "")
                            + RowTime.label(hitRow.modelData.timestamp)
                        textFormat: Text.PlainText
                        elide: Text.ElideRight
                        color: Theme.textMuted
                        font.pixelSize: 12
                    }
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 4
                        ReadReceipt {
                            visible: Boolean(hitRow.modelData.from_me)
                                && String(hitRow.modelData.status || "") !== ""
                            Layout.alignment: Qt.AlignVCenter
                            status: String(hitRow.modelData.status || "")
                        }
                        TintedIcon {
                            visible: String(hitRow.modelData.kind || "") === "document"
                            Layout.preferredWidth: visible ? 15 : 0
                            Layout.preferredHeight: 15
                            Layout.alignment: Qt.AlignVCenter
                            source: Qt.resolvedUrl("icons/document.svg")
                            tint: Theme.textMuted
                        }
                        Label {
                            Layout.fillWidth: true
                            Layout.minimumWidth: 0
                            text: SearchHighlight.markup(hitRow.hitText, root.query, Theme.primary)
                            textFormat: Text.StyledText
                            color: Theme.text
                            font.pixelSize: 14
                            elide: Text.ElideRight
                            maximumLineCount: 1
                        }
                    }
                    Item { Layout.fillHeight: true }
                }
                HoverHandler { cursorShape: Qt.PointingHandCursor }
            }
        }
    }
}
