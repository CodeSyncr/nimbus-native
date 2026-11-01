package space.nimbusgo.nativekit

import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri

/** sms: whichever messages app they use. It never says what happened. */
internal object SmsPlugin {
    fun handle(call: PluginCall) {
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to Plugins.context.packageManager.hasSystemFeature(PackageManager.FEATURE_TELEPHONY_MESSAGING)))
            "compose" -> {
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
                val to = call.args.optJSONArray("to")
                val numbers = List(to?.length() ?: 0) { to!!.getString(it) }.joinToString(";")
                val i = Intent(Intent.ACTION_SENDTO, Uri.parse("smsto:" + Uri.encode(numbers))).putExtra("sms_body", call.string("body") ?: "")
                ResultAsker.start(a, i, { call.reject(Plugins.UNAVAILABLE, "this phone has no messages app") }) { _, _ ->
                    call.resolve(mapOf("status" to "unknown"))
                }
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
