package space.nimbusgo.nativekit

import android.content.Context
import android.view.inputmethod.InputMethodManager

/** keyboard: put it away. */
internal object KeyboardPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "dismiss") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.resolve()
        val imm = a.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager
        imm.hideSoftInputFromWindow(a.window.decorView.windowToken, 0)
        a.currentFocus?.clearFocus()
        call.resolve()
    }
}
