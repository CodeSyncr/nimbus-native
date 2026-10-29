package space.nimbusgo.nativekit

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Looper
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import org.json.JSONArray
import org.json.JSONObject
import org.json.JSONTokener
import java.security.KeyStore
import java.util.Locale
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** One call of a plugin method from Go. Answer it once, with [resolve] or [reject], from any thread. */
class PluginCall internal constructor(
    private val id: Long,
    val plugin: String,
    val method: String,
    json: String,
) {
    /** The arguments as Go passed them: a JSONObject, JSONArray, String, Number, Boolean or null. */
    val rawArgs: Any? = if (json.isEmpty()) null else try {
        JSONTokener(json).nextValue().takeUnless { it == JSONObject.NULL }
    } catch (e: Exception) {
        null
    }

    /** The arguments when Go passed an object (a struct or map); else empty. */
    val args: JSONObject = rawArgs as? JSONObject ?: JSONObject()

    /** A string argument, or null. */
    fun string(key: String): String? = if (args.isNull(key)) null else args.optString(key)

    private var answered = false

    /** Answers with any JSON value: null, Boolean, numbers, String, Map, List, JSONObject or JSONArray. */
    fun resolve(value: Any? = null) = finish(value = toJSON(value))

    /** Fails the call. [code] is one of [Plugins.CANCELLED], DENIED, NOT_FOUND, UNAVAILABLE or FAILED. */
    fun reject(code: String, message: String) = finish(code = code, err = message)

    @Synchronized
    private fun finish(value: String = "", code: String = "", err: String = "") {
        if (answered) return
        answered = true
        Bridge.send(Render.encode(Render.PluginResult(id = id, value = value, code = code, err = err)))
    }

    private fun toJSON(v: Any?): String = when (v) {
        null, JSONObject.NULL -> "null"
        is String -> JSONObject.quote(v)
        is Number, is Boolean -> v.toString()
        is JSONObject, is JSONArray -> v.toString()
        is Map<*, *> -> JSONObject(v).toString()
        is Collection<*> -> JSONArray(v).toString()
        else -> JSONObject.wrap(v)?.toString() ?: "null"
    }
}

/**
 * Native code Go can call (package plugin). Register app plugins at launch, before the first
 * NimbusView:
 *
 *     Plugins.register("battery") { call -> call.resolve(level) }
 *
 * Handlers run on the main thread.
 */
object Plugins {
    fun interface Handler {
        fun handle(call: PluginCall)
    }

    const val CANCELLED = "cancelled"
    const val DENIED = "denied"
    const val NOT_FOUND = "not_found"
    const val UNAVAILABLE = "unavailable"
    const val FAILED = "failed"

    private val handlers = HashMap<String, Handler>()
    private val main = android.os.Handler(Looper.getMainLooper())

    /** The application, for plugins that need a Context. Set by NimbusView. */
    @Volatile
    lateinit var context: Context
        internal set

    private var activityRef: java.lang.ref.WeakReference<android.app.Activity>? = null

    /** The activity showing the app, for plugins that show UI (dialogs); null when it is gone. */
    var activity: android.app.Activity?
        get() = activityRef?.get()?.takeUnless { it.isFinishing }
        internal set(a) {
            activityRef = a?.let { java.lang.ref.WeakReference(it) }
        }

    init {
        handlers["secureStorage"] = Handler(SecureStorage::handle)
        handlers["app"] = Handler(AppInfo::handle)
        handlers["linking"] = Handler(Linking::handle)
        handlers["dialog"] = Handler(Dialogs::handle)
    }

    /** Makes [name] callable from Go, replacing any plugin of that name. */
    @Synchronized
    fun register(name: String, handler: Handler) {
        handlers[name] = handler
    }

