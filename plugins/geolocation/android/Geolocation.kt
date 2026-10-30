package space.nimbusgo.nativekit

import android.Manifest
import android.annotation.SuppressLint
import android.content.Context
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Build
import android.os.Looper
import org.json.JSONObject

/** geolocation: the platform's location service, while the app is open. */
@SuppressLint("MissingPermission") // allowed() comes first
internal object GeolocationPlugin {
    private var latest: Location? = null
    private val listener = LocationListener { latest = it }
    private val manager get() = Plugins.context.getSystemService(Context.LOCATION_SERVICE) as LocationManager

    private fun allowed() = Plugins.context.checkSelfPermission(Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED

    private fun provider(): String {
        val on = manager.getProviders(true)
        return listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER).firstOrNull { it in on } ?: on.firstOrNull() ?: LocationManager.GPS_PROVIDER
    }

    private fun position(l: Location): JSONObject = JSONObject().put("latitude", l.latitude).put("longitude", l.longitude)
        .put("accuracy", l.accuracy.toDouble()).put("altitude", l.altitude)
        .put("speed", if (l.hasSpeed()) l.speed.toDouble() else -1.0).put("heading", if (l.hasBearing()) l.bearing.toDouble() else -1.0)

    fun handle(call: PluginCall) {
        when (call.method) {
            "request" -> {
                if (allowed()) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.ACCESS_FINE_LOCATION) { call.resolve(mapOf("granted" to it)) }
            }
            "current" -> {
                if (!allowed()) return call.reject(Plugins.DENIED, "the location is not allowed")
                if (Build.VERSION.SDK_INT >= 30) {
                    manager.getCurrentLocation(provider(), null, Plugins.context.mainExecutor) { l ->
                        val got = l ?: manager.getLastKnownLocation(provider())
                        if (got != null) call.resolve(position(got)) else call.reject(Plugins.FAILED, "no location yet")
                    }
                } else {
                    @Suppress("DEPRECATION")
                    manager.requestSingleUpdate(provider(), { l: Location -> call.resolve(position(l)) }, Looper.getMainLooper())
                }
            }
            "start" -> {
                if (!allowed()) return call.reject(Plugins.DENIED, "the location is not allowed")
                manager.requestLocationUpdates(provider(), 1000L, 0f, listener, Looper.getMainLooper())
                call.resolve()
            }
            "stop" -> {
                manager.removeUpdates(listener)
                call.resolve()
            }
            "latest" -> {
                val l = latest ?: return call.resolve(mapOf("known" to false))
                call.resolve(position(l).put("known", true))
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
