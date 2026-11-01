/// navigationbar: iOS has no such bar, so shared code gets a quiet yes.
enum NavigationBarPlugin {
    static func handle(_ call: PluginCall) { call.resolve() }
}
