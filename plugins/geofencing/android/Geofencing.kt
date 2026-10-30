package space.nimbusgo.nativekit

import android.Manifest
import android.annotation.SuppressLint
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.location.LocationManager
import org.json.JSONArray
import org.json.JSONObject

/** Keeps a crossing until the app asks, so ones that happen in the background aren't lost. */
class GeofenceReceiver : BroadcastReceiver() {
    override fun onReceive(c: Context, i: Intent) {
        val id = i.getStringExtra("id") ?: return
        val enter = i.getBooleanExtra(LocationManager.KEY_PROXIMITY_ENTERING, false)
        GeofencingPlugin.record(c, id, enter)
    }
}

/** geofencing: the platform's proximity alerts, which need no Google services. */
@SuppressLint("MissingPermission") // allowed() comes first
internal object GeofencingPlugin {
    private const val PREFS = "nimbus.geofence"

    private fun prefs(c: Context) = c.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    private val manager get() = Plugins.context.getSystemService(Context.LOCATION_SERVICE) as LocationManager

    private fun allowed(p: String) = Plugins.context.checkSelfPermission(p) == PackageManager.PERMISSION_GRANTED

    internal fun record(c: Context, id: String, enter: Boolean) {
        val list = JSONArray(prefs(c).getString("events", "[]"))
        list.put(JSONObject().put("id", id).put("enter", enter).put("time", System.currentTimeMillis() / 1000.0))
        prefs(c).edit().putString("events", list.toString()).apply()
    }

    private fun intent(id: String): PendingIntent {
        val c = Plugins.context
        val i = Intent(c, GeofenceReceiver::class.java).setAction("nimbus.geofence.$id").putExtra("id", id)
        return PendingIntent.getBroadcast(c, id.hashCode(), i, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_MUTABLE)
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "request" -> {
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                val fine = Manifest.permission.ACCESS_FINE_LOCATION
                val back = Manifest.permission.ACCESS_BACKGROUND_LOCATION
                val second = { if (allowed(back)) call.resolve(mapOf("granted" to true)) else PermissionAsker.ask(a, back) { call.resolve(mapOf("granted" to it)) } }
                if (allowed(fine)) second() else PermissionAsker.ask(a, fine) { ok -> if (ok) second() else call.resolve(mapOf("granted" to false)) }
            }
            "add" -> {
                if (!allowed(Manifest.permission.ACCESS_FINE_LOCATION)) return call.reject(Plugins.DENIED, "the location is not allowed")
                val id = call.string("id") ?: return call.reject(Plugins.FAILED, "a region needs an id")
                manager.addProximityAlert(call.args.optDouble("latitude"), call.args.optDouble("longitude"), call.args.optDouble("radius").toFloat(), -1, intent(id))
                call.resolve()
            }
            "remove" -> {
                manager.removeProximityAlert(intent(call.string("id") ?: return call.reject(Plugins.FAILED, "which region?")))
                call.resolve()
            }
            "events" -> {
                val p = prefs(Plugins.context)
                val list = JSONArray(p.getString("events", "[]"))
                p.edit().remove("events").apply()
                call.resolve(JSONObject().put("events", list))
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    fun boot() {}
}
