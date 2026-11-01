package space.nimbusgo.nativekit

import android.Manifest
import android.content.ContentUris
import android.content.ContentValues
import android.content.pm.PackageManager
import android.provider.CalendarContract
import android.provider.CalendarContract.Calendars
import android.provider.CalendarContract.Events
import android.provider.CalendarContract.Instances
import org.json.JSONObject
import java.util.TimeZone
import kotlin.concurrent.thread

/** calendarevents: the calendar provider. */
internal object CalendarEventsPlugin {
    private const val ACCOUNT = "Nimbus"
    private val needed = listOf(Manifest.permission.READ_CALENDAR, Manifest.permission.WRITE_CALENDAR)
    private val resolver get() = Plugins.context.contentResolver

    private fun missing() = needed.filter { Plugins.context.checkSelfPermission(it) != PackageManager.PERMISSION_GRANTED }

    fun handle(call: PluginCall) {
        if (call.method == "request") return request(call)
        if (missing().isNotEmpty()) return call.reject(Plugins.DENIED, "calendars aren't allowed: call Request first")
        thread {
            try {
                when (call.method) {
                    "calendars" -> call.resolve(calendars())
                    "newCalendar" -> newCalendar(call)
                    "deleteCalendar" -> {
                        val n = resolver.delete(asSync(ContentUris.withAppendedId(Calendars.CONTENT_URI, id(call))), null, null)
                        if (n == 0) call.reject(Plugins.NOT_FOUND, "no such calendar") else call.resolve()
                    }
                    "list" -> list(call)
                    "create" -> create(call)
                    "update" -> {
                        val v = values(call)
                        val c = call.string("calendarId")
                        if (!c.isNullOrEmpty()) v.put(Events.CALENDAR_ID, c.toLong())
                        val n = resolver.update(ContentUris.withAppendedId(Events.CONTENT_URI, id(call)), v, null, null)
                        if (n == 0) call.reject(Plugins.NOT_FOUND, "no such event") else call.resolve()
                    }
                    "delete" -> {
                        val n = resolver.delete(ContentUris.withAppendedId(Events.CONTENT_URI, id(call)), null, null)
                        if (n == 0) call.reject(Plugins.NOT_FOUND, "no such event") else call.resolve()
                    }
                    else -> Plugins.unknownMethod(call)
                }
            } catch (e: Exception) {
                call.reject(Plugins.FAILED, e.toString())
            }
        }
    }

    private fun id(call: PluginCall) = call.string("id")?.toLongOrNull() ?: -1

    private fun request(call: PluginCall) {
        val next = missing().firstOrNull() ?: return call.resolve(mapOf("granted" to true))
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
        PermissionAsker.ask(a, next) { ok -> if (ok) request(call) else call.resolve(mapOf("granted" to false)) }
    }

    /** A calendar of the app's own has to be written as its account's sync adapter. */
    private fun asSync(uri: android.net.Uri) = uri.buildUpon().appendQueryParameter(CalendarContract.CALLER_IS_SYNCADAPTER, "true")
        .appendQueryParameter(Calendars.ACCOUNT_NAME, ACCOUNT).appendQueryParameter(Calendars.ACCOUNT_TYPE, CalendarContract.ACCOUNT_TYPE_LOCAL).build()

    private fun calendars(): List<Map<String, Any>> {
        val out = ArrayList<Map<String, Any>>()
        val columns = arrayOf(Calendars._ID, Calendars.CALENDAR_DISPLAY_NAME, Calendars.CALENDAR_ACCESS_LEVEL, Calendars.IS_PRIMARY)
        resolver.query(Calendars.CONTENT_URI, columns, "${Calendars.VISIBLE} = 1", null, null)?.use {
            while (it.moveToNext()) {
                out.add(mapOf("id" to it.getString(0), "title" to (it.getString(1) ?: ""),
                    "writable" to (it.getInt(2) >= Calendars.CAL_ACCESS_CONTRIBUTOR), "default" to (it.getInt(3) == 1)))
            }
        }
        // The first writable one, when the phone has no primary.
        if (out.none { it["default"] == true }) out.indexOfFirst { it["writable"] == true }.takeIf { it >= 0 }?.let { out[it] = out[it] + ("default" to true) }
        return out
    }

