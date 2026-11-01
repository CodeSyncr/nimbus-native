package space.nimbusgo.nativekit

/** trackingconsent: Android has no such prompt. */
internal object TrackingConsentPlugin {
    fun handle(call: PluginCall) {
        when (call.method) {
            "status", "request" -> call.resolve(mapOf("status" to "not-needed"))
            else -> Plugins.unknownMethod(call)
        }
    }
}
