import QtQuick
import QtMultimedia
import org.whatsappgo

// Created only after an explicit selection. Stopping unloads the decoder, so
// a long chat never accumulates a player or a frame cache for every message.
Item {
    id: root
    property url source
    property bool sticker: false
    property bool playing: true
    readonly property bool hasFrame: decoder.item ? decoder.item.hasFrame : false
    signal failed(string message)
    function stop() { playing = false }

    Loader {
        id: decoder
        anchors.fill: parent
        active: root.playing && root.visible && root.source.toString() !== ""
        sourceComponent: root.sticker ? stickerComponent : videoComponent
    }
    Component {
        id: stickerComponent
        StickerAnimation {
            objectName: "inlineStickerDecoder"
            source: root.source
            playing: true
            onErrorChanged: if (error !== "") root.failed(error)
        }
    }
    Component {
        id: videoComponent
        Item {
            readonly property alias hasFrame: video.hasFrame
            VideoSurface { id: video; anchors.fill: parent }
            MediaPlayer {
                id: player
                objectName: "inlineGifPlayer"
                source: root.source
                videoOutput: video
                loops: MediaPlayer.Infinite
                onMediaStatusChanged: if (mediaStatus === MediaPlayer.LoadedMedia) play()
                onErrorOccurred: (error, errorString) => root.failed(qsTr("Could not play GIF: %1").arg(errorString))
                // GIFs are silent; there is deliberately no AudioOutput.
            }
        }
    }
}
