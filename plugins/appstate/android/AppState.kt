package space.nimbusgo.nativekit

import android.app.Activity
import android.app.Application
import android.os.Bundle

/** appstate: follows the activity's lifecycle from launch. */
internal object AppStatePlugin {
    @Volatile private var active = true

    fun boot() {
        (Plugins.context as Application).registerActivityLifecycleCallbacks(object : Application.ActivityLifecycleCallbacks {
            override fun onActivityResumed(a: Activity) { active = true }
            override fun onActivityPaused(a: Activity) { active = false }
            override fun onActivityStarted(a: Activity) {}
            override fun onActivityStopped(a: Activity) {}
            override fun onActivityCreated(a: Activity, b: Bundle?) {}
            override fun onActivitySaveInstanceState(a: Activity, b: Bundle) {}
            override fun onActivityDestroyed(a: Activity) {}
        })
    }

    fun handle(call: PluginCall) {
        if (call.method != "get") return Plugins.unknownMethod(call)
        call.resolve(mapOf("active" to active))
    }
}
