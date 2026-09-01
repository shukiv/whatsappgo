import QtQuick
import org.whatsappgo

Item {
    id: root
    property string title: ""
    property url source
    property int itemCount: 1
    property int diameter: 58

    implicitWidth: diameter
    implicitHeight: diameter

    Canvas {
        id: ring
        anchors.fill: parent
        antialiasing: true
        onPaint: {
            const context = getContext("2d")
            context.reset()
            const count = Math.max(1, Math.min(20, root.itemCount))
            const gap = count === 1 ? 0 : Math.min(0.13, 0.55 / count)
            const segment = Math.PI * 2 / count
            context.strokeStyle = Theme.primary
            context.lineWidth = 3
            context.lineCap = "round"
            for (let index = 0; index < count; ++index) {
                const start = -Math.PI / 2 + index * segment + gap
                const end = -Math.PI / 2 + (index + 1) * segment - gap
                context.beginPath()
                context.arc(width / 2, height / 2, width / 2 - 2, start, end)
                context.stroke()
            }
        }
        Connections {
            target: Theme
            function onPrimaryChanged() { ring.requestPaint() }
        }
    }

    Avatar {
        anchors.centerIn: parent
        diameter: root.diameter - 10
        title: root.title
        source: root.source
    }
}
