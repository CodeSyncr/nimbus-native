package space.nimbusgo.nativekit

/** badge: Android has no number of its own to set. */
internal object BadgePlugin {
    fun handle(call: PluginCall) = if (call.method == "set") call.resolve() else Plugins.unknownMethod(call)
}
