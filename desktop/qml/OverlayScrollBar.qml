import QtQuick
import QtQuick.Controls
import org.whatsappgo

ScrollBar {
    id: control
    policy: ScrollBar.AsNeeded
    interactive: true
    minimumSize: 0.06
    padding: 1
    implicitWidth: 6
    implicitHeight: 6

    // Flickable lays out attached bars automatically, but ScrollView's style
    // supplies that geometry. Replacing its bar also replaces those bindings.
    // Only supply them for ScrollView; leave attached and standalone list bars
    // under their existing geometry management.
    readonly property ScrollView scrollView: parent instanceof ScrollView ? parent : null

    Binding on x {
        when: control.scrollView !== null
        value: control.scrollView
            ? (control.vertical
                ? (control.scrollView.mirrored ? 0 : control.scrollView.width - control.width)
                : control.scrollView.leftPadding)
            : 0
    }
    Binding on y {
        when: control.scrollView !== null
        value: control.scrollView
            ? (control.vertical ? control.scrollView.topPadding : control.scrollView.height - control.height)
            : 0
    }
    Binding on height {
        when: control.scrollView !== null && control.vertical
        value: control.scrollView ? control.scrollView.availableHeight : control.implicitHeight
    }
    Binding on width {
        when: control.scrollView !== null && control.horizontal
        value: control.scrollView ? control.scrollView.availableWidth : control.implicitWidth
    }

    background: Item {}

    contentItem: Rectangle {
        visible: control.size < 1.0
        implicitWidth: 4
        implicitHeight: 36
        radius: width / 2
        color: control.hovered || control.pressed ? Theme.scrollbarHandleHover : Theme.scrollbarHandle
        opacity: control.active || control.hovered || control.pressed ? 0.9 : 0.55

        Behavior on color { ColorAnimation { duration: 100 } }
        Behavior on opacity { NumberAnimation { duration: 100 } }
    }
}
