package space.nimbusgo.nativekit

/** livephoto: Android has no Live Photos, so Go shows View.Fallback and Pick fails. */
internal object LivePhotoPlugin {
    fun boot() {}

    fun handle(call: PluginCall) = call.reject(Plugins.UNAVAILABLE, "Live Photos are for iOS")
}
