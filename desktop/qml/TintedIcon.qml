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
                ctx.moveTo(20, 11.5)
                ctx.bezierCurveTo(20, 16.5, 16, 19.7, 11.5, 19.5)
                ctx.lineTo(5, 21)
                ctx.lineTo(6.5, 17)
                ctx.bezierCurveTo(3.2, 13, 4, 7.2, 8.2, 4.5)
                ctx.bezierCurveTo(13.5, 1.2, 20, 5, 20, 11.5)
                ctx.stroke()
                if (root.kind === "new-chat") {
                    ctx.beginPath()
                    ctx.moveTo(12, 7.5)
                    ctx.lineTo(12, 13.5)
                    ctx.moveTo(9, 10.5)
                    ctx.lineTo(15, 10.5)
                    ctx.stroke()
                } else {
                    for (let x of [9, 12, 15]) {
                        ctx.beginPath()
                        ctx.arc(x, 11.5, 0.8, 0, Math.PI * 2)
                        ctx.fill()
                    }
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
                ctx.beginPath()
                ctx.arc(12, 12, 7, 0.15 * Math.PI, 1.65 * Math.PI)
                ctx.stroke()
                ctx.beginPath()
                ctx.moveTo(3.5, 6)
                ctx.lineTo(3.5, 12)
                ctx.lineTo(9.5, 12)
                ctx.stroke()
                ctx.restore()
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
            } else if (root.kind === "calls") {
                ctx.beginPath()
                ctx.moveTo(7, 3)
                ctx.lineTo(10, 8)
                ctx.lineTo(7.5, 10.5)
                ctx.bezierCurveTo(9, 13.5, 10.5, 15, 13.5, 16.5)
                ctx.lineTo(16, 14)
                ctx.lineTo(21, 17)
                ctx.bezierCurveTo(18, 23, 10, 19, 6, 15)
                ctx.bezierCurveTo(2, 11, -2, 3, 4, 0)
                ctx.closePath()
                ctx.stroke()
            } else if (root.kind === "status") {
                ctx.beginPath()
                ctx.arc(12, 12, 8, -1.15, 1.15)
                ctx.moveTo(9.1, 19.4)
                ctx.arc(12, 12, 8, 1.95, 4.33)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 12, 3.2, 0, Math.PI * 2)
                ctx.stroke()
            } else if (root.kind === "channels") {
                ctx.beginPath()
                ctx.arc(12, 12, 2.6, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 12, 7, -0.75, 0.75)
                ctx.moveTo(7.1, 17)
                ctx.arc(12, 12, 7, 2.35, 3.93)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 12, 10, -0.7, 0.7)
                ctx.moveTo(4.4, 18.5)
                ctx.arc(12, 12, 10, 2.45, 3.83)
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
                ctx.beginPath()
                ctx.arc(12, 8, 3.2, 0, Math.PI * 2)
                ctx.arc(5.5, 10, 2.4, 0, Math.PI * 2)
                ctx.arc(18.5, 10, 2.4, 0, Math.PI * 2)
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(6, 21)
                ctx.bezierCurveTo(6, 13, 18, 13, 18, 21)
                ctx.closePath()
                ctx.fill()
                ctx.beginPath()
                ctx.moveTo(1.5, 20)
                ctx.bezierCurveTo(1.5, 15, 6, 14, 8, 16)
                ctx.moveTo(22.5, 20)
                ctx.bezierCurveTo(22.5, 15, 18, 14, 16, 16)
                ctx.stroke()
            } else if (root.kind === "profile") {
                ctx.beginPath()
                ctx.arc(12, 8, 4, 0, Math.PI * 2)
                ctx.stroke()
                ctx.beginPath()
                ctx.arc(12, 20, 7, Math.PI, Math.PI * 2)
                ctx.stroke()
            }

            ctx.restore()
        }
    }
}
