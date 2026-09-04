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

    signal closeRequested()
    signal queryEdited(string text)
    signal messageChosen(string messageId)

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
                Accessible.name: qsTr("Search this chat")
                text: root.query
                onTextEdited: root.queryEdited(text)
                Keys.onEscapePressed: root.closeRequested()
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
                onClicked: root.queryEdited("")
                background: Item {}
            }
        }

        Label {
            objectName: "chatSearchPanelHint"
            Layout.fillWidth: true
            Layout.margins: 20
            visible: root.results.length === 0
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
            color: Theme.textMuted
            font.pixelSize: 13
            // Before anything is typed the panel explains itself rather than
            // claiming there is nothing to find.
            text: root.query.trim().length === 0
                ? qsTr("Search for messages in this chat")
                : qsTr("No messages found")
            Accessible.name: text
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
            ScrollBar.vertical: OverlayScrollBar {}

            delegate: ItemDelegate {
                id: hitRow
                required property var modelData
                width: ListView.view.width
                height: 72
                padding: 0
                leftPadding: 16
                rightPadding: 16
                hoverEnabled: true
                // A document is found by its filename, so that is what the row
                // shows when the message carries no text.
                readonly property string hitText: String(modelData.body || modelData.media_name || "")
                Accessible.name: hitText
                onClicked: root.messageChosen(String(modelData.id || ""))
                background: Rectangle {
                    color: hitRow.hovered ? Theme.hoverRow : "transparent"
                }
                contentItem: ColumnLayout {
                    spacing: 4
                    Item { Layout.fillHeight: true }
                    Label {
                        text: RowTime.label(hitRow.modelData.timestamp)
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