    private fun newCalendar(call: PluginCall) {
        val title = call.string("title") ?: "Calendar"
        val v = ContentValues().apply {
            put(Calendars.ACCOUNT_NAME, ACCOUNT)
            put(Calendars.ACCOUNT_TYPE, CalendarContract.ACCOUNT_TYPE_LOCAL)
            put(Calendars.NAME, title)
            put(Calendars.CALENDAR_DISPLAY_NAME, title)
            put(Calendars.CALENDAR_COLOR, 0xff0d7377.toInt())
            put(Calendars.CALENDAR_ACCESS_LEVEL, Calendars.CAL_ACCESS_OWNER)
            put(Calendars.OWNER_ACCOUNT, ACCOUNT)
            put(Calendars.VISIBLE, 1)
            put(Calendars.SYNC_EVENTS, 1)
            put(Calendars.CALENDAR_TIME_ZONE, TimeZone.getDefault().id)
        }
        val uri = resolver.insert(asSync(Calendars.CONTENT_URI), v) ?: return call.reject(Plugins.FAILED, "the calendar couldn't be made")
        call.resolve(mapOf("id" to uri.lastPathSegment))
    }

    private fun list(call: PluginCall) {
        val from = (call.args.optDouble("from") * 1000).toLong()
        val to = (call.args.optDouble("to") * 1000).toLong()
        val columns = arrayOf(Instances.EVENT_ID, Instances.CALENDAR_ID, Instances.TITLE, Instances.EVENT_LOCATION, Instances.DESCRIPTION,
            Instances.BEGIN, Instances.END, Instances.ALL_DAY)
        val out = ArrayList<JSONObject>()
        Instances.query(resolver, columns, from, to)?.use {
            while (it.moveToNext()) {
                out.add(JSONObject().put("id", it.getString(0)).put("calendarId", it.getString(1)).put("title", it.getString(2) ?: "")
                    .put("location", it.getString(3) ?: "").put("notes", it.getString(4) ?: "").put("start", it.getLong(5) / 1000.0)
                    .put("end", it.getLong(6) / 1000.0).put("allDay", it.getInt(7) == 1))
            }
        }
        call.resolve(out.sortedBy { it.getDouble("start") })
    }

    private fun values(call: PluginCall) = ContentValues().apply {
        val all = call.args.optBoolean("allDay")
        put(Events.TITLE, call.string("title") ?: "")
        put(Events.EVENT_LOCATION, call.string("location") ?: "")
        put(Events.DESCRIPTION, call.string("notes") ?: "")
        put(Events.DTSTART, (call.args.optDouble("start") * 1000).toLong())
        put(Events.DTEND, (call.args.optDouble("end") * 1000).toLong())
        put(Events.ALL_DAY, if (all) 1 else 0)
        put(Events.EVENT_TIMEZONE, if (all) "UTC" else TimeZone.getDefault().id)
    }

    private fun create(call: PluginCall) {
        val calendar = call.string("calendarId")?.takeIf { it.isNotEmpty() } ?: calendars().firstOrNull { it["default"] == true }?.get("id") as String?
            ?: return call.reject(Plugins.UNAVAILABLE, "this phone has no calendar to add to: make one with NewCalendar")
        val v = values(call).apply { put(Events.CALENDAR_ID, calendar.toLong()) }
        val uri = resolver.insert(Events.CONTENT_URI, v) ?: return call.reject(Plugins.FAILED, "the event couldn't be saved")
        call.resolve(mapOf("id" to uri.lastPathSegment))
    }
}
