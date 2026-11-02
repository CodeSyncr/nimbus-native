package space.nimbusgo.nativekit

/** applemaps: Apple Maps has no Android SDK, so Go shows Map.Fallback here instead. */
internal object AppleMapsPlugin {
    fun boot() {}

    fun handle(call: PluginCall) = call.reject(Plugins.UNAVAILABLE, "Apple Maps is for iOS")
}
