package space.nimbusgo.nativekit

import android.content.ComponentName
import android.content.Intent
import android.net.Uri

/** intentlauncher: an intent, and what came back. */
internal object IntentLauncherPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "start") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        val i = Intent(call.string("action"))
        val data = call.string("data")?.let { Uri.parse(it) }
        val type = call.string("type")
        when {
            data != null && type != null -> i.setDataAndType(data, type)
            data != null -> i.data = data
            type != null -> i.type = type
        }
        val pkg = call.string("package")
        val cls = call.string("class")
        if (pkg != null && cls != null) i.component = ComponentName(pkg, cls) else if (pkg != null) i.setPackage(pkg)
        call.args.optJSONObject("extras")?.let { e -> e.keys().forEach { i.putExtra(it, e.getString(it)) } }
        ResultAsker.start(a, i, { call.reject(Plugins.NOT_FOUND, "nothing on this phone handles that") }) { code, back ->
            call.resolve(mapOf("code" to code, "data" to (back?.dataString ?: "")))
        }
    }
}
