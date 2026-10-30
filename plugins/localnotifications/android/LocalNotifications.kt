package space.nimbusgo.nativekit

import android.Manifest
import android.app.AlarmManager
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.SystemClock

/** Shows a notification when its time comes, even if the app has gone. */
class NotificationReceiver : BroadcastReceiver() {
    override fun onReceive(c: Context, i: Intent) {
        val nm = c.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        nm.createNotificationChannel(NotificationChannel("nimbus", "Notifications", NotificationManager.IMPORTANCE_DEFAULT))
        val open = c.packageManager.getLaunchIntentForPackage(c.packageName)
        val tap = PendingIntent.getActivity(c, 0, open, PendingIntent.FLAG_IMMUTABLE)
        val n = Notification.Builder(c, "nimbus").setSmallIcon(c.applicationInfo.icon).setContentTitle(i.getStringExtra("title"))
            .setContentText(i.getStringExtra("body")).setContentIntent(tap).setAutoCancel(true).build()
        nm.notify(i.getStringExtra("id").hashCode(), n)
    }
}

/** localnotifications: an alarm that fires the receiver. Inexact, so it needs no special permission. */
internal object LocalNotificationsPlugin {
    private fun pending(id: String, title: String?, body: String?): PendingIntent {
        val c = Plugins.context
        val i = Intent(c, NotificationReceiver::class.java).setAction("nimbus.notification.$id")
            .putExtra("id", id).putExtra("title", title).putExtra("body", body)
        return PendingIntent.getBroadcast(c, id.hashCode(), i, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    }

    fun handle(call: PluginCall) {
        val c = Plugins.context
        val alarms = c.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        when (call.method) {
            "request" -> {
                if (Build.VERSION.SDK_INT < 33 || c.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
                    return call.resolve(mapOf("granted" to true))
                }
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.POST_NOTIFICATIONS) { call.resolve(mapOf("granted" to it)) }
            }
            "schedule" -> {
                val id = call.string("id") ?: return call.reject(Plugins.FAILED, "a notification needs an id")
                val at = SystemClock.elapsedRealtime() + (call.args.optDouble("seconds", 1.0) * 1000).toLong()
                alarms.setAndAllowWhileIdle(AlarmManager.ELAPSED_REALTIME_WAKEUP, at, pending(id, call.string("title"), call.string("body")))
                call.resolve()
            }
            "cancel" -> {
                alarms.cancel(pending(call.string("id") ?: "", null, null))
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
