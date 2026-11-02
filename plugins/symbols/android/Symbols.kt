package space.nimbusgo.nativekit

/** symbols: Android draws Material Symbols from the font in Go (a ligature), so nothing runs here. */
internal object SymbolsPlugin {
    fun boot() {}

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}
