import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import org.whatsappgo

Popup {
    id: root
    objectName: "emojiPicker"
    signal emojiChosen(string emoji)

    width: 390
    height: 360
    padding: 0
    modal: false
    focus: true
    // Nothing may paint outside the card. Without this the emoji grid drew
    // past the rounded background and over the conversation behind it.
    clip: true
    closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside

    property int selectedCategory: 0
    readonly property var categories: [
        {
            name: qsTr("Smileys and people"), icon: "😀", items: [
                "😀|grinning face happy", "😃|smiling face happy", "😄|smile eyes happy", "😁|beaming face",
                "😆|laughing squinting", "😅|sweat smile", "😂|tears joy laughing", "🤣|rolling laughing",
                "😊|blush smile", "😇|angel halo", "🙂|slightly smiling", "🙃|upside down",
                "😉|wink", "😌|relieved", "😍|heart eyes love", "🥰|smiling hearts love",
                "😘|kiss", "😋|delicious yummy", "😜|wink tongue", "🤪|zany silly",
                "🤔|thinking", "🫡|salute", "🤗|hug", "🤭|hand over mouth",
                "🫢|gasp", "🤫|quiet shush", "🤐|zipper mouth", "🤨|raised eyebrow",
                "😐|neutral", "😑|expressionless", "😶|no mouth", "🫥|dotted face",
                "🙄|rolling eyes", "😏|smirk", "😒|unamused", "😬|grimace",
                "🤥|lying pinocchio", "😔|sad pensive", "🥺|pleading", "😢|cry",
                "😭|sobbing crying", "😤|triumph steam", "😠|angry", "😡|rage",
                "🤬|swearing", "🤯|mind blown", "😳|flushed", "🥵|hot face",
                "🥶|cold face", "😱|scream fear", "😨|fearful", "😰|anxious sweat",
                "🤢|nauseated", "🤮|vomit", "🤧|sneeze", "😷|mask",
                "🤒|thermometer sick", "🤕|bandage hurt", "😴|sleep", "🥱|yawn",
                "😎|sunglasses cool", "🤓|nerd", "🧐|monocle", "🥳|party",
                "🥸|disguise", "😈|smiling devil", "👿|angry devil", "💩|poop",
                "👋|wave hello goodbye", "🤚|raised hand", "🖐️|hand fingers", "✋|stop hand",
                "👌|ok hand", "🤌|pinched fingers", "🤏|pinching", "✌️|victory peace",
                "🤞|fingers crossed", "🫰|finger heart", "🤟|love you gesture", "🤘|rock hand",
                "🤙|call me", "👈|point left", "👉|point right", "👆|point up",
                "👇|point down", "☝️|index up", "👍|thumbs up like", "👎|thumbs down dislike",
                "✊|fist", "👊|punch", "🤛|left fist", "🤜|right fist",
                "👏|clap applause", "🙌|celebrate hands", "🫶|heart hands", "🤝|handshake",
                "🙏|pray thanks please", "💪|muscle strong", "🫂|people hugging", "👀|eyes"
            ]
        },
        {
            name: qsTr("Animals and nature"), icon: "🐻", items: [
                "🐶|dog", "🐱|cat", "🐭|mouse", "🐹|hamster", "🐰|rabbit", "🦊|fox",
                "🐻|bear", "🐼|panda", "🐨|koala", "🐯|tiger", "🦁|lion", "🐮|cow",
                "🐷|pig", "🐸|frog", "🐵|monkey", "🙈|see no evil", "🙉|hear no evil",
                "🙊|speak no evil", "🐔|chicken", "🐧|penguin", "🐦|bird", "🐤|chick",
                "🦄|unicorn", "🐝|bee", "🦋|butterfly", "🐌|snail", "🐞|ladybug",
                "🐢|turtle", "🐍|snake", "🦎|lizard", "🐙|octopus", "🦑|squid",
                "🐠|fish", "🐬|dolphin", "🐳|whale", "🦈|shark", "🐊|crocodile",
                "🐅|tiger", "🐆|leopard", "🦓|zebra", "🦍|gorilla", "🦧|orangutan",
                "🐘|elephant", "🦛|hippo", "🦏|rhino", "🐪|camel", "🦒|giraffe",
                "🌵|cactus", "🎄|christmas tree", "🌲|evergreen", "🌴|palm tree",
                "🌱|seedling", "🌿|herb", "☘️|shamrock", "🍀|four leaf clover",
                "🌹|rose", "🌷|tulip", "🌻|sunflower", "🌞|sun face", "🌙|moon",
                "⭐|star", "🌈|rainbow", "🔥|fire", "💧|water drop", "❄️|snowflake"
            ]
        },
        {
            name: qsTr("Food and drink"), icon: "🍔", items: [
                "🍏|green apple", "🍎|red apple", "🍐|pear", "🍊|orange", "🍋|lemon",
                "🍌|banana", "🍉|watermelon", "🍇|grapes", "🍓|strawberry", "🫐|blueberries",
                "🍈|melon", "🍒|cherries", "🍑|peach", "🥭|mango", "🍍|pineapple",
                "🥥|coconut", "🥑|avocado", "🍅|tomato", "🥕|carrot", "🌽|corn",
                "🌶️|pepper", "🥐|croissant", "🍞|bread", "🥨|pretzel", "🧀|cheese",
                "🍳|egg", "🥞|pancakes", "🥓|bacon", "🍗|chicken", "🍔|burger",
                "🍟|fries", "🍕|pizza", "🌭|hot dog", "🥪|sandwich", "🌮|taco",
                "🍝|pasta", "🍜|noodles", "🍣|sushi", "🍚|rice", "🍦|ice cream",
                "🍩|doughnut", "🍪|cookie", "🎂|birthday cake", "🍫|chocolate", "🍿|popcorn",
                "☕|coffee", "🫖|tea", "🥤|cup straw", "🍺|beer", "🍷|wine",
                "🥂|cheers", "🍹|cocktail", "🧉|mate", "🧊|ice"
            ]
        },
        {
            name: qsTr("Activities"), icon: "⚽", items: [
                "⚽|football soccer", "🏀|basketball", "🏈|american football", "⚾|baseball",
                "🥎|softball", "🎾|tennis", "🏐|volleyball", "🏉|rugby", "🥏|flying disc",
                "🎱|pool billiards", "🏓|table tennis", "🏸|badminton", "🥊|boxing",
                "🥋|martial arts", "⛳|golf", "⛸️|ice skate", "🎣|fishing", "🤿|diving",
                "🎿|ski", "🏂|snowboard", "🏋️|weights", "🚴|cycling", "🏆|trophy",
                "🥇|gold medal", "🎯|target", "🎮|video game", "🕹️|joystick", "🎲|dice",
                "♟️|chess", "🎨|art palette", "🎭|theater", "🎤|microphone", "🎧|headphones",
                "🎸|guitar", "🎹|piano", "🥁|drum", "🎬|movie", "🎉|party popper",
                "🎊|confetti", "🎁|gift", "🎈|balloon"
            ]
        },
        {
            name: qsTr("Travel and places"), icon: "🚗", items: [
                "🚗|car", "🚕|taxi", "🚌|bus", "🚎|trolleybus", "🏎️|race car",
                "🚓|police car", "🚑|ambulance", "🚒|fire engine", "🚚|truck", "🏍️|motorcycle",
                "🚲|bicycle", "🛴|scooter", "🚆|train", "🚇|metro", "🚊|tram",
                "✈️|airplane", "🚀|rocket", "🛸|flying saucer", "🚁|helicopter", "⛵|sailboat",
                "🚢|ship", "⚓|anchor", "⛽|fuel", "🚦|traffic light", "🗺️|map",
                "🗿|moai", "🗽|statue liberty", "🗼|tower", "🏰|castle", "🏠|home",
                "🏢|office", "🏥|hospital", "🏫|school", "⛺|tent", "🏖️|beach",
                "🏝️|island", "🏜️|desert", "🌋|volcano", "⛰️|mountain", "🌍|earth"
            ]
        },
        {
            name: qsTr("Objects"), icon: "💡", items: [
                "⌚|watch", "📱|phone", "💻|laptop", "⌨️|keyboard", "🖥️|desktop computer",
                "🖨️|printer", "🖱️|mouse", "📷|camera", "📹|video camera", "📺|television",
                "📻|radio", "⏰|alarm clock", "💡|light bulb", "🔦|flashlight", "🕯️|candle",
                "💰|money bag", "💳|credit card", "💎|gem", "⚖️|scales", "🔧|wrench",
                "🔨|hammer", "🛠️|tools", "🧲|magnet", "🔫|water pistol", "💊|pill",
                "🩹|bandage", "🧸|teddy bear", "🛒|shopping cart", "🎒|backpack", "👓|glasses",
                "🕶️|sunglasses", "☂️|umbrella", "📌|pin", "📎|paperclip", "✂️|scissors",
                "🔒|lock", "🔑|key", "📣|megaphone", "🔔|bell", "✉️|envelope"
            ]
        },
        {
            name: qsTr("Symbols"), icon: "❤️", items: [
                "❤️|red heart love", "🧡|orange heart", "💛|yellow heart", "💚|green heart",
                "💙|blue heart", "💜|purple heart", "🖤|black heart", "🤍|white heart",
                "🤎|brown heart", "💔|broken heart", "❣️|heart exclamation", "💕|two hearts",
                "💞|revolving hearts", "💓|beating heart", "💗|growing heart", "💖|sparkling heart",
                "💘|heart arrow", "💝|heart ribbon", "💯|hundred", "💢|anger",
                "💥|collision", "💫|dizzy", "💦|sweat drops", "💨|dash", "🕳️|hole",
                "💬|speech bubble", "👁️‍🗨️|eye speech", "🗨️|left speech", "🗯️|anger bubble",
                "💭|thought bubble", "✅|check mark", "❌|cross mark", "❓|question",
                "❗|exclamation", "⚠️|warning", "🚫|prohibited", "🔞|18 adult", "♻️|recycle",
                "▶️|play", "⏸️|pause", "⏹️|stop", "🔄|refresh", "➕|plus", "➖|minus",
                "➗|divide", "➡️|right arrow", "⬅️|left arrow", "⬆️|up arrow", "⬇️|down arrow"
            ]
        },
        {
            name: qsTr("Flags"), icon: "🏳️", items: [
                "🏳️|white flag", "🏴|black flag", "🏁|chequered flag", "🚩|triangular flag",
                "🏳️‍🌈|rainbow pride flag", "🏳️‍⚧️|transgender flag", "🇮🇱|Israel flag", "🇺🇸|United States flag",
                "🇨🇦|Canada flag", "🇲🇽|Mexico flag", "🇨🇴|Colombia flag", "🇧🇷|Brazil flag",
                "🇦🇷|Argentina flag", "🇬🇧|United Kingdom flag", "🇪🇸|Spain flag", "🇫🇷|France flag",
                "🇩🇪|Germany flag", "🇮🇹|Italy flag", "🇳🇱|Netherlands flag", "🇵🇹|Portugal flag",
                "🇺🇦|Ukraine flag", "🇷🇺|Russia flag", "🇹🇷|Turkey flag", "🇬🇷|Greece flag",
                "🇮🇳|India flag", "🇨🇳|China flag", "🇯🇵|Japan flag", "🇰🇷|Korea flag",
                "🇦🇺|Australia flag", "🇿🇦|South Africa flag", "🇪🇬|Egypt flag", "🇦🇪|United Arab Emirates flag"
            ]
        }
    ]

    readonly property var visibleEntries: {
        const query = searchField.text.trim().toLowerCase()
        const source = []
        if (query.length > 0) {
            for (let category of categories)
                source.push(...category.items)
        } else {
            source.push(...categories[selectedCategory].items)
        }
        return source.filter(entry => entry.toLowerCase().indexOf(query) >= 0)
    }

    function glyph(entry) { return String(entry).split("|")[0] }
    function description(entry) { return String(entry).split("|")[1] || qsTr("Emoji") }

    onOpened: {
        searchField.clear()
        Qt.callLater(() => searchField.forceActiveFocus())
    }

    background: Rectangle {
        color: Theme.surfaceRaised
        radius: 12
        border.color: Theme.border
    }

    contentItem: ColumnLayout {
        spacing: 0

        RowLayout {
            Layout.fillWidth: true
            Layout.leftMargin: 10
            Layout.rightMargin: 10
            Layout.topMargin: 8
            Layout.bottomMargin: 6
            spacing: 2

            Repeater {
                model: root.categories
                ToolButton {
                    required property int index
                    required property var modelData
                    Layout.fillWidth: true
                    Layout.preferredHeight: 38
                    focusPolicy: Qt.TabFocus
                    Accessible.name: modelData.name
                    Accessible.checked: root.selectedCategory === index
                    ToolTip.visible: hovered
                    ToolTip.text: modelData.name
                    onClicked: {
                        root.selectedCategory = index
                        emojiGrid.positionViewAtBeginning()
                    }
                    contentItem: Label {
                        text: parent.modelData.icon
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                        font.family: Theme.emojiFontFamily
                        font.pixelSize: 19
                    }
                    background: Rectangle {
                        color: parent.hovered || parent.checked ? Theme.hoverRow : "transparent"
                        radius: 8
                        Rectangle {
                            visible: root.selectedCategory === index
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            anchors.leftMargin: 7
                            anchors.rightMargin: 7
                            height: 3
                            radius: 1.5
                            color: Theme.primary
                        }
                    }
                }
            }
        }

        TextField {
            id: searchField
            objectName: "emojiSearchField"
            Layout.fillWidth: true
            Layout.leftMargin: 12
            Layout.rightMargin: 12
            Layout.bottomMargin: 8
            implicitHeight: 42
            leftPadding: 14
            rightPadding: 14
            placeholderText: qsTr("Search emoji")
            Accessible.name: placeholderText
            color: Theme.text
            background: Rectangle {
                color: Theme.surfaceMuted
                radius: 21
                border.color: searchField.activeFocus ? Theme.primary : "transparent"
                border.width: 2
            }
            Keys.onDownPressed: emojiGrid.forceActiveFocus()
        }

        GridView {
            id: emojiGrid
            objectName: "emojiGrid"
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.leftMargin: 10
            Layout.rightMargin: 4
            Layout.bottomMargin: 8
            cellWidth: 46
            cellHeight: 44
            clip: true
            model: root.visibleEntries
            keyNavigationEnabled: true
            keyNavigationWraps: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: OverlayScrollBar {}

            delegate: ToolButton {
                required property int index
                required property var modelData
                width: emojiGrid.cellWidth
                height: emojiGrid.cellHeight
                focusPolicy: Qt.TabFocus
                Accessible.name: root.description(modelData)
                ToolTip.visible: hovered
                ToolTip.text: Accessible.name
                onClicked: {
                    root.emojiChosen(root.glyph(modelData))
                    root.close()
                }
                contentItem: Label {
                    text: root.glyph(parent.modelData)
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                    font.family: Theme.emojiFontFamily
                    font.pixelSize: 27
                }
                background: Rectangle {
                    color: parent.down ? Theme.pressedRow : parent.hovered || parent.activeFocus ? Theme.hoverRow : "transparent"
                    radius: 8
                    border.color: parent.activeFocus ? Theme.primary : "transparent"
                    border.width: parent.activeFocus ? 2 : 0
                }
            }

            Label {
                anchors.centerIn: parent
                visible: emojiGrid.count === 0
                text: qsTr("No emoji found")
                color: Theme.textMuted
            }
        }
    }
}
