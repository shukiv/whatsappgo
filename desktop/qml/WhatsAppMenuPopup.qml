import QtQuick
import QtQuick.Controls
import org.whatsappgo

Popup {
    id: root
    default property alias menuItems: menuColumn.data

    width: 229
    implicitHeight: menuColumn.implicitHeight + topPadding + bottomPadding
    padding: 5
    modal: false
    focus: true
    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

    function clampToParent() {
        if (!parent)
            return
        const shownHeight = Math.max(height, implicitHeight)
        if (parent.width >= width + 16)
            x = Math.max(8, Math.min(parent.width - width - 8, x))
        // Only pull the menu upwards when the parent can actually hold it.
        // Clamping unconditionally would drag a menu that is taller than its
        // parent up to the top edge, which moves anchored menus (the account
        // switcher opens below its chip) away from the control they belong to.
        if (parent.height >= shownHeight + 16)
            y = Math.max(8, Math.min(parent.height - shownHeight - 8, y))
    }

    // Some Popup content is polished only during open(), so run the clamp
    // both before and immediately after that polish. This prevents the last
    // actions from being cut off when a menu is opened near the composer.
    onAboutToShow: Qt.callLater(root.clampToParent)
    onOpened: {
        clampToParent()
        Qt.callLater(root.clampToParent)
    }
    // `opened` only becomes true once the enter transition has finished, so a
    // menu whose rows are still being laid out during that animation would keep
    // the position it was given before its final height was known. Track
    // `visible` instead: it is true for the whole time the menu is on screen.
    onVisibleChanged: {
        if (visible)
            clampToParent()
    }
    onHeightChanged: {
        if (visible)
            clampToParent()
    }
    onImplicitHeightChanged: {
        if (visible)
            clampToParent()
    }

    enter: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: 90 }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: 70 }
    }

    background: Item {
        Rectangle {
            anchors.fill: parent
            anchors.leftMargin: 2
            anchors.topMargin: 4
            color: Theme.dark ? "#52000000" : "#26000000"
            radius: 9
        }
        Rectangle {
            anchors.fill: parent
            anchors.rightMargin: 2
            anchors.bottomMargin: 4
            color: Theme.surfaceRaised
            border.color: Theme.border
            border.width: 1
            radius: 9
        }
    }

    contentItem: Column {
        id: menuColumn
        spacing: 1
    }
}
