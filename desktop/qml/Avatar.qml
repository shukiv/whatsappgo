import QtQuick
import QtQuick.Controls
import org.whatsappgo

Item {
    id: root
    property string title: ""
    property url source
    property bool fallbackIdentity: false
    property int diameter: 49

    implicitWidth: diameter
    implicitHeight: diameter
    Accessible.name: source ? qsTr("Profile picture for %1").arg(title) : qsTr("Default profile picture for %1").arg(title)

    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: Theme.avatar
    }
    Label {
        anchors.centerIn: parent
        visible: !root.fallbackIdentity && avatarSource.status !== Image.Ready
        text: root.title ? root.title.charAt(0).toUpperCase() : "?"
        color: Theme.avatarText
        font.pixelSize: Math.round(root.diameter * 0.39)
        font.weight: Font.DemiBold
    }
    TintedIcon {
        anchors.centerIn: parent
        visible: root.fallbackIdentity && avatarSource.status !== Image.Ready
        width: Math.round(root.diameter * 0.51)
        height: width
        source: Qt.resolvedUrl("icons/user.svg")
        tint: Theme.avatarText
        Accessible.ignored: true
    }
    Image {
        id: avatarSource
        anchors.fill: parent
        source: root.source
        sourceSize.width: root.diameter * 2
        sourceSize.height: root.diameter * 2
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        cache: true
        visible: status === Image.Ready
    }
}
