package space.nimbusgo.nativekit

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.os.Build
import android.util.Base64
import android.view.View
import org.json.JSONObject
import java.nio.ByteBuffer
import java.nio.ByteOrder

/** meshgradient: the grid Go cut, drawn as triangles that blend their corners' colors. */
internal object MeshGradientPlugin {
    fun boot() {
        NativeViews.register("meshgradient") { c, _ -> MeshGradientHost(c) }
    }

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}

private class MeshGradientView(c: Context) : View(c) {
    private val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    private var w = 0
    private var h = 0
    private var unit = FloatArray(0) // x y per vertex, in the unit square
    private var points = FloatArray(0)
    private var colors = IntArray(0)
    private var indices = ShortArray(0)

    init {
        // Vertex colors draw in hardware from Android 10; before, in software.
        if (Build.VERSION.SDK_INT < 29) setLayerType(LAYER_TYPE_SOFTWARE, null)
    }

    private fun floats(s: String): FloatArray {
        val b = ByteBuffer.wrap(Base64.decode(s, Base64.DEFAULT)).order(ByteOrder.LITTLE_ENDIAN).asFloatBuffer()
        return FloatArray(b.remaining()).also { b.get(it) }
    }

    fun set(p: JSONObject) {
        w = p.optInt("w")
        h = p.optInt("h")
        unit = floats(p.optString("positions"))
        val c = floats(p.optString("colors"))
        colors = IntArray(c.size / 4) { i ->
            Color.argb((c[i * 4 + 3] * 255).toInt(), (c[i * 4] * 255).toInt(), (c[i * 4 + 1] * 255).toInt(), (c[i * 4 + 2] * 255).toInt())
        }
        indices = ShortArray(w * h * 6)
        var k = 0
        for (y in 0 until h) for (x in 0 until w) {
            val a = (y * (w + 1) + x).toShort()
            val b = (a + 1).toShort()
            val d = (a + w + 1).toShort()
            val e = (d + 1).toShort()
            indices[k++] = a; indices[k++] = d; indices[k++] = b
            indices[k++] = b; indices[k++] = d; indices[k++] = e
        }
        points = FloatArray(unit.size)
        invalidate()
    }

    override fun onDraw(canvas: Canvas) {
        if (w == 0 || h == 0 || unit.size != (w + 1) * (h + 1) * 2 || colors.size * 2 != unit.size) return
        for (i in unit.indices step 2) {
            points[i] = unit[i] * width
            points[i + 1] = unit[i + 1] * height
        }
        canvas.drawVertices(Canvas.VertexMode.TRIANGLES, points.size, points, 0, null, 0, colors, 0, indices, 0, indices.size, paint)
    }
}

private class MeshGradientHost(c: Context) : NimbusNativeView {
    private val v = MeshGradientView(c)
    override val view: View get() = v
    override fun update(props: JSONObject) = v.set(props)
}
