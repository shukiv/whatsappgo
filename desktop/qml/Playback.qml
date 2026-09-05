pragma Singleton
import QtQuick
import QtMultimedia

// Playback owns the one media player the application uses.
//
// Voice notes, audio files, and videos play inside the window. Handing them to
// the desktop instead opened a browser on systems without a registered handler
// for Opus audio, which is not a media player.
//
// A single shared player also means starting one voice note stops the previous
// one, and only one decoder exists no matter how long the conversation is.
Item {
    id: root

    // Identity of the message being played, so a delegate can show its own
    // progress without holding a player of its own.
    property string currentId: ""
    property string currentPath: ""
    property bool isVideo: false
    // Set by the window to the surface video frames are drawn on.
    property var videoSurface: null
    // The message waiting for its file to finish downloading.
    property string pendingId: ""
    property bool pendingIsVideo: false
    // Loading a MediaPlayer and loading Main's video overlay are separate
    // operations. Keep the requested start until both objects exist so the
    // first frames cannot be sent to a null VideoOutput.
    property bool startPending: false
	property string announcedId: ""

    readonly property bool active: currentId !== ""
    readonly property bool playing: loader.item ? loader.item.playbackState === MediaPlayer.PlayingState : false
    readonly property int position: loader.item ? loader.item.position : 0
    readonly property int duration: loader.item ? loader.item.duration : 0
    readonly property bool videoActive: active && isVideo
    readonly property bool waitingForVideoSurface: startPending && isVideo && !videoSurface

    signal downloadRequested(string messageId)
    signal failed(string message)
    // Emitted when a recording reaches its end, so the conversation can move
    // on to the next one the way WhatsApp plays a run of voice notes.
    signal finished(string messageId)
	// Fired only after the media player has actually been asked to play. This
	// keeps deferred downloads from being acknowledged on the initial click.
	signal started(string messageId)

    function isCurrent(messageId) {
        return messageId !== "" && messageId === currentId
    }

    function beginPlayback() {
        if (!startPending || !loader.item)
            return
        if (isVideo && !videoSurface)
            return
        loader.item.source = Theme.fileUrl(currentPath)
        loader.item.play()
        startPending = false
    }

    // start plays a message, downloading it first when it is not cached yet.
    function start(messageId, path, video) {
        if (!messageId)
            return
        if (isCurrent(messageId)) {
            toggle()
            return
        }
        if (!path) {
            pendingId = messageId
            pendingIsVideo = Boolean(video)
            downloadRequested(messageId)
            return
        }
        pendingId = ""
        currentId = messageId
        currentPath = String(path)
        isVideo = Boolean(video)
        startPending = true
        loader.active = true
        beginPlayback()
    }

    // fileReady resumes a start() that was waiting for a download.
    function fileReady(messageId, path) {
        if (messageId !== pendingId || !path)
            return
        const video = pendingIsVideo
        pendingId = ""
        start(messageId, path, video)
    }

    function toggle() {
        if (!loader.item)
            return
        if (loader.item.playbackState === MediaPlayer.PlayingState)
            loader.item.pause()
        else
            loader.item.play()
    }

    function stop() {
        pendingId = ""
        startPending = false
        if (loader.item) {
            loader.item.stop()
            loader.item.source = ""
        }
        currentId = ""
        currentPath = ""
        isVideo = false
    }

    onVideoSurfaceChanged: beginPlayback()

    function seek(milliseconds) {
        if (loader.item && loader.item.seekable)
            loader.item.position = Math.max(0, Math.min(milliseconds, loader.item.duration))
    }

    // The player is created on first use. Declaring it eagerly would start the
    // multimedia backend during application startup, which is both wasteful and
    // noisy on machines without a working audio stack.
    Loader {
        id: loader
        active: false
        onLoaded: root.beginPlayback()
        sourceComponent: Component {
            MediaPlayer {
                audioOutput: AudioOutput {}
                videoOutput: root.videoSurface
				onPlaybackStateChanged: {
					if (playbackState === MediaPlayer.PlayingState && root.announcedId !== root.currentId) {
						root.announcedId = root.currentId
						root.started(root.currentId)
					}
				}
                onErrorOccurred: (error, errorString) => {
                    root.failed(errorString)
                    root.stop()
                }
                onMediaStatusChanged: {
                    if (mediaStatus !== MediaPlayer.EndOfMedia || root.isVideo)
                        return
                    const completed = root.currentId
                    root.stop()
                    root.finished(completed)
                }
            }
        }
    }
}
