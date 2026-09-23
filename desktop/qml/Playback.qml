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
    // Video controls do not alter the volume or speed of voice notes.
    property bool videoMuted: false
    property real videoVolume: 1.0
    property real videoRate: 1.0
    readonly property bool seekable: loader.item ? loader.item.seekable : false
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
    // Where playback should begin, as a share of the whole recording. Dragging
    // the handle of a recording that is not playing asks for a position the
    // player does not know yet: it has no file open, so it has no length to
    // measure the position against. The share is kept until it does.
    property real startFraction: 0

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

    // finishCurrent stops what is playing without cancelling a recording that
    // is still being downloaded: one recording ending used to throw away the
    // request for the next one, which then never played.
    function finishCurrent() {
        startPending = false
        startFraction = 0
        if (loader.item) {
            loader.item.stop()
            loader.item.source = ""
        }
        currentId = ""
        currentPath = ""
        isVideo = false
    }

    function stop() {
        pendingId = ""
        finishCurrent()
    }

    onVideoSurfaceChanged: beginPlayback()

    function seek(milliseconds) {
        if (readyToSeek())
            loader.item.position = Math.max(0, Math.min(milliseconds, loader.item.duration))
    }

    // seekFraction moves within a recording by a share of its whole, which is
    // what dragging a handle across a waveform produces. A recording that is
    // not playing starts from where the handle was left rather than from the
    // beginning: the handle was moved to be listened from.
    function seekFraction(messageId, path, fraction) {
        const target = Math.max(0, Math.min(1, Number(fraction) || 0))
        if (isCurrent(messageId)) {
            if (readyToSeek()) {
                seek(duration * target)
                return
            }
            startFraction = target
            return
        }
        startFraction = target
        start(messageId, path, false)
    }

    // applyStartFraction places a kept position as soon as the player knows
    // enough to accept one. Length, seekability and the file itself arrive
    // separately and in no fixed order, so all three are watched.
    //
    // The share is cleared before the position is written, not after. Moving
    // the position makes the decoder re-announce what it knows, and that
    // arrives back here: a share still set would be placed again, and again,
    // until the stack ran out.
    function applyStartFraction() {
        if (startFraction <= 0 || !readyToSeek())
            return
        const share = startFraction
        startFraction = 0
        loader.item.position = Math.max(0, Math.min(loader.item.duration,
                                                    loader.item.duration * share))
    }

    // readyToSeek is true only once the file is open and measured. Moving
    // within a stream the decoder is still reading is what makes it report the
    // samples it had to skip.
    function readyToSeek() {
        return loader.item
            && loader.item.seekable
            && loader.item.duration > 0
            && (loader.item.mediaStatus === MediaPlayer.LoadedMedia
                || loader.item.mediaStatus === MediaPlayer.BufferedMedia
                || loader.item.mediaStatus === MediaPlayer.BufferingMedia)
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
                audioOutput: AudioOutput {
                    muted: root.isVideo && root.videoMuted
                    volume: root.isVideo ? Math.max(0, Math.min(1, root.videoVolume)) : 1
                }
                playbackRate: root.isVideo ? Math.max(0.5, Math.min(2, root.videoRate)) : 1
                videoOutput: root.videoSurface
                onDurationChanged: root.applyStartFraction()
                onSeekableChanged: root.applyStartFraction()
				onPlaybackStateChanged: {
					if (playbackState === MediaPlayer.PlayingState && root.announcedId !== root.currentId) {
						root.announcedId = root.currentId
						root.started(root.currentId)
					}
				}
                onErrorOccurred: (error, errorString) => {
                    root.failed(errorString)
                    root.finishCurrent()
                }
                onMediaStatusChanged: {
                    // The moment the file is open is the moment a position
                    // asked for before it opened can be placed.
                    root.applyStartFraction()
                    if (mediaStatus !== MediaPlayer.EndOfMedia || root.isVideo)
                        return
                    const completed = root.currentId
                    root.finishCurrent()
                    root.finished(completed)
                }
            }
        }
    }
}
