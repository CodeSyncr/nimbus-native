package space.nimbusgo.nativekit

import android.Manifest
import android.annotation.SuppressLint
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Build
import android.os.IBinder
import android.os.Looper
import org.json.JSONObject

/** Keeps the process following the location, with a notification that says so. */
class BackgroundLocationService : Service(), LocationListener {
    private val manager get() = getSystemService(Context.LOCATION_SERVICE) as LocationManager

    override fun onBind(i: Intent?): IBinder? = null

    @SuppressLint("MissingPermission") // the plugin asks first
    override fun onStartCommand(i: Intent?, flags: Int, id: Int): Int {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(NotificationChannel(CHANNEL, "Location", NotificationManager.IMPORTANCE_LOW))
        val open = packageManager.getLaunchIntentForPackage(packageName)
        val note = Notification.Builder(this, CHANNEL).setOngoing(true).setSmallIcon(android.R.drawable.ic_menu_mylocation)
            .setContentTitle(i?.getStringExtra("title") ?: "Following your location")
            .setContentText(i?.getStringExtra("text") ?: "")
            .setContentIntent(open?.let { PendingIntent.getActivity(this, 0, it, PendingIntent.FLAG_IMMUTABLE) })
            .build()
        if (Build.VERSION.SDK_INT >= 29) startForeground(NOTE, note, ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION) else startForeground(NOTE, note)
        val on = manager.getProviders(true)
        val provider = listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER).firstOrNull { it in on } ?: on.firstOrNull()
        if (provider != null) {
            manager.requestLocationUpdates(provider, 1000L, (i?.getDoubleExtra("minDistance", 10.0) ?: 10.0).toFloat(), this, Looper.getMainLooper())
        }
        BackgroundLocationPlugin.running = true
        return START_NOT_STICKY // it needs the app's say-so to begin again
    }

    override fun onLocationChanged(l: Location) = BackgroundLocationPlugin.add(l)

    override fun onDestroy() {
        manager.removeUpdates(this)
        BackgroundLocationPlugin.running = false
        super.onDestroy()
    }

    private companion object {
        const val CHANNEL = "nimbus-location"
        const val NOTE = 7301
    }
}

/** backgroundlocation: starts the service, and holds what it finds. */
internal object BackgroundLocationPlugin {
    private const val LIMIT = 5000
    private val points = ArrayDeque<JSONObject>()
    @Volatile var running = false

    private fun allowed() = Plugins.context.checkSelfPermission(Manifest.permission.ACCESS_FINE_LOCATION) == PackageManager.PERMISSION_GRANTED

    @Synchronized fun add(l: Location) {
        points.addLast(JSONObject().put("latitude", l.latitude).put("longitude", l.longitude).put("accuracy", l.accuracy.toDouble())
            .put("speed", if (l.hasSpeed()) l.speed.toDouble() else -1.0).put("time", l.time / 1000.0))
        while (points.size > LIMIT) points.removeFirst()
    }

    @Synchronized private fun drain(): List<JSONObject> = points.toList().also { points.clear() }

    private fun request(call: PluginCall) {
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
        val c = Plugins.context
        if (!allowed()) {
            return PermissionAsker.ask(a, Manifest.permission.ACCESS_FINE_LOCATION) { ok ->
                if (ok) request(call) else call.resolve(mapOf("granted" to false))
            }
        }
        // Following works without its notification, so a no here isn't a no to following.
        if (Build.VERSION.SDK_INT >= 33 && c.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            return PermissionAsker.ask(a, Manifest.permission.POST_NOTIFICATIONS) { call.resolve(mapOf("granted" to true)) }
        }
        call.resolve(mapOf("granted" to true))
    }

    fun handle(call: PluginCall) {
        val c = Plugins.context
        when (call.method) {
            "request" -> request(call)
            "start" -> {
                if (!allowed()) return call.reject(Plugins.DENIED, "the location is not allowed")
                val i = Intent(c, BackgroundLocationService::class.java).putExtra("title", call.string("title")).putExtra("text", call.string("text"))
                    .putExtra("minDistance", call.args.optDouble("minDistance", 10.0))
                try {
                    c.startForegroundService(i)
                    call.resolve()
                } catch (e: IllegalStateException) { // a phone won't start this from the background
                    call.reject(Plugins.FAILED, "start it from a screen on show")
                }
            }
            "stop" -> {
                c.stopService(Intent(c, BackgroundLocationService::class.java))
                call.resolve()
            }
            "running" -> call.resolve(mapOf("running" to running))
            "points" -> call.resolve(drain())
            else -> Plugins.unknownMethod(call)
        }
    }
}
