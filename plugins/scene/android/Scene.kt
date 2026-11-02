package space.nimbusgo.nativekit

import android.content.Context
import android.os.HandlerThread
import android.util.Base64
import android.view.Choreographer
import android.view.GestureDetector
import android.view.MotionEvent
import android.view.Surface
import android.view.SurfaceHolder
import android.view.SurfaceView
import android.view.View
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/** The Vulkan renderer, libnimbusscene.so (scene_vk.c). Call it on one thread only. */
internal object SceneNative {
    init { System.loadLibrary("nimbusscene") }

    @JvmStatic external fun create(surface: Surface): Long
    @JvmStatic external fun resize(h: Long)
    @JvmStatic external fun destroy(h: Long)
    @JvmStatic external fun beginMeshes(h: Long)
    @JvmStatic external fun hasMesh(h: Long, key: String): Boolean
    @JvmStatic external fun addMesh(h: Long, key: String, vertices: ByteArray, indices: ByteArray)
    @JvmStatic external fun endMeshes(h: Long)
    @JvmStatic external fun scene(h: Long, params: FloatArray)
    @JvmStatic external fun nodes(h: Long, ids: Array<String>, meshes: Array<String>, data: FloatArray, now: Double)
    @JvmStatic external fun spinning(h: Long): Boolean
    @JvmStatic external fun draw(h: Long, now: Double): Boolean
}

/** scene: a 3D scene Go describes (plugins/scene), drawn with Vulkan. */
internal object ScenePlugin {
    fun boot() {
        NativeViews.register("scene") { c, emit -> SceneHost(c, emit) }
    }

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}

private fun now() = System.nanoTime() / 1e9

private fun floats(a: JSONArray?, into: FloatArray, at: Int, n: Int) {
    for (i in 0 until n) into[at + i] = (a?.optDouble(i, 0.0) ?: 0.0).toFloat()
}

private fun vec(o: JSONObject?, into: FloatArray, at: Int) {
    into[at] = (o?.optDouble("X", 0.0) ?: 0.0).toFloat()
    into[at + 1] = (o?.optDouble("Y", 0.0) ?: 0.0).toFloat()
    into[at + 2] = (o?.optDouble("Z", 0.0) ?: 0.0).toFloat()
}

/**
 * A SurfaceView whose Vulkan work all happens on a thread of its own. With
 * something spinning it draws every frame; otherwise only when the scene changes.
 */
private class SceneHost(c: Context, private val emit: (String, String) -> Unit) :
    NimbusNativeView, SurfaceHolder.Callback, Choreographer.FrameCallback {
    private val surface = SurfaceView(c)
    override val view: View get() = surface
    private val thread = HandlerThread("nimbus-scene").apply { start() }
    private val render = android.os.Handler(thread.looper)
    private var handle = 0L // render thread only
    private var props: JSONObject? = null // the latest, for a surface made later
    private var looping = false

    init {
        surface.holder.addCallback(this)
        val taps = GestureDetector(c, object : GestureDetector.SimpleOnGestureListener() {
            override fun onDown(e: MotionEvent) = true
            override fun onSingleTapUp(e: MotionEvent): Boolean {
                emit("tap", JSONObject().put("X", e.x).put("Y", e.y).put("W", surface.width).put("H", surface.height).toString())
                return true
            }
        })
        surface.setOnTouchListener { _, e -> taps.onTouchEvent(e) }
    }

    override fun update(props: JSONObject) {
        render.post {
            this.props = props
            apply()
        }
    }

    /** Hands the scene to the renderer: meshes it lacks, the camera and light, the nodes. */
    private fun apply() {
        val h = handle
        val p = props ?: return
        if (h == 0L) return
        SceneNative.beginMeshes(h)
        val meshes = p.optJSONObject("meshes") ?: JSONObject()
        for (key in meshes.keys()) {
            if (SceneNative.hasMesh(h, key)) continue
            val m = meshes.getJSONObject(key)
            SceneNative.addMesh(h, key, Base64.decode(m.optString("vertices"), Base64.DEFAULT), Base64.decode(m.optString("indices"), Base64.DEFAULT))
        }
        SceneNative.endMeshes(h)

        val lights = p.optJSONArray("lights") ?: JSONArray()
        val n = minOf(lights.length(), 4)
        val s = FloatArray(28 + n * 6)
        val bg = p.optLong("background")
        s[0] = ((bg shr 24) and 255) / 255f; s[1] = ((bg shr 16) and 255) / 255f
        s[2] = ((bg shr 8) and 255) / 255f; s[3] = (bg and 255) / 255f
        floats(p.optJSONArray("view"), s, 4, 16)
        vec(p.optJSONObject("eye"), s, 20)
        s[23] = p.optDouble("fov", 60.0).toFloat(); s[24] = p.optDouble("near", 0.1).toFloat()
        s[25] = p.optDouble("far", 100.0).toFloat(); s[26] = p.optDouble("ambient", 0.25).toFloat()
        s[27] = n.toFloat()
        for (i in 0 until n) {
            val l = lights.getJSONObject(i)
            vec(l.optJSONObject("direction"), s, 28 + i * 6)
            floats(l.optJSONArray("color"), s, 31 + i * 6, 3)
        }
        SceneNative.scene(h, s)

        val list = p.optJSONArray("nodes") ?: JSONArray()
        val ids = Array(list.length()) { "" }
        val keys = Array(list.length()) { "" }
        val data = FloatArray(list.length() * 42)
        for (i in 0 until list.length()) {
            val o = list.getJSONObject(i)
            val at = i * 42
            ids[i] = o.optString("id")
            keys[i] = o.optString("mesh")
            floats(o.optJSONArray("base"), data, at, 16)
            floats(o.optJSONArray("local"), data, at + 16, 16)
            floats(o.optJSONArray("color"), data, at + 32, 4)
            vec(o.optJSONObject("axis"), data, at + 36)
            data[at + 39] = o.optDouble("speed", 0.0).toFloat()
            data[at + 40] = o.optDouble("shine", 0.0).toFloat()
            data[at + 41] = if (o.optBoolean("unlit")) 1f else 0f
        }
        SceneNative.nodes(h, ids, keys, data, now())
        frame()
    }

    /** Draws now, and keeps drawing each frame while something spins. */
    private fun frame() {
        if (handle == 0L) return
        SceneNative.draw(handle, now())
        if (SceneNative.spinning(handle) && !looping) {
            looping = true
            Choreographer.getInstance().postFrameCallback(this)
        }
    }

    override fun doFrame(frameTimeNanos: Long) {
        looping = false
        frame()
    }

    override fun surfaceCreated(holder: SurfaceHolder) {
        render.post {
            handle = SceneNative.create(holder.surface)
            apply()
        }
    }

    override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
        render.post {
            if (handle != 0L) {
                SceneNative.resize(handle)
                frame()
            }
        }
    }

    /** The surface goes once this returns, so the renderer must be gone first. */
    override fun surfaceDestroyed(holder: SurfaceHolder) {
        val done = CountDownLatch(1)
        render.post {
            Choreographer.getInstance().removeFrameCallback(this)
            looping = false
            if (handle != 0L) SceneNative.destroy(handle)
            handle = 0L
            done.countDown()
        }
        done.await(2, TimeUnit.SECONDS)
    }
}
