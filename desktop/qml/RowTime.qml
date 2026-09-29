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

    // untilLabel says how long there is still to wait for something set to
    // happen later.
    //
    // It is deliberately approximate above the hour. Somebody who scheduled a
    // message wants to know whether it goes this morning or tomorrow, not that
    // seven hours and fifty-two minutes remain, and a figure that precise on a
    // label nobody is watching is only a thing to be wrong about.
    function untilLabel(epochMillis, nowMillis) {
        const target = Number(epochMillis || 0)
        if (target <= 0)
            return ""
        const remaining = target - Number(nowMillis || Date.now())
        const minute = 60 * 1000
        const hour = 60 * minute
        const day = 24 * hour
        // The daemon looks every ten seconds for messages whose time has come,
        // so one whose moment has passed is already on its way out.
        if (remaining <= 0)
            return qsTr("any moment now")
        if (remaining < minute)
            return qsTr("in under a minute")
        // Each unit is chosen from its own rounded value, so fifty-nine and a
        // half minutes reads as about an hour rather than as sixty minutes.
        //
        // The two forms are written out rather than left to %n. No translation
        // is shipped, and without one Qt hands back the source text with the
        // number put in and the brackets left where they are, so "%n minute(s)"
        // reaches the reader as "in 5 minute(s)".
        const minutes = Math.round(remaining / minute)
        if (minutes < 60)
            return minutes === 1 ? qsTr("in 1 minute") : qsTr("in %1 minutes").arg(minutes)
        const hours = Math.round(remaining / hour)
        if (hours < 24)
            return hours === 1 ? qsTr("in about 1 hour") : qsTr("in about %1 hours").arg(hours)
        const days = Math.round(remaining / day)
        return days === 1 ? qsTr("in about 1 day") : qsTr("in about %1 days").arg(days)
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