    /** Starts a call from Go (on a Go thread): accepted unless no plugin has the name. */
    internal fun start(c: Render.PluginCall): Render.PluginAck {
        val handler = synchronized(this) { handlers[c.plugin] }
            ?: return Render.PluginAck(err = "no plugin named ${c.plugin} in this app")
        val call = PluginCall(c.id, c.plugin, c.method, c.args)
        main.post {
            try {
                handler.handle(call)
            } catch (e: Exception) {
                call.reject(FAILED, e.toString())
            }
        }
        return Render.PluginAck()
    }

    internal fun unknownMethod(call: PluginCall) = call.reject(UNAVAILABLE, "${call.plugin} has no method ${call.method}")
}

/**
 * secureStorage: small secrets (tokens, keys), encrypted with an AES key that never leaves the
 * Android Keystore; the ciphertext lives in the app's private preferences.
 */
internal object SecureStorage {
    private const val ALIAS = "nimbus-native.secureStorage"
    private const val PREFS = "nimbus-native.secureStorage"

    fun handle(call: PluginCall) {
        val key = call.string("key")
        if (key.isNullOrEmpty()) return call.reject(Plugins.FAILED, "key is required")
        val prefs = Plugins.context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        when (call.method) {
            "get" -> {
                val stored = prefs.getString(key, null)
                    ?: return call.resolve(mapOf("value" to null, "found" to false))
                val bytes = Base64.decode(stored, Base64.NO_WRAP)
                val cipher = Cipher.getInstance("AES/GCM/NoPadding")
                cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes, 0, 12))
                val plain = cipher.doFinal(bytes, 12, bytes.size - 12)
                call.resolve(mapOf("value" to String(plain, Charsets.UTF_8), "found" to true))
            }
            "set" -> {
                val value = call.string("value") ?: return call.reject(Plugins.FAILED, "value is required")
                val cipher = Cipher.getInstance("AES/GCM/NoPadding")
                cipher.init(Cipher.ENCRYPT_MODE, key())
                val sealed = cipher.iv + cipher.doFinal(value.toByteArray(Charsets.UTF_8))
                prefs.edit().putString(key, Base64.encodeToString(sealed, Base64.NO_WRAP)).apply()
                call.resolve()
            }
            "delete" -> {
                prefs.edit().remove(key).apply()
                call.resolve()
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun key(): SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        gen.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return gen.generateKey()
    }
}

/** app: facts about the app and device, and where it may write files. */
internal object AppInfo {
    fun handle(call: PluginCall) {
        if (call.method != "info") return Plugins.unknownMethod(call)
        val c = Plugins.context
        val pkg = c.packageManager.getPackageInfo(c.packageName, 0)
        @Suppress("DEPRECATION")
        val build = if (Build.VERSION.SDK_INT >= 28) pkg.longVersionCode else pkg.versionCode.toLong()
        call.resolve(
            mapOf(
                "platform" to "android",
                "id" to c.packageName,
                "version" to (pkg.versionName ?: ""),
                "build" to build.toString(),
                "osVersion" to Build.VERSION.RELEASE,
                "locale" to Locale.getDefault().toLanguageTag(),
                "dataDir" to c.filesDir.absolutePath,
                "cacheDir" to c.cacheDir.absolutePath,
            ),
        )
    }
}

/** linking: open URLs in other apps (the browser, Maps, Mail, tel:). */
internal object Linking {
    fun handle(call: PluginCall) {
        val url = call.string("url") ?: return call.reject(Plugins.FAILED, "url is required")
        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        when (call.method) {
            "open" -> try {
                Plugins.context.startActivity(intent)
                call.resolve(true)
            } catch (e: android.content.ActivityNotFoundException) {
                call.reject(Plugins.UNAVAILABLE, "nothing opens $url")
            }
            "canOpen" -> call.resolve(intent.resolveActivity(Plugins.context.packageManager) != null)
            else -> Plugins.unknownMethod(call)
        }
    }
}

