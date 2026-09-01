import QtQuick
import QtQuick.Controls
import org.whatsappgo

WhatsAppMenuPopup {
    id: root
    objectName: "attachmentMenu"
    width: 224

    signal documentRequested()
    signal photosVideosRequested()
    signal audioRequested()
    signal unavailableRequested(string feature)

    onOpened: documentAction.forceActiveFocus()

    WhatsAppMenuItem {
        id: documentAction
        objectName: "attachmentDocument"
        text: qsTr("Document")
        iconSource: Qt.resolvedUrl("icons/document.svg")
        iconTint: Theme.attachmentDocument
        onClicked: {
            root.close()
            root.documentRequested()
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentPhotosVideos"
        text: qsTr("Photos and videos")
        iconSource: Qt.resolvedUrl("icons/gallery.svg")
        iconTint: Theme.attachmentMedia
        onClicked: {
            root.close()
            root.photosVideosRequested()
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentCamera"
        text: qsTr("Camera")
        iconSource: Qt.resolvedUrl("icons/camera.svg")
        iconTint: Theme.attachmentCamera
        Accessible.description: qsTr("Camera capture is not supported by the linked-device API yet")
        onClicked: {
            root.close()
            root.unavailableRequested(text)
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentAudio"
        text: qsTr("Audio")
        iconSource: Qt.resolvedUrl("icons/headphones.svg")
        iconTint: Theme.attachmentAudio
        onClicked: {
            root.close()
            root.audioRequested()
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentContact"
        text: qsTr("Contact")
        iconSource: Qt.resolvedUrl("icons/contact.svg")
        iconTint: Theme.attachmentContact
        Accessible.description: qsTr("Sending contact cards is not supported by the linked-device API yet")
        onClicked: {
            root.close()
            root.unavailableRequested(text)
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentPoll"
        text: qsTr("Poll")
        iconSource: Qt.resolvedUrl("icons/poll.svg")
        iconTint: Theme.attachmentPoll
        Accessible.description: qsTr("Creating polls is not supported by the linked-device API yet")
        onClicked: {
            root.close()
            root.unavailableRequested(text)
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentEvent"
        text: qsTr("Event")
        iconSource: Qt.resolvedUrl("icons/calendar.svg")
        iconTint: Theme.attachmentEvent
        Accessible.description: qsTr("Creating events is not supported by the linked-device API yet")
        onClicked: {
            root.close()
            root.unavailableRequested(text)
        }
    }

    WhatsAppMenuItem {
        objectName: "attachmentSticker"
        text: qsTr("New sticker")
        iconSource: Qt.resolvedUrl("icons/sticker.svg")
        iconTint: Theme.attachmentSticker
        Accessible.description: qsTr("Creating stickers is not supported by the linked-device API yet")
        onClicked: {
            root.close()
            root.unavailableRequested(text)
        }
    }
}
