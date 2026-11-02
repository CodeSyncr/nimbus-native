package space.nimbusgo.nativekit

import android.content.Context
import android.view.View
import androidx.core.util.Consumer
import androidx.window.java.layout.WindowInfoTrackerCallbackAdapter
import androidx.window.layout.FoldingFeature
import androidx.window.layout.WindowInfoTracker
import androidx.window.layout.WindowLayoutInfo
import org.json.JSONObject

/** hinge: a probe view that reports the window's fold to Go (Jetpack WindowManager) while it's on screen. */
internal object HingePlugin {
    fun boot() {
        NativeViews.register("hinge") { c, emit -> HingeProbe(c, emit) }
    }

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}

private class HingeProbe(c: Context, private val emit: (String, String) -> Unit) : NimbusNativeView {
    private var tracker: WindowInfoTrackerCallbackAdapter? = null
    private var last = ""

    private val listener = Consumer<WindowLayoutInfo> { info ->
        val d = view.resources.displayMetrics.density
        val f = info.displayFeatures.filterIsInstance<FoldingFeature>().firstOrNull()
        val o = JSONObject().put("present", f != null)
        if (f != null) {
            val b = f.bounds
            o.put("halfOpen", f.state == FoldingFeature.State.HALF_OPENED)
                .put("vertical", f.orientation == FoldingFeature.Orientation.VERTICAL)
                .put("separates", f.isSeparating)
                .put("occludes", f.occlusionType == FoldingFeature.OcclusionType.FULL)
                .put("x", b.left / d).put("y", b.top / d).put("w", b.width() / d).put("h", b.height() / d)
        }
        val s = o.toString()
        if (s != last) {
            last = s
            emit("fold", s)
        }
    }

    override val view: View = object : View(c) {
        override fun onAttachedToWindow() {
            super.onAttachedToWindow()
            val a = Plugins.activity ?: return
            tracker = WindowInfoTrackerCallbackAdapter(WindowInfoTracker.getOrCreate(a)).also {
                it.addWindowLayoutInfoListener(a, a.mainExecutor, listener)
            }
        }

        override fun onDetachedFromWindow() {
            tracker?.removeWindowLayoutInfoListener(listener)
            tracker = null
            super.onDetachedFromWindow()
        }
    }

    override fun update(props: JSONObject) {}
}
