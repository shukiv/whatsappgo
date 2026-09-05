import QtQuick

Item {
    id: root

    property url source
    property color tint: "black"
    readonly property string kind: {
        const parts = String(source).split("/")
        return parts[parts.length - 1].replace(".svg", "")
    }

    onTintChanged: iconCanvas.requestPaint()
    onKindChanged: iconCanvas.requestPaint()

    Canvas {
        id: iconCanvas
        anchors.fill: parent
        renderStrategy: Canvas.Immediate
        antialiasing: true

        onPaint: {
            const ctx = getContext("2d")
            ctx.reset()
            ctx.clearRect(0, 0, width, height)
            ctx.save()
            ctx.scale(width / 24, height / 24)
            ctx.strokeStyle = root.tint
            ctx.fillStyle = root.tint
            ctx.lineWidth = 2
            ctx.lineCap = "round"
            ctx.lineJoin = "round"

            if (root.kind === "back") {
                ctx.beginPath()
                ctx.moveTo(19, 12)
                ctx.lineTo(5, 12)
                ctx.moveTo(10, 7)
                ctx.lineTo(5, 12)
                ctx.lineTo(10, 17)
                ctx.stroke()
            } else if (root.kind === "search") {
                ctx.beginPath()
                ctx.arc(10.5, 10.5, 6.5, 0, Math.PI * 2)
                ctx.moveTo(15.5, 15.5)
                ctx.lineTo(20, 20)
                ctx.stroke()
            } else if (root.kind === "menu") {
                for (let y of [5, 12, 19]) {
                    ctx.beginPath()
                    ctx.arc(12, y, 1.8, 0, Math.PI * 2)
                    ctx.fill()
                }
            } else if (root.kind === "chats" || root.kind === "new-chat") {
                ctx.beginPath()
                // WhatsApp Web's bubble is squarer than a circle and sits
                // wider in its box, with the tail dropped from the left edge.
                ctx.moveTo(21.5, 12)
                ctx.bezierCurveTo(21.5, 17.4, 17.4, 20.4, 12, 20.4)
                ctx.lineTo(7.6, 20.4)
                ctx.lineTo(3, 22.4)
                ctx.lineTo(4.4, 18.6)
                ctx.bezierCurveTo(2.9, 16.9, 2.5, 14.6, 2.5, 12)
                ctx.bezierCurveTo(2.5, 6.6, 6.6, 3.6, 12, 3.6)
                ctx.bezierCurveTo(17.4, 3.6, 21.5, 6.6, 21.5, 12)
                ctx.stroke()
                if (root.kind === "new-chat") {
                    ctx.beginPath()
                    ctx.moveTo(12, 7.5)
                    ctx.lineTo(12, 13.5)
                    ctx.moveTo(9, 10.5)
                    ctx.lineTo(15, 10.5)
                    ctx.stroke()
                } else {
                    // WhatsApp Web puts two lines of writing in the bubble,
                    // not three dots.
                    ctx.beginPath()
                    ctx.moveTo(8, 9.8)
                    ctx.lineTo(16, 9.8)
                    ctx.moveTo(8, 14)
                    ctx.lineTo(13.6, 14)
                    ctx.stroke()
                }
            } else if (root.kind === "attach") {
                ctx.beginPath()
                ctx.moveTo(9, 17)
                ctx.lineTo(16.8, 9.2)
                ctx.bezierCurveTo(18, 8, 18, 6, 16.8, 4.8)
                ctx.bezierCurveTo(15.5, 3.5, 13.5, 3.5, 12.3, 4.7)
                ctx.lineTo(4.7, 12.3)
                ctx.bezierCurveTo(2.7, 14.3, 2.7, 17.4, 4.7, 19.4)
                ctx.bezierCurveTo(6.7, 21.4, 9.8, 21.4, 11.8, 19.4)
                ctx.lineTo(18.8, 12.4)
                ctx.stroke()
            } else if (root.kind === "send") {
                ctx.beginPath()
                ctx.moveTo(3, 4)
                ctx.lineTo(21, 12)
                ctx.lineTo(3, 20)
                ctx.lineTo(6, 12)
                ctx.closePath()
                ctx.moveTo(6, 12)
                ctx.lineTo(21, 12)
                ctx.stroke()
            } else if (root.kind === "mic") {
                ctx.beginPath()
                ctx.moveTo(8.5, 7)
                ctx.bezierCurveTo(8.5, 1.7, 15.5, 1.7, 15.5, 7)
                ctx.lineTo(15.5, 11)
                ctx.bezierCurveTo(15.5, 16.3, 8.5, 16.3, 8.5, 11)
                ctx.closePath()
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(5.5, 11.5)
                ctx.bezierCurveTo(5.5, 20, 18.5, 20, 18.5, 11.5)
                ctx.moveTo(12, 18)
                ctx.lineTo(12, 21)
                ctx.moveTo(9, 21)
                ctx.lineTo(15, 21)
                ctx.stroke()
            } else if (root.kind === "stop") {
                ctx.fillRect(6, 6, 12, 12)
            } else if (root.kind === "close") {
                ctx.beginPath()
                ctx.moveTo(5, 5)
                ctx.lineTo(19, 19)
                ctx.moveTo(19, 5)
                ctx.lineTo(5, 19)
                ctx.stroke()
            } else if (root.kind === "smile") {
                ctx.beginPath()
                ctx.arc(12, 12, 9, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(9, 9.5, 1, 0, Math.PI * 2)
                ctx.arc(15, 9.5, 1, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.arc(12, 12, 5, 0.25 * Math.PI, 0.75 * Math.PI)
                ctx.stroke()
            } else if (root.kind === "reconnect") {
                ctx.beginPath()
                ctx.arc(12, 12, 7, 1.12 * Math.PI, 1.92 * Math.PI)
                ctx.moveTo(20, 7)
                ctx.lineTo(20, 12)
                ctx.lineTo(15, 12)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 12, 7, 0.12 * Math.PI, 0.92 * Math.PI)
                ctx.moveTo(4, 17)
                ctx.lineTo(4, 12)
                ctx.lineTo(9, 12)
                ctx.stroke()
            } else if (root.kind === "rotate-left" || root.kind === "rotate-right") {
                const mirrored = root.kind === "rotate-right"
                ctx.save()
                if (mirrored) {
                    ctx.translate(24, 0)
                    ctx.scale(-1, 1)
                }
                // Three quarters of a circle with a solid head at the open
                // end, so the icon reads as a turn rather than as a bracket.
                const cx = 12, cy = 12.5, r = 7.2
                const head = Math.PI * 0.78
                ctx.beginPath()
                ctx.arc(cx, cy, r, head, Math.PI * 2.42)
                ctx.stroke()
                const hx = cx + r * Math.cos(head)
                const hy = cy + r * Math.sin(head)
                ctx.beginPath()
                ctx.moveTo(hx - 3.4, hy - 1.6)
                ctx.lineTo(hx + 2.4, hy - 3.1)
                ctx.lineTo(hx + 1.0, hy + 2.9)
                ctx.closePath()
                ctx.fill()
                ctx.restore()
            } else if (root.kind === "archive") {
                ctx.strokeRect(4, 4, 16, 4)
                ctx.beginPath()
                ctx.moveTo(5.5, 8)
                ctx.lineTo(5.5, 19)
                ctx.lineTo(18.5, 19)
                ctx.lineTo(18.5, 8)
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(9.5, 12)
                ctx.lineTo(14.5, 12)
                ctx.stroke()
            } else if (root.kind === "mute") {
                ctx.beginPath()
                ctx.moveTo(7, 10)
                ctx.lineTo(7, 15)
                ctx.lineTo(10, 15)
                ctx.lineTo(14, 19)
                ctx.lineTo(14, 6)
                ctx.lineTo(10, 10)
                ctx.closePath()
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(16.5, 9.5)
                ctx.lineTo(20.5, 15.5)
                ctx.moveTo(20.5, 9.5)
                ctx.lineTo(16.5, 15.5)
                ctx.stroke()
            } else if (root.kind === "pin") {
                // Traced row by row off WhatsApp Web at the same scale, where
                // the mark is 13 by 21 device pixels. One unit here is one of
                // those pixels. What makes it read as a drawing pin rather
                // than as a little table is the barrel: walls two pixels
                // thick around a five pixel gap, so it looks solid, with the
                // cap and the base plate standing out past it on both sides.
                ctx.fillRect(8, 1, 9, 1)      // cap, rounded by one pixel
                ctx.fillRect(7, 2, 11, 1)
                ctx.fillRect(8, 3, 3, 1)      // shoulders under the cap
                ctx.fillRect(14, 3, 3, 1)
                ctx.fillRect(8, 4, 2, 7)      // barrel walls
                ctx.fillRect(15, 4, 2, 7)
                ctx.fillRect(7, 11, 3, 1)     // flaring into the plate
                ctx.fillRect(15, 11, 3, 1)
                ctx.fillRect(6, 12, 4, 1)
                ctx.fillRect(15, 12, 4, 1)
                ctx.fillRect(6, 13, 13, 1)    // base plate
                ctx.fillRect(7, 14, 11, 1)
                ctx.fillRect(12, 15, 1, 7)    // needle
            } else if (root.kind === "logout") {
                ctx.beginPath()
                ctx.moveTo(10, 5)
                ctx.lineTo(5, 5)
                ctx.lineTo(5, 19)
                ctx.lineTo(10, 19)
                ctx.moveTo(13, 8)
                ctx.lineTo(17, 12)
                ctx.lineTo(13, 16)
                ctx.moveTo(17, 12)
                ctx.lineTo(8, 12)
                ctx.stroke()
            } else if (root.kind === "reply") {
                ctx.beginPath()
                ctx.moveTo(9, 17)
                ctx.lineTo(4, 12)
                ctx.lineTo(9, 7)
                ctx.moveTo(4, 12)
                ctx.lineTo(13, 12)
                ctx.bezierCurveTo(18, 12, 20, 15, 20, 19)
                ctx.stroke()
            } else if (root.kind === "heart") {
                ctx.beginPath()
                ctx.moveTo(12, 20)
                ctx.lineTo(4.2, 12.5)
                ctx.bezierCurveTo(0.3, 8.6, 2.8, 3, 7.5, 3)
                ctx.bezierCurveTo(9.5, 3, 11, 4, 12, 5.5)
                ctx.bezierCurveTo(13, 4, 14.5, 3, 16.5, 3)
                ctx.bezierCurveTo(21.2, 3, 23.7, 8.6, 19.8, 12.5)
                ctx.closePath()
                ctx.stroke()
            } else if (root.kind === "copy") {
                ctx.strokeRect(8, 8, 11, 12)
                ctx.beginPath()
                ctx.moveTo(15, 8)
                ctx.lineTo(15, 4)
                ctx.lineTo(4, 4)
                ctx.lineTo(4, 16)
                ctx.lineTo(8, 16)
                ctx.stroke()
            } else if (root.kind === "edit") {
                ctx.beginPath()
                ctx.moveTo(5, 19)
                ctx.lineTo(6, 15)
                ctx.lineTo(16.5, 4.5)
                ctx.bezierCurveTo(18.5, 2.5, 21.5, 5.5, 19.5, 7.5)
                ctx.lineTo(9, 18)
                ctx.closePath()
                ctx.moveTo(12, 20)
                ctx.lineTo(21, 20)
                ctx.stroke()
            } else if (root.kind === "delete") {
                ctx.beginPath()
                ctx.moveTo(4, 7)
                ctx.lineTo(20, 7)
                ctx.moveTo(7, 7)
                ctx.lineTo(8, 21)
                ctx.lineTo(16, 21)
                ctx.lineTo(17, 7)
                ctx.moveTo(9, 7)
                ctx.lineTo(9, 4)
                ctx.lineTo(15, 4)
                ctx.lineTo(15, 7)
                ctx.moveTo(10, 11)
                ctx.lineTo(10, 17)
                ctx.moveTo(14, 11)
                ctx.lineTo(14, 17)
                ctx.stroke()
            } else if (root.kind === "sun") {
                ctx.beginPath()
                ctx.arc(12, 12, 4, 0, Math.PI * 2)
                for (let i = 0; i < 8; ++i) {
                    const angle = i * Math.PI / 4
                    ctx.moveTo(12 + Math.cos(angle) * 8, 12 + Math.sin(angle) * 8)
                    ctx.lineTo(12 + Math.cos(angle) * 10, 12 + Math.sin(angle) * 10)
                }
                ctx.stroke()
            } else if (root.kind === "moon") {
                ctx.beginPath()
                ctx.moveTo(20.2, 15.4)
                ctx.bezierCurveTo(14, 17, 7, 10, 8.6, 3.8)
                ctx.bezierCurveTo(1.5, 6.2, 2.5, 17.5, 10, 20.2)
                ctx.bezierCurveTo(14, 21.6, 18.3, 19.6, 20.2, 15.4)
                ctx.stroke()
            } else if (root.kind === "user") {
                ctx.beginPath()
                ctx.arc(12, 8, 4, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(4.5, 21)
                ctx.bezierCurveTo(4.5, 11, 19.5, 11, 19.5, 21)
                ctx.closePath()
                ctx.fill()
            } else if (root.kind === "calls" || root.kind === "phone") {
                // WhatsApp Web's handset: two hollow ends, the earpiece at the
                // top right and the mouthpiece at the bottom left, joined by a
                // curve that bows away from them. Ours used to be a closed
                // loop, which read as a whole telephone rather than a call.
                ctx.beginPath()
                ctx.moveTo(15.2, 4.4)
                ctx.lineTo(19.6, 4.4)
                ctx.lineTo(19.6, 9)
                ctx.lineTo(17.4, 10.4)
                ctx.lineTo(14.6, 7.4)
                ctx.closePath()
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(4.4, 15.2)
                ctx.lineTo(7.4, 14.6)
                ctx.lineTo(10.4, 17.4)
                ctx.lineTo(9, 19.6)
                ctx.lineTo(4.4, 19.6)
                ctx.closePath()
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(17.2, 10.6)
                ctx.bezierCurveTo(15.8, 13.6, 13.6, 15.8, 10.6, 17.2)
                ctx.stroke()
            } else if (root.kind === "status") {
                // A ring broken at the sides around a smaller solid ring, the
                // proportions WhatsApp Web uses: the outer one nearly twice
                // the inner, both drawn in the same weight.
                ctx.beginPath()
                // The ring is broken along the diagonal, not at the sides.
                ctx.arc(12, 12, 9, 1.15, 3.55)
                ctx.moveTo(15.8, 3.9)
                ctx.arc(12, 12, 9, 4.29, 6.69)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 12, 5, 0, Math.PI * 2)
                ctx.stroke()
            } else if (root.kind === "channels") {
                // WhatsApp Web draws a message bubble with a broadcast mark
                // inside it: a dot between two arcs. Ours was three bare rings
                // with no bubble around them, which read as a wifi symbol.
                ctx.beginPath()
                ctx.arc(12, 11.4, 9.4, 0.62, Math.PI * 2 + 0.12)
                ctx.lineTo(19.8, 20.6)
                ctx.closePath()
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 11.4, 1.5, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.arc(12, 11.4, 5.2, -0.72, 0.72)
                ctx.moveTo(8.1, 8)
                ctx.arc(12, 11.4, 5.2, Math.PI - 0.72, Math.PI + 0.72)
                ctx.stroke()
            } else if (root.kind === "group-add") {
                ctx.beginPath()
                ctx.arc(9, 8, 3, 0, Math.PI * 2)
                ctx.arc(16.5, 9.5, 2.3, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(3, 20)
                ctx.bezierCurveTo(3, 13, 15, 13, 15, 20)
                ctx.moveTo(17.5, 14)
                ctx.lineTo(17.5, 21)
                ctx.moveTo(14, 17.5)
                ctx.lineTo(21, 17.5)
                ctx.stroke()
            } else if (root.kind === "user-add") {
                ctx.beginPath()
                ctx.arc(9, 8, 3.5, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(3, 21)
                ctx.bezierCurveTo(3, 12.5, 15, 12.5, 15, 21)
                ctx.closePath()
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(18, 10)
                ctx.lineTo(18, 17)
                ctx.moveTo(14.5, 13.5)
                ctx.lineTo(21.5, 13.5)
                ctx.stroke()
            } else if (root.kind === "communities") {
                // Three people: the one in the middle drawn in outline and the
                // two beside it solid, which is how WhatsApp Web separates the
                // group from the community around it.
                ctx.beginPath()
                ctx.arc(4.2, 8.6, 2.4, 0, Math.PI * 2)
                ctx.arc(19.8, 8.6, 2.4, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.roundedRect(0.6, 13.6, 4.8, 6, 2.4, 2.4)
                ctx.roundedRect(18.6, 13.6, 4.8, 6, 2.4, 2.4)
                ctx.fill()
                ctx.beginPath()
                ctx.arc(12, 7.4, 3, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.roundedRect(7.2, 13.2, 9.6, 6.6, 2, 2)
                ctx.stroke()
            } else if (root.kind === "profile") {
                ctx.beginPath()
                ctx.arc(12, 8, 4, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 20, 7, Math.PI, Math.PI * 2)
                ctx.stroke()
            } else if (root.kind === "video") {
                ctx.strokeRect(3, 6, 13, 12)
                ctx.beginPath()
                ctx.moveTo(16, 10)
                ctx.lineTo(21, 7)
                ctx.lineTo(21, 17)
                ctx.lineTo(16, 14)
                ctx.stroke()
            } else if (root.kind === "gallery") {
                ctx.strokeRect(3, 4, 18, 16)
                ctx.beginPath()
                ctx.arc(8, 9, 1.5, 0, Math.PI * 2)
                ctx.moveTo(4, 18)
                ctx.lineTo(9.5, 12.5)
                ctx.lineTo(13, 16)
                ctx.lineTo(16, 13)
                ctx.lineTo(21, 18)
                ctx.stroke()
            } else if (root.kind === "document") {
                ctx.beginPath()
                ctx.moveTo(6, 3)
                ctx.lineTo(14, 3)
                ctx.lineTo(19, 8)
                ctx.lineTo(19, 21)
                ctx.lineTo(6, 21)
                ctx.closePath()
                ctx.moveTo(14, 3)
                ctx.lineTo(14, 8)
                ctx.lineTo(19, 8)
                ctx.moveTo(9, 13)
                ctx.lineTo(16, 13)
                ctx.moveTo(9, 17)
                ctx.lineTo(16, 17)
                ctx.stroke()
            } else if (root.kind === "camera") {
                ctx.beginPath()
                ctx.moveTo(4, 7)
                ctx.lineTo(7, 7)
                ctx.lineTo(8.5, 5)
                ctx.lineTo(15.5, 5)
                ctx.lineTo(17, 7)
                ctx.lineTo(20, 7)
                ctx.lineTo(20, 19)
                ctx.lineTo(4, 19)
                ctx.closePath()
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 13, 3.5, 0, Math.PI * 2)
                ctx.stroke()
            } else if (root.kind === "headphones") {
                ctx.beginPath()
                ctx.arc(12, 11, 8, Math.PI, Math.PI * 2)
                ctx.moveTo(4, 11)
                ctx.lineTo(4, 19)
                ctx.bezierCurveTo(4, 20, 5, 20, 7, 20)
                ctx.lineTo(7, 13)
                ctx.lineTo(4, 13)
                ctx.moveTo(20, 11)
                ctx.lineTo(20, 19)
                ctx.bezierCurveTo(20, 20, 19, 20, 17, 20)
                ctx.lineTo(17, 13)
                ctx.lineTo(20, 13)
                ctx.stroke()
            } else if (root.kind === "contact") {
                ctx.beginPath()
                ctx.arc(12, 8, 3, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(6, 20)
                ctx.bezierCurveTo(6, 12, 18, 12, 18, 20)
                ctx.closePath()
                ctx.fill()
            } else if (root.kind === "poll") {
                ctx.beginPath()
                ctx.moveTo(5, 7)
                ctx.lineTo(13, 7)
                ctx.moveTo(5, 12)
                ctx.lineTo(19, 12)
                ctx.moveTo(5, 17)
                ctx.lineTo(15, 17)
                ctx.stroke()
            } else if (root.kind === "calendar") {
                ctx.strokeRect(4, 5, 16, 15)
                ctx.beginPath()
                ctx.moveTo(8, 3)
                ctx.lineTo(8, 7)
                ctx.moveTo(16, 3)
                ctx.lineTo(16, 7)
                ctx.moveTo(4, 10)
                ctx.lineTo(20, 10)
                ctx.stroke()
                for (let x of [8, 12, 16]) {
                    for (let y of [14, 17]) {
                        ctx.beginPath()
                        ctx.arc(x, y, 0.7, 0, Math.PI * 2)
                        ctx.fill()
                    }
                }
            } else if (root.kind === "sticker") {
                ctx.beginPath()
                ctx.moveTo(5, 4)
                ctx.lineTo(19, 4)
                ctx.lineTo(19, 14)
                ctx.lineTo(13, 20)
                ctx.lineTo(5, 20)
                ctx.closePath()
                ctx.moveTo(13, 20)
                ctx.lineTo(13, 14)
                ctx.lineTo(19, 14)
                ctx.moveTo(8, 10)
                ctx.lineTo(12, 10)
                ctx.moveTo(10, 8)
                ctx.lineTo(10, 12)
                ctx.stroke()
            } else if (root.kind === "link") {
                ctx.beginPath()
                ctx.arc(8, 12, 5, Math.PI / 2, Math.PI * 1.5)
                ctx.arc(16, 12, 5, -Math.PI / 2, Math.PI / 2)
                ctx.moveTo(8, 12)
                ctx.lineTo(16, 12)
                ctx.stroke()
            } else if (root.kind === "lock") {
                ctx.strokeRect(5, 10, 14, 11)
                ctx.beginPath()
                ctx.arc(12, 10, 5, Math.PI, Math.PI * 2)
                ctx.moveTo(12, 14)
                ctx.lineTo(12, 17)
                ctx.stroke()
            } else if (root.kind === "check") {
                // A plain tick for selection state; the receipt marks draw
                // their own pair separately.
                ctx.beginPath()
                ctx.moveTo(5, 12.5)
                ctx.lineTo(9.8, 17)
                ctx.lineTo(19, 7)
                ctx.stroke()
            } else if (root.kind === "star" || root.kind === "star-filled") {
                const points = 5
                const outer = 8.6
                const inner = 3.6
                ctx.beginPath()
                for (let i = 0; i < points * 2; ++i) {
                    // Start at the top point rather than at angle zero, which
                    // would stand the star on a vertex.
                    const angle = -Math.PI / 2 + i * Math.PI / points
                    const radius = (i % 2 === 0) ? outer : inner
                    const px = 12 + Math.cos(angle) * radius
                    const py = 12 + Math.sin(angle) * radius
                    if (i === 0)
                        ctx.moveTo(px, py)
                    else
                        ctx.lineTo(px, py)
                }
                ctx.closePath()
                if (root.kind === "star-filled")
                    ctx.fill()
                else
                    ctx.stroke()
            } else if (root.kind === "forward") {
                // An arrow curving out to the right: the mirror of reply.
                ctx.beginPath()
                ctx.moveTo(15, 17)
                ctx.lineTo(20, 12)
                ctx.lineTo(15, 7)
                ctx.moveTo(20, 12)
                ctx.lineTo(11, 12)
                ctx.bezierCurveTo(6, 12, 4, 15, 4, 19)
                ctx.stroke()
            } else if (root.kind === "chevron-right") {
                ctx.beginPath()
                ctx.moveTo(9, 5)
                ctx.lineTo(16, 12)
                ctx.lineTo(9, 19)
                ctx.stroke()
            } else if (root.kind === "bug") {
                // Rounded body with a head, antennae and three legs a side.
                ctx.beginPath()
                ctx.arc(12, 13.5, 5, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 7, 2.6, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(10.2, 5.2)
                ctx.lineTo(8.6, 3.2)
                ctx.moveTo(13.8, 5.2)
                ctx.lineTo(15.4, 3.2)
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(7, 11)
                ctx.lineTo(3.5, 9.5)
                ctx.moveTo(7, 13.5)
                ctx.lineTo(3.2, 13.5)
                ctx.moveTo(7, 16)
                ctx.lineTo(3.5, 17.5)
                ctx.moveTo(17, 11)
                ctx.lineTo(20.5, 9.5)
                ctx.moveTo(17, 13.5)
                ctx.lineTo(20.8, 13.5)
                ctx.moveTo(17, 16)
                ctx.lineTo(20.5, 17.5)
                ctx.stroke()
            } else if (root.kind === "info") {
                ctx.beginPath()
                ctx.arc(12, 12, 8.5, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(12, 11)
                ctx.lineTo(12, 16.5)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 7.8, 1.15, 0, Math.PI * 2)
                ctx.fill()
            } else if (root.kind === "play") {
                ctx.beginPath()
                ctx.moveTo(8, 5)
                ctx.lineTo(19, 12)
                ctx.lineTo(8, 19)
                ctx.closePath()
                ctx.fill()
            } else if (root.kind === "sort") {
                ctx.beginPath()
                ctx.moveTo(4, 7)
                ctx.lineTo(20, 7)
                ctx.moveTo(6, 12)
                ctx.lineTo(18, 12)
                ctx.moveTo(9, 17)
                ctx.lineTo(15, 17)
                ctx.stroke()
            } else if (root.kind === "plus") {
                ctx.beginPath()
                ctx.moveTo(12, 5)
                ctx.lineTo(12, 19)
                ctx.moveTo(5, 12)
                ctx.lineTo(19, 12)
                ctx.stroke()
            } else if (root.kind === "block") {
                ctx.beginPath()
                ctx.arc(12, 12, 8.5, 0, Math.PI * 2)
                ctx.moveTo(6, 18)
                ctx.lineTo(18, 6)
                ctx.stroke()
            } else if (root.kind === "chevron-down") {
                ctx.beginPath()
                ctx.moveTo(5, 9)
                ctx.lineTo(12, 16)
                ctx.lineTo(19, 9)
                ctx.stroke()
            }

            ctx.restore()
        }
    }
}
