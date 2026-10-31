package space.nimbusgo.nativekit

import android.content.Intent
import android.provider.CalendarContract
import android.provider.CalendarContract.Events

/** calendar: the calendar app's new event screen. */
internal object CalendarPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "add") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        val i = Intent(Intent.ACTION_INSERT).setData(Events.CONTENT_URI).putExtra(Events.TITLE, call.string("title"))
            .putExtra(Events.EVENT_LOCATION, call.string("location")).putExtra(Events.DESCRIPTION, call.string("notes"))
            .putExtra(CalendarContract.EXTRA_EVENT_BEGIN_TIME, call.args.optLong("start") * 1000)
            .putExtra(CalendarContract.EXTRA_EVENT_END_TIME, call.args.optLong("end") * 1000)
        ResultAsker.start(a, i, { call.reject(Plugins.UNAVAILABLE, "this phone has no calendar app") }) { _, _ ->
            call.resolve(mapOf("saved" to true))
        }
    }
}
