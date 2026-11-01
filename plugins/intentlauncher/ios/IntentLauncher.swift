/// intentlauncher: Android's alone.
enum IntentLauncherPlugin {
    static func handle(_ call: PluginCall) {
        call.reject(Plugins.unavailable, "iOS has no intents")
    }
}
