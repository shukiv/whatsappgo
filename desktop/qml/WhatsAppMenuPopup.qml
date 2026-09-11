import QtQuick
import QtQuick.Controls
import QtQuick.Templates as T
import org.whatsappgo

Popup {
    id: root
    default property alias menuItems: menuColumn.data
    // Some callers parent their popup to a small button/filter row rather
    // than the overlay. The window, not that anchor, bounds the usable menu.
    readonly property Item viewportItem: Overlay.overlay || parent

    width: 229
    implicitHeight: menuColumn.implicitHeight + topPadding + bottomPadding
    height: viewportItem && viewportItem.height > 0
        ? Math.min(implicitHeight, Math.max(1, viewportItem.height - 16)) : implicitHeight
    padding: 5
    modal: false
    focus: true
    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

    // The control the menu belongs to. A menu hangs under its button, lined
    // up with the button's right edge, the way WhatsApp Web's do.
    //
    // The position cannot be a binding: mapToItem is a function call, so QML
    // has nothing to re-evaluate it on. A menu bound that way keeps whatever
    // the layout happened to say the first time - before the window had been
    // laid out - and afterwards opens far from the button that owns it.
    property Item anchorItem
    // A menu normally hangs below its control and lines up with its right
    // edge. The composer's attachment menu opens upwards, and a couple of
    // menus sit a few pixels in from the edge they line up with.
    property bool anchorAbove: false
    property bool anchorAlignLeft: false
    property real anchorOffsetX: 0

    function viewportBounds() {
        if (!parent || !viewportItem)
            return Qt.rect(0, 0, 0, 0)
        const point = viewportItem.mapToItem(parent, 0, 0)
        return Qt.rect(point.x, point.y, viewportItem.width, viewportItem.height)
    }

    function positionUnder(item) {
        if (!item || !parent)
            return
        anchorItem = item
        const anchor = item.mapToItem(parent, 0, 0)
        const shownHeight = height
        const wantedX = (anchorAlignLeft ? anchor.x : anchor.x + item.width - width) + anchorOffsetX
        const wantedY = anchorAbove
            ? anchor.y - shownHeight - 8
            : anchor.y + item.height + 2
        const bounds = viewportBounds()
        x = bounds.width >= width + 16
            ? Math.max(bounds.x + 8, Math.min(bounds.x + bounds.width - width - 8, wantedX))
            : wantedX
        y = bounds.height >= shownHeight + 16
            ? Math.max(bounds.y + 8, Math.min(bounds.y + bounds.height - shownHeight - 8, wantedY))
            : wantedY
    }

    function openUnder(item) {
        positionUnder(item)
        open()
    }

    function toggleUnder(item) {
        if (opened)
            close()
        else
            openUnder(item)
    }

    function clampToParent() {
        // A menu with an anchor is placed against it again, so growing content
        // pushes it back under its button rather than merely inside the window.
        if (anchorItem && visible) {
            positionUnder(anchorItem)
            return
        }
        if (!parent)
            return
        const shownHeight = height
        const bounds = viewportBounds()
        if (bounds.width >= width + 16)
            x = Math.max(bounds.x + 8, Math.min(bounds.x + bounds.width - width - 8, x))
        if (bounds.height >= shownHeight + 16)
            y = Math.max(bounds.y + 8, Math.min(bounds.y + bounds.height - shownHeight - 8, y))
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

    function focusMenuAction(edge, step) {
        const actions = []
        for (let child of menuColumn.children) {
            if (child instanceof T.AbstractButton && child.visible && child.enabled)
                actions.push(child)
        }
        if (actions.length === 0)
            return
        let index = edge < 0 ? actions.length - 1 : 0
        if (step !== 0) {
            for (let i = 0; i < actions.length; ++i) {
                if (actions[i].activeFocus) {
                    index = (i + step + actions.length) % actions.length
                    break
                }
            }
        }
        const action = actions[index]
        action.forceActiveFocus(Qt.TabFocusReason)
        if (action.y < menuViewport.contentY)
            menuViewport.contentY = action.y
        else if (action.y + action.height > menuViewport.contentY + menuViewport.height)
            menuViewport.contentY = Math.max(0, action.y + action.height - menuViewport.height)
    }

    // Connections keep the common behavior when a caller has its own
    // onOpened handler (for example the attachment menu).
    Connections {
        target: root
        function onOpened() {
            menuViewport.contentY = 0
            root.focusMenuAction(0, 0)
        }
    }
    Connections {
        target: root.viewportItem
        function onWidthChanged() { if (root.visible) Qt.callLater(root.clampToParent) }
        function onHeightChanged() { if (root.visible) Qt.callLater(root.clampToParent) }
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

    contentItem: Flickable {
        id: menuViewport
        objectName: "menuViewport"
        contentWidth: width
        contentHeight: menuColumn.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        flickableDirection: Flickable.VerticalFlick
        ScrollBar.vertical: OverlayScrollBar {}
        Keys.onPressed: event => {
            if (event.modifiers !== Qt.NoModifier)
                return
            switch (event.key) {
            case Qt.Key_Down: root.focusMenuAction(0, 1); break
            case Qt.Key_Up: root.focusMenuAction(-1, -1); break
            case Qt.Key_Home: root.focusMenuAction(0, 0); break
            case Qt.Key_End: root.focusMenuAction(-1, 0); break
            default: return
            }
            event.accepted = true
        }
        Column {
            id: menuColumn
            width: menuViewport.width
            spacing: 1
        }
    }
}
