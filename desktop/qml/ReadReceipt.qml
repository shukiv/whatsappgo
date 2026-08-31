import QtQuick
import org.whatsappgo

// The delivery marks on an outgoing message.
//
// Writing them as the characters "✓✓" produced two separate ticks with a gap
// between them, which is not the mark WhatsApp shows. The real one is a single
// symbol whose second tick overlaps the first, so it is drawn here instead of
// being typed.
Canvas {
    id: root

    property string status: ""

    readonly property bool delivered: status === "delivered" || status === "read" || status === "played"
    readonly property bool seen: status === "read" || status === "played"
    readonly property color markColor: seen ? Theme.readReceipt : Theme.textMuted

    implicitWidth: delivered ? 15 : 11
    implicitHeight: 11
    width: implicitWidth
    height: implicitHeight

    renderStrategy: Canvas.Immediate
    antialiasing: true

    Accessible.role: Accessible.StaticText
    Accessible.name: seen ? qsTr("Read") : delivered ? qsTr("Delivered") : qsTr("Sent")

    onMarkColorChanged: requestPaint()
    onDeliveredChanged: requestPaint()
    onWidthChanged: requestPaint()

    onPaint: {
        const ctx = getContext("2d")
        ctx.reset()
        ctx.clearRect(0, 0, width, height)
        ctx.strokeStyle = markColor
        ctx.lineWidth = 1.3
        ctx.lineCap = "round"
        ctx.lineJoin = "round"

        const tick = offset => {
            ctx.beginPath()
            ctx.moveTo(offset + 0.8, 6.1)
            ctx.lineTo(offset + 3.6, 8.9)
            ctx.lineTo(offset + 9.4, 2.2)
            ctx.stroke()
        }

        // The trailing tick sits behind, shifted just enough for the two to
        // read as one mark rather than as two separate ticks.
        if (delivered)
            tick(4.4)
        tick(0)
    }
}
