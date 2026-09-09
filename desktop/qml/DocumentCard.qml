import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

AbstractButton {
    id: root
    objectName: "mediaAction"
    required property var message
    readonly property string fileName: String(message.media_name || qsTr("Document"))
    readonly property string fileType: {
        const name = fileName.split(/[\\/]/).pop()
        const extension = name.indexOf(".") >= 0 ? name.split(".").pop() : ""
        if (/^[a-z0-9]{1,8}$/i.test(extension))
            return extension.toUpperCase()
        const mime = String(message.media_mime || "").split(";")[0].toLowerCase()
        if (mime === "application/pdf") return "PDF"
        return qsTr("FILE")
    }
    readonly property string fileSize: {
        const bytes = Number(message.media_size || 0)
        if (!isFinite(bytes) || bytes <= 0) return ""
        if (bytes < 1024) return qsTr("%1 B").arg(bytes)
        if (bytes < 1048576) return qsTr("%1 kB").arg(Math.round(bytes / 1024))
        if (bytes < 1073741824) return qsTr("%1 MB").arg((bytes / 1048576).toFixed(1))
        return qsTr("%1 GB").arg((bytes / 1073741824).toFixed(1))
    }
    readonly property bool downloading: backend.documentDownloads.indexOf(
        String(message.chat_jid || "") + "/" + String(message.id || "")) >= 0
    readonly property color badgeColor: fileType === "PDF" ? Theme.documentPdf : Theme.attachmentDocument

    implicitWidth: 328
    implicitHeight: Math.max(66, contentItem.implicitHeight + topPadding + bottomPadding)
    leftPadding: 20
    rightPadding: 24 // Keep the message-options chevron clear of the filename.
    topPadding: 12
    bottomPadding: 12
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    Accessible.name: qsTr("Download %1").arg(fileName)
    Accessible.description: downloading ? qsTr("Downloading…") : fileType + (fileSize ? " · " + fileSize : "")
    onClicked: {
        if (!downloading)
            backend.downloadDocument(message)
    }
    ToolTip.visible: hovered || activeFocus
    ToolTip.delay: 650
    ToolTip.text: fileName + "\n" + (downloading ? qsTr("Downloading…") : qsTr("Download to Downloads"))
    HoverHandler { cursorShape: Qt.PointingHandCursor }

    background: Rectangle {
        radius: 6
        color: root.message.from_me ? Qt.darker(Theme.outgoingBubble, 1.06) : Theme.surfaceMuted
        border.width: root.visualFocus ? 2 : 0
        border.color: Theme.primary
        Rectangle {
            anchors.fill: parent
            radius: parent.radius
            color: Theme.text
            opacity: root.down ? 0.08 : root.hovered ? 0.035 : 0
        }
    }
    contentItem: RowLayout {
        spacing: 12
        Item {
            Layout.preferredWidth: 24
            Layout.preferredHeight: 28
            Layout.alignment: Qt.AlignTop
            Layout.topMargin: 2
            Canvas {
                anchors.fill: parent
                visible: !root.downloading
                property color sheetColor: root.badgeColor
                onSheetColorChanged: requestPaint()
                onPaint: {
                    const ctx = getContext("2d")
                    ctx.reset()
                    ctx.fillStyle = sheetColor
                    ctx.beginPath()
                    ctx.moveTo(3, 0); ctx.lineTo(16, 0); ctx.lineTo(24, 8)
                    ctx.lineTo(24, 25); ctx.quadraticCurveTo(24, 28, 21, 28)
                    ctx.lineTo(3, 28); ctx.quadraticCurveTo(0, 28, 0, 25)
                    ctx.lineTo(0, 3); ctx.quadraticCurveTo(0, 0, 3, 0)
                    ctx.fill()
                    ctx.fillStyle = Qt.lighter(sheetColor, 1.6)
                    ctx.beginPath(); ctx.moveTo(16, 0); ctx.lineTo(16, 8); ctx.lineTo(24, 8)
                    ctx.closePath(); ctx.fill()
                }
            }
            Label {
                anchors.horizontalCenter: parent.horizontalCenter
                y: 14
                visible: !root.downloading
                text: root.fileType.slice(0, 4)
                textFormat: Text.PlainText
                font.pixelSize: 7
                font.bold: true
                color: Theme.primaryText
            }
            BusyIndicator {
                anchors.fill: parent
                running: root.downloading
                visible: running
            }
        }
        ColumnLayout {
            Layout.fillWidth: true
            Layout.minimumWidth: 0
            spacing: 4
            Label {
                objectName: "documentFileName"
                Layout.fillWidth: true
                Layout.minimumWidth: 0
                text: root.fileName
                textFormat: Text.PlainText
                color: Theme.text
                font.pixelSize: 14
                wrapMode: Text.Wrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }
            Label {
                objectName: "documentFileDetails"
                Layout.fillWidth: true
                text: root.downloading ? qsTr("Downloading…") : root.fileType + (root.fileSize ? " · " + root.fileSize : "")
                textFormat: Text.PlainText
                color: Theme.textMuted
                font.pixelSize: 11
                elide: Text.ElideRight
            }
        }
    }
}
