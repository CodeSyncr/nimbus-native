package space.nimbusgo.nativekit

import android.Manifest
import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import org.json.JSONArray
import org.json.JSONObject

/** Takes what Firebase delivers while the app is open: Firebase itself only shows it in the background. */
class PushService : FirebaseMessagingService() {
    override fun onMessageReceived(m: RemoteMessage) {
        val title = m.notification?.title ?: m.data["title"] ?: ""
        val body = m.notification?.body ?: m.data["body"] ?: ""
        PushPlugin.keep(this, title, body, m.data, false)
        if (title.isEmpty() && body.isEmpty()) return
        val nm = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        nm.createNotificationChannel(NotificationChannel("nimbus-push", "Notifications", NotificationManager.IMPORTANCE_HIGH))
        val tap = PendingIntent.getActivity(this, 0, packageManager.getLaunchIntentForPackage(packageName), PendingIntent.FLAG_IMMUTABLE)
        nm.notify(m.messageId.hashCode(), Notification.Builder(this, "nimbus-push").setSmallIcon(applicationInfo.icon)
            .setContentTitle(title).setContentText(body).setContentIntent(tap).setAutoCancel(true).build())
    }
}

/** push: Firebase, started from the settings in google-services.json. */
internal object PushPlugin {
    private const val PREFS = "nimbus.push"
    private var seenLaunch: String? = null

    private fun ready() = PushConfig.APP_ID.isNotEmpty()

    fun boot() {
        if (!ready() || FirebaseApp.getApps(Plugins.context).isNotEmpty()) return
        FirebaseApp.initializeApp(Plugins.context, FirebaseOptions.Builder().setApplicationId(PushConfig.APP_ID)
            .setApiKey(PushConfig.API_KEY).setProjectId(PushConfig.PROJECT_ID).setGcmSenderId(PushConfig.SENDER).build())
    }

    internal fun keep(c: Context, title: String, body: String, data: Map<String, String>, tapped: Boolean) {
        val p = c.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        val list = JSONArray(p.getString("inbox", "[]"))
        list.put(JSONObject().put("title", title).put("body", body).put("data", JSONObject(data)).put("tapped", tapped))
        p.edit().putString("inbox", list.toString()).apply()
    }

    /** A notification tapped in the background opens the app with its data in the launch intent. */
    private fun launchTap() {
        val e = Plugins.activity?.intent?.extras ?: return
        val id = e.getString("google.message_id") ?: return
        if (id == seenLaunch) return
        seenLaunch = id
        val data = e.keySet().filter { !it.startsWith("google.") && it != "from" && it != "collapse_key" }.associateWith { e.get(it).toString() }
        keep(Plugins.context, "", "", data, true)
    }

    fun handle(call: PluginCall) {
        val c = Plugins.context
        when (call.method) {
            "request" -> {
                if (Build.VERSION.SDK_INT < 33 || c.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
                    return call.resolve(mapOf("granted" to true))
                }
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.POST_NOTIFICATIONS) { call.resolve(mapOf("granted" to it)) }
            }
            "token" -> {
                if (!ready()) return call.reject(Plugins.UNAVAILABLE, "put google-services.json at native/android/app and build again")
                FirebaseMessaging.getInstance().token.addOnCompleteListener {
                    if (it.isSuccessful) call.resolve(mapOf("value" to it.result, "service" to "fcm"))
                    else call.reject(Plugins.FAILED, it.exception?.toString() ?: "no token")
                }
            }
            "messages" -> {
                launchTap()
                val p = c.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
                val list = JSONArray(p.getString("inbox", "[]"))
                p.edit().remove("inbox").apply()
                call.resolve(JSONObject().put("messages", list))
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
