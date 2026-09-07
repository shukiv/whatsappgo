import QtQuick
import QtQuick.Window

// Semantic names stay compatible with existing callers; artwork is bundled
// Lucide SVG, rendered by QtSvg without a GPU-only colorization effect.
Item {
    id: root

    property url source
    property color tint: "black"
    readonly property string kind: {
        const parts = String(source).split("/")
        return parts[parts.length - 1].replace(".svg", "")
    }

    implicitWidth: 24
    implicitHeight: 24
    Accessible.ignored: true

    Image {
        readonly property int pixelEdge: Math.max(1, Math.ceil(width * Screen.devicePixelRatio))
        anchors.centerIn: parent
        width: Math.min(root.width, root.height)
        height: width
        source: root.kind === "" ? ""
            : "image://lucide/" + root.kind + "/" + String(root.tint).substring(1)
        sourceSize: Qt.size(pixelEdge, pixelEdge)
        fillMode: Image.PreserveAspectFit
        cache: true
        smooth: true
        Accessible.ignored: true
    }
}
