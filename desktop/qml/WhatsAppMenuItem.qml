import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

ItemDelegate {
    id: root
    property url iconSource
    property bool destructive: false
    property color iconTint: destructive ? Theme.danger : Theme.text
    property string trailingText: ""
    property url actionIconSource
    property string actionAccessibleName: ""
    property string actionObjectName: ""
    signal actionTriggered()
    // A second row action, for entries that both edit and remove something.
    property url secondaryActionIconSource
    property string secondaryActionAccessibleName: ""
    property string secondaryActionObjectName: ""
    property bool secondaryActionDestructive: true
    signal secondaryActionTriggered()

    // A row that carries edit/remove buttons needs a little more height than a
    // plain one, so the buttons are not pressed against its edges.
    readonly property bool hasRowActions: actionIconSource.toString().length > 0
        || secondaryActionIconSource.toString().length > 0

    width: parent ? parent.width : 256
    implicitHeight: hasRowActions ? 44 : 36
    height: implicitHeight
    leftPadding: 10
    rightPadding: 10
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    Accessible.name: text
    Accessible.checked: checkable && checked

    background: Rectangle {
        color: root.down ? Theme.pressedRow : root.hovered || root.activeFocus ? Theme.hoverRow : "transparent"
        radius: 5
        border.color: root.activeFocus ? Theme.primary : "transparent"
        border.width: root.activeFocus ? 2 : 0
    }

    contentItem: RowLayout {
        spacing: 10

        TintedIcon {
            Layout.alignment: Qt.AlignVCenter
            Layout.preferredWidth: 18
            Layout.preferredHeight: 18
            source: root.iconSource
            tint: root.iconTint
        }

        // A RowLayout stretches its children to the row's height, and a Label
        // draws its text at the top of whatever height it is given, so every
        // label here says where it sits.
        Label {
            Layout.alignment: Qt.AlignVCenter
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            text: root.text
            color: root.destructive ? Theme.danger : Theme.text
            font.pixelSize: 14
            verticalAlignment: Text.AlignVCenter
            elide: Text.ElideRight
        }

        Label {
            Layout.alignment: Qt.AlignVCenter
            visible: root.trailingText.length > 0
            text: root.trailingText
            color: Theme.primary
            font.pixelSize: 12
            font.weight: Font.DemiBold
            verticalAlignment: Text.AlignVCenter
        }

        ThemedToolButton {
            objectName: root.actionObjectName
            visible: root.actionIconSource.toString().length > 0
            Layout.alignment: Qt.AlignVCenter
            Layout.preferredWidth: 32
            Layout.preferredHeight: 32
            iconSource: root.actionIconSource
            iconSize: 15
            Accessible.name: root.actionAccessibleName
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            background: Rectangle {
                radius: width / 2
                color: parent.down ? Theme.pressedRow
                    : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                border.color: parent.activeFocus ? Theme.primary : "transparent"
                border.width: parent.activeFocus ? 2 : 0
            }
            onClicked: root.actionTriggered()
        }

        ThemedToolButton {
            objectName: root.secondaryActionObjectName
            visible: root.secondaryActionIconSource.toString().length > 0
            Layout.alignment: Qt.AlignVCenter
            Layout.preferredWidth: 32
            Layout.preferredHeight: 32
            iconSource: root.secondaryActionIconSource
            iconSize: 15
            iconTint: root.secondaryActionDestructive ? Theme.danger : Theme.icon
            Accessible.name: root.secondaryActionAccessibleName
            ToolTip.visible: hovered
            ToolTip.text: Accessible.name
            background: Rectangle {
                radius: width / 2
                color: parent.down ? Theme.pressedRow
                    : parent.hovered || parent.activeFocus ? Theme.navigationHover : "transparent"
                border.color: parent.activeFocus ? Theme.primary : "transparent"
                border.width: parent.activeFocus ? 2 : 0
            }
            onClicked: root.secondaryActionTriggered()
        }

        Label {
            Layout.alignment: Qt.AlignVCenter
            visible: root.checkable && root.checked
            text: "✓"
            color: Theme.primary
            font.pixelSize: 16
            font.weight: Font.DemiBold
            verticalAlignment: Text.AlignVCenter
            Accessible.ignored: true
        }
    }
}