/** dialog: alerts, action sheets, toasts and date pickers, over the current activity. */
internal object Dialogs {
    fun handle(call: PluginCall) {
        if (call.method == "toast") {
            android.widget.Toast.makeText(Plugins.context, call.string("message") ?: "", android.widget.Toast.LENGTH_SHORT).show()
            return call.resolve()
        }
        val a = Plugins.activity ?: return call.reject(Plugins.UNAVAILABLE, "no activity to show a dialog in")
        when (call.method) {
            "show" -> show(call, a)
            "date" -> date(call, a)
            else -> Plugins.unknownMethod(call)
        }
    }

    private fun show(call: PluginCall, a: android.app.Activity) {
        val buttons = call.args.optJSONArray("buttons")
        val labels = (0 until (buttons?.length() ?: 0)).map { buttons!!.getJSONObject(it) }
        val b = android.app.AlertDialog.Builder(a)
        call.string("title")?.takeIf { it.isNotEmpty() }?.let(b::setTitle)
        call.string("message")?.takeIf { it.isNotEmpty() }?.let(b::setMessage)
        val cancel = labels.indexOfFirst { it.optString("style") == "cancel" }
        val others = labels.indices.filter { it != cancel }
        if (call.args.optBoolean("sheet") || others.size > 2) {
            // A list: every choice but cancel, which stays a button.
            b.setItems(others.map { labels[it].optString("label") }.toTypedArray()) { _, which -> call.resolve(others[which]) }
        } else {
            // Up to two choices: the last is the positive button, the other neutral.
            others.lastOrNull()?.let { i -> b.setPositiveButton(labels[i].optString("label")) { _, _ -> call.resolve(i) } }
            if (others.size == 2) {
                val i = others[0]
                b.setNeutralButton(labels[i].optString("label")) { _, _ -> call.resolve(i) }
            }
        }
        if (cancel >= 0) b.setNegativeButton(labels[cancel].optString("label")) { _, _ -> call.resolve(cancel) }
        b.setOnCancelListener { call.reject(Plugins.CANCELLED, "dismissed") } // Back or a tap outside
        val dialog = b.create()
        dialog.setOnShowListener {
            labels.forEachIndexed { i, l ->
                if (l.optString("style") == "destructive") {
                    val which = when (i) {
                        others.lastOrNull() -> android.content.DialogInterface.BUTTON_POSITIVE
                        cancel -> android.content.DialogInterface.BUTTON_NEGATIVE
                        else -> android.content.DialogInterface.BUTTON_NEUTRAL
                    }
                    dialog.getButton(which)?.setTextColor(0xffdc2626.toInt())
                }
            }
        }
        dialog.show()
    }

    private fun date(call: PluginCall, a: android.app.Activity) {
        val cal = java.util.Calendar.getInstance()
        if (call.args.has("initial")) cal.timeInMillis = call.args.optLong("initial")
        val mode = call.string("mode") ?: "date"
        val cancelled = { call.reject(Plugins.CANCELLED, "cancelled") }
        val pickTime = {
            android.app.TimePickerDialog(a, { _, h, m ->
                cal.set(java.util.Calendar.HOUR_OF_DAY, h)
                cal.set(java.util.Calendar.MINUTE, m)
                call.resolve(cal.timeInMillis)
            }, cal.get(java.util.Calendar.HOUR_OF_DAY), cal.get(java.util.Calendar.MINUTE),
                android.text.format.DateFormat.is24HourFormat(a)).apply { setOnCancelListener { cancelled() } }.show()
        }
        if (mode == "time") return pickTime()
        android.app.DatePickerDialog(a, { _, y, mo, d ->
            cal.set(y, mo, d)
            if (mode == "datetime") pickTime() else call.resolve(cal.timeInMillis)
        }, cal.get(java.util.Calendar.YEAR), cal.get(java.util.Calendar.MONTH), cal.get(java.util.Calendar.DAY_OF_MONTH))
            .apply { setOnCancelListener { cancelled() } }.show()
    }
}
