import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

// A settings line whose value is one of a short list, shown as the PWA shows
// them: the current answer on the right, a menu of the rest on press.
ItemDelegate {
    id: root

    property var choices: []
    property var choiceLabels: []
    property string value: ""
    signal choiceSelected(string choice)

    implicitHeight: 56
    onClicked: choiceMenu.toggleUnder(root)

    readonly property string valueLabel: {
        for (let i = 0; i < root.choices.length; ++i) {
            if (String(root.choices[i]) === root.value)
                return String(root.choiceLabels[i])
        }
        return root.value === "" ? qsTr("Unknown") : root.value
    }

    background: Rectangle {
        color: root.down ? Theme.pressedRow : root.hovered ? Theme.hoverRow : "transparent"
    }

    contentItem: RowLayout {
        spacing: 16
        Label {
            Layout.leftMargin: 22
            Layout.fillWidth: true
            text: root.text
            color: Theme.text
            font.pixelSize: 15
            elide: Text.ElideRight
        }
        Label {
            objectName: "settingsChoiceValue"
            Layout.rightMargin: 22
            text: root.valueLabel
            color: Theme.textMuted
            font.pixelSize: 14
        }
    }

    WhatsAppMenuPopup {
        id: choiceMenu
        objectName: "settingsChoiceMenu"
        parent: Overlay.overlay
        width: 240
        anchorItem: root
        anchorOffsetX: -20

        Repeater {
            model: root.choices
            WhatsAppMenuItem {
                required property string modelData
                required property int index
                text: String(root.choiceLabels[index])
                checkable: true
                checked: modelData === root.value
                onClicked: {
                    choiceMenu.close()
                    if (modelData !== root.value)
                        root.choiceSelected(modelData)
                }
            }
        }
    }
}
