import QtQuick
import QtMultimedia
import QtCore

Item {
    id: root

    readonly property bool recording: recorder.recorderState === MediaRecorder.RecordingState
    signal finished(url path)
    signal failed(string message)

    function start() {
        recorder.outputLocation = StandardPaths.writableLocation(StandardPaths.TempLocation)
                + "/whatsappgo-voice-" + Date.now() + ".ogg"
        recorder.record()
    }

    function stop() {
        recorder.stop()
    }

    AudioInput { id: microphone }

    MediaRecorder {
        id: recorder
        mediaFormat.fileFormat: MediaFormat.Ogg
        mediaFormat.audioCodec: MediaFormat.AudioCodec.Opus
        onRecorderStateChanged: {
            if (recorderState === MediaRecorder.StoppedState && actualLocation)
                root.finished(actualLocation)
        }
        onErrorOccurred: (error, errorString) => root.failed(errorString)
    }

    CaptureSession {
        audioInput: microphone
        recorder: recorder
    }
}
