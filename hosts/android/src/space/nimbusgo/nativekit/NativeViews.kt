package space.nimbusgo.nativekit

import android.content.Context
import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import org.json.JSONObject

/** A view a plugin makes, for ui.Native. update gets the props from Go. */
interface NimbusNativeView {
    val view: View
    fun update(props: JSONObject)
}

/** The native views Go can show. Register at launch, like [Plugins]. */
object NativeViews {
    /** emit sends a name and JSON to Go (Element.OnEvent). */
    fun interface Maker {
        fun make(c: Context, emit: (String, String) -> Unit): NimbusNativeView
    }

    private val makers = HashMap<String, Maker>()

    @Synchronized
    fun register(kind: String, make: Maker) {
        makers[kind] = make
    }

    @Synchronized
    internal fun maker(kind: String) = makers[kind]
}

internal class NNNativeHost(c: Context) : FrameLayout(c) {
    private var content: NimbusNativeView? = null

    fun set(o: Render.Native, emit: (String, String) -> Unit) {
        if (content == null) {
            content = NativeViews.maker(o.type)?.make(context, emit)?.also {
                addView(it.view, LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT))
            }
        }
        content?.update(if (o.props.isEmpty()) JSONObject() else JSONObject(o.props))
    }
}
