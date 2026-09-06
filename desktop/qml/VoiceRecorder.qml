import QtQuick
import QtMultimedia
import QtCore

Item {
    id: root

    readonly property bool recording: recorder.recorderState === MediaRecorder.RecordingState
    signal finished(url path)
    signal failed(string message)
    property bool sendWhenStopped: false

    function start() {
        sendWhenStopped = true
        recorder.outputLocation = StandardPaths.writableLocation(StandardPaths.TempLocation)
                + "/whatsappgo-voice-" + Date.now() + ".ogg"
        recorder.record()
    }

    function stop() {
        recorder.stop()
    }

    function cancel() {
        // Set this before stop(): a stop can synchronously emit the state change.
        sendWhenStopped = false
        recorder.stop()
    }

    AudioInput { id: microphone }

    MediaRecorder {
        id: recorder
        mediaFormat.fileFormat: MediaFormat.Ogg
        mediaFormat.audioCodec: MediaFormat.AudioCodec.Opus
        onRecorderStateChanged: {
            if (recorderState === MediaRecorder.StoppedState) {
                const shouldSend = root.sendWhenStopped
                root.sendWhenStopped = false
                if (shouldSend && actualLocation)
                    root.finished(actualLocation)
            }
        }
        onErrorOccurred: (error, errorString) => root.failed(errorString)
    }

    CaptureSession {
        audioInput: microphone
        recorder: recorder
    }
}
