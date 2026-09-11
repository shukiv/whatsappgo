import QtQuick
import org.whatsappgo

QtObject {
    id: root
    required property var client
    property string token: ""
    property bool busy: false
    property string error: ""
    property var result: ({})
    signal finished(var result, string error)
    function reset() { token = ""; busy = false; error = ""; result = ({}) }
    function run(method, params) {
        if (busy) return
        error = ""
        busy = true
        token = "feature-" + Date.now() + "-" + Math.random()
        client.requestInteractiveFeature(token, method, params)
    }
    property Connections connection: Connections {
        target: root.client
        function onProfileChanged() { root.reset() }
        function onInteractiveFeatureFinished(token, result, error) {
            if (!root.busy || token !== root.token) return
            root.busy = false
            root.error = error
            if (!error) root.result = result
            root.finished(result, error)
        }
    }
}
