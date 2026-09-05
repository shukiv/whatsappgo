pragma Singleton
import QtQuick

// WhatsApp Web only prints a clock time for today. Yesterday and the rest of
// the week are named, and anything older falls back to a date. Chat rows and
// search results share the rule so a conversation reads the same in both.
QtObject {
    function label(epochMillis) {
        const value = Number(epochMillis || 0)
        if (value <= 0)
            return ""
        const when = new Date(value)
        const now = new Date()
        const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
        if (when.getTime() >= startOfToday)
            return Qt.formatTime(when, "HH:mm")
        if (when.getTime() >= startOfToday - 24 * 60 * 60 * 1000)
            return qsTr("Yesterday")
        if (when.getTime() >= startOfToday - 6 * 24 * 60 * 60 * 1000)
            return when.toLocaleDateString(Qt.locale(), "dddd")
        return when.toLocaleDateString(Qt.locale(), Locale.ShortFormat)
    }

    // daySeparator names the day a run of messages belongs to, for the pill
    // WhatsApp Web puts between one day and the next. Today and yesterday are
    // named, the rest of the week is its weekday, and older days are a date.
    function daySeparator(epochMillis) {
        const value = Number(epochMillis || 0)
        if (value <= 0)
            return ""
        const when = new Date(value)
        const now = new Date()
        const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
        const day = 24 * 60 * 60 * 1000
        if (when.getTime() >= startOfToday)
            return qsTr("Today")
        if (when.getTime() >= startOfToday - day)
            return qsTr("Yesterday")
        if (when.getTime() >= startOfToday - 6 * day)
            return when.toLocaleDateString(Qt.locale(), "dddd")
        return when.toLocaleDateString(Qt.locale(), Locale.ShortFormat)
    }
}
