package space.nimbusgo.nativekit

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.HandlerThread
import android.util.Base64
import android.view.Choreographer
import android.view.GestureDetector
import android.view.MotionEvent
import android.view.ScaleGestureDetector
import android.view.Surface
import android.view.SurfaceHolder
import android.view.SurfaceView
import android.view.View
import org.json.JSONArray
import org.json.JSONObject
import java.nio.ByteBuffer
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.math.atan2
import kotlin.math.cos
import kotlin.math.sin
import kotlin.math.sqrt

/** The Vulkan renderer, libnimbusscene.so (scene_vk.c). Call it on one thread only. */
internal object SceneNative {
    init { System.loadLibrary("nimbusscene") }

    @JvmStatic external fun create(surface: Surface): Long
    @JvmStatic external fun resize(h: Long)
    @JvmStatic external fun destroy(h: Long)
    @JvmStatic external fun beginAssets(h: Long)
    @JvmStatic external fun hasMesh(h: Long, key: String): Boolean
    @JvmStatic external fun hasTexture(h: Long, key: String): Boolean
    @JvmStatic external fun addMesh(h: Long, key: String, vertices: ByteArray, indices: ByteArray)
    @JvmStatic external fun addTexture(h: Long, key: String, rgba: ByteArray, w: Int, height: Int)
    @JvmStatic external fun endAssets(h: Long)
    @JvmStatic external fun scene(h: Long, params: FloatArray)
    @JvmStatic external fun camera(h: Long, eyeTarget: FloatArray)
    @JvmStatic external fun nodes(h: Long, ids: Array<String>, meshes: Array<String>, textures: Array<String>, data: FloatArray, now: Double)
    @JvmStatic external fun spinning(h: Long): Boolean
    @JvmStatic external fun draw(h: Long, now: Double): Boolean
}

/**
 * The meshes and pictures Go has sent, by key, for every scene on screen. Each scene's renderer
 * copies what it uses onto its GPU; these stay so a scene made later needn't ask again.
 */
internal object SceneAssets {
    class Mesh(val vertices: ByteArray, val indices: ByteArray)

    private val meshes = HashMap<String, Mesh>()
    private val textures = HashMap<String, ByteArray>() // PNG or JPEG
    private val waiting = HashSet<() -> Unit>()

    @Synchronized fun mesh(key: String) = meshes[key]
    @Synchronized fun texture(key: String) = textures[key]
    @Synchronized fun listen(f: () -> Unit) { waiting.add(f) }
    @Synchronized fun unlisten(f: () -> Unit) { waiting.remove(f) }

    fun arrived(put: () -> Unit) {
        val tell = synchronized(this) { put(); waiting.toList() }
        tell.forEach { it() }
    }

    fun putMesh(key: String, m: Mesh) = arrived { meshes[key] = m }
    fun putTexture(key: String, data: ByteArray) = arrived { textures[key] = data }

    /** RGBA rows from the top, unpremultiplied, as the renderer takes them. */
    fun pixels(data: ByteArray): Triple<ByteArray, Int, Int>? {
        val o = BitmapFactory.Options().apply { inPremultiplied = false; inPreferredConfig = Bitmap.Config.ARGB_8888 }
        val b = BitmapFactory.decodeByteArray(data, 0, data.size, o) ?: return null
        val px = ByteBuffer.allocate(b.byteCount)
        b.copyPixelsToBuffer(px) // ARGB_8888 is R, G, B, A in memory
        val out = Triple(px.array(), b.width, b.height)
        b.recycle()
        return out
    }
}

/** scene: a 3D scene Go describes (plugins/scene), drawn with Vulkan. */
internal object ScenePlugin {
    fun boot() {
        NativeViews.register("scene") { c, emit -> SceneHost(c, emit) }
    }

    fun handle(call: PluginCall) {
        val key = call.string("key") ?: return call.reject(Plugins.FAILED, "no key")
        when (call.method) {
            "mesh" -> SceneAssets.putMesh(key, SceneAssets.Mesh(
                Base64.decode(call.string("vertices") ?: "", Base64.DEFAULT), Base64.decode(call.string("indices") ?: "", Base64.DEFAULT)))
            "texture" -> SceneAssets.putTexture(key, Base64.decode(call.string("data") ?: "", Base64.DEFAULT))
            else -> return Plugins.unknownMethod(call)
        }
        call.resolve()
    }
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
 * something spinning it draws every frame; otherwise only when something changes.
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
    private var asked = HashSet<String>() // keys asked for since the last props

    // The camera: Go's, and where Orbit has taken it. Main thread.
    private var goCamera = ""
    private val eye = FloatArray(3)
    private val target = FloatArray(3)
    private var orbit = false
    private var yaw = 0f
    private var pitch = 0f
    private var dist = 5f
    private var near = 0.1f
    private var far = 100f

    private val assetArrived: () -> Unit = { render.post { apply() } }

    init {
        surface.holder.addCallback(this)
        val taps = GestureDetector(c, object : GestureDetector.SimpleOnGestureListener() {
            override fun onDown(e: MotionEvent) = true
            override fun onSingleTapUp(e: MotionEvent): Boolean {
                val o = JSONObject().put("X", e.x).put("Y", e.y).put("W", surface.width).put("H", surface.height)
                    .put("Eye", JSONObject().put("X", eye[0]).put("Y", eye[1]).put("Z", eye[2]))
                    .put("Target", JSONObject().put("X", target[0]).put("Y", target[1]).put("Z", target[2]))
                emit("tap", o.toString())
                return true
            }

            override fun onScroll(e1: MotionEvent?, e2: MotionEvent, dx: Float, dy: Float): Boolean {
                if (!orbit || e2.pointerCount > 1) return false
                val d = surface.resources.displayMetrics.density
                yaw += dx / d * 0.4f
                pitch = (pitch - dy / d * 0.4f).coerceIn(-85f, 85f)
                moved()
                return true
            }
        })
        val pinch = ScaleGestureDetector(c, object : ScaleGestureDetector.SimpleOnScaleGestureListener() {
            override fun onScale(d: ScaleGestureDetector): Boolean {
                if (!orbit) return false
                dist = (dist / d.scaleFactor).coerceIn(near * 4, far / 2)
                moved()
                return true
            }
        })
        surface.setOnTouchListener { _, e ->
            pinch.onTouchEvent(e)
            taps.onTouchEvent(e)
        }
    }

    /** Where yaw, pitch and distance put the eye, around the target. */
    private fun moved() {
        val y = Math.toRadians(yaw.toDouble())
        val p = Math.toRadians(pitch.toDouble())
        eye[0] = target[0] + (dist * cos(p) * sin(y)).toFloat()
        eye[1] = target[1] + (dist * sin(p)).toFloat()
        eye[2] = target[2] + (dist * cos(p) * cos(y)).toFloat()
        val et = eye + target
        render.post {
            if (handle == 0L) return@post
            SceneNative.camera(handle, et)
            frame()
        }
    }

    /** Takes Go's camera when it changes; keeps Orbit's otherwise. */
    private fun camera(p: JSONObject) {
        val e = p.optJSONObject("eye")
        val t = p.optJSONObject("target")
        orbit = p.optBoolean("orbit")
        near = p.optDouble("near", 0.1).toFloat()
        far = p.optDouble("far", 100.0).toFloat()
        val key = "$e$t"
        if (key == goCamera) return
        goCamera = key
        vec(e, eye, 0)
        vec(t, target, 0)
        val dx = eye[0] - target[0]
        val dy = eye[1] - target[1]
        val dz = eye[2] - target[2]
        dist = sqrt(dx * dx + dy * dy + dz * dz).coerceAtLeast(1e-3f)
        yaw = Math.toDegrees(atan2(dx, dz).toDouble()).toFloat()
        pitch = Math.toDegrees(kotlin.math.asin((dy / dist).coerceIn(-1f, 1f)).toDouble()).toFloat()
    }

    override fun update(props: JSONObject) {
        camera(props)
        val et = eye + target
        render.post {
            this.props = props
            asked.clear()
            apply()
            if (handle != 0L) {
                SceneNative.camera(handle, et)
                frame()
            }
        }
    }

    /** Copies onto the GPU what the scene uses and this renderer lacks, and asks Go for what nobody has. */
    private fun assets(h: Long, nodes: JSONArray): List<String> {
        val missing = ArrayList<String>()
        SceneNative.beginAssets(h)
        for (i in 0 until nodes.length()) {
            val o = nodes.getJSONObject(i)
            val m = o.optString("mesh")
            if (m.isNotEmpty() && !SceneNative.hasMesh(h, m)) {
                val got = SceneAssets.mesh(m)
                if (got != null) SceneNative.addMesh(h, m, got.vertices, got.indices) else missing.add(m)
            }
            val t = o.optString("texture")
            if (t.isNotEmpty() && !SceneNative.hasTexture(h, t)) {
                val got = SceneAssets.texture(t)?.let { SceneAssets.pixels(it) }
                if (got != null) SceneNative.addTexture(h, t, got.first, got.second, got.third) else missing.add(t)
            }
        }
        SceneNative.endAssets(h)
        return missing
    }

    /** Hands the scene to the renderer: what it lacks, the camera and light, the nodes. */
    private fun apply() {
        val h = handle
        val p = props ?: return
        if (h == 0L) return
        val list = p.optJSONArray("nodes") ?: JSONArray()
        val missing = assets(h, list).filter { asked.add(it) }.distinct()
        if (missing.isNotEmpty()) emit("need", JSONObject().put("keys", JSONArray(missing)).toString())

        val lights = p.optJSONArray("lights") ?: JSONArray()
        val n = minOf(lights.length(), 4)
        val s = FloatArray(32 + n * 6)
        val bg = p.optLong("background")
        s[0] = ((bg shr 24) and 255) / 255f; s[1] = ((bg shr 16) and 255) / 255f
        s[2] = ((bg shr 8) and 255) / 255f; s[3] = (bg and 255) / 255f
        vec(p.optJSONObject("eye"), s, 4)
        vec(p.optJSONObject("target"), s, 7)
        s[10] = p.optDouble("fov", 60.0).toFloat(); s[11] = p.optDouble("near", 0.1).toFloat()
        s[12] = p.optDouble("far", 100.0).toFloat(); s[13] = p.optDouble("ambient", 0.25).toFloat()
        val shadow = p.optJSONObject("shadow")
        s[14] = if (shadow != null) 1f else 0f
        floats(shadow?.optJSONArray("vp"), s, 15, 16)
        s[31] = n.toFloat()
        for (i in 0 until n) {
            val l = lights.getJSONObject(i)
            vec(l.optJSONObject("direction"), s, 32 + i * 6)
            floats(l.optJSONArray("color"), s, 35 + i * 6, 3)
        }
        SceneNative.scene(h, s)

        val ids = Array(list.length()) { "" }
        val meshes = Array(list.length()) { "" }
        val textures = Array(list.length()) { "" }
        val data = FloatArray(list.length() * 46)
        fun f(b: Boolean) = if (b) 1f else 0f
        for (i in 0 until list.length()) {
            val o = list.getJSONObject(i)
            val at = i * 46
            ids[i] = o.optString("id")
            meshes[i] = o.optString("mesh")
            textures[i] = o.optString("texture")
            floats(o.optJSONArray("base"), data, at, 16)
            floats(o.optJSONArray("local"), data, at + 16, 16)
            floats(o.optJSONArray("color"), data, at + 32, 4)
            vec(o.optJSONObject("axis"), data, at + 36)
            data[at + 39] = o.optDouble("speed", 0.0).toFloat()
            data[at + 40] = o.optDouble("shine", 0.0).toFloat()
            data[at + 41] = f(o.optBoolean("unlit"))
            data[at + 42] = f(o.optBoolean("double"))
            data[at + 43] = f(o.optBoolean("cast"))
            data[at + 44] = o.optInt("parent", 0).toFloat()
        }
        SceneNative.nodes(h, ids, meshes, textures, data, now())
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
        SceneAssets.listen(assetArrived)
        val et = eye + target
        render.post {
            handle = SceneNative.create(holder.surface)
            apply()
            if (handle != 0L) {
                SceneNative.camera(handle, et)
                frame()
            }
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
        SceneAssets.unlisten(assetArrived)
        val done = CountDownLatch(1)
        render.post {
            Choreographer.getInstance().removeFrameCallback(this)
            looping = false
            if (handle != 0L) SceneNative.destroy(handle)
            handle = 0L
            asked.clear()
            done.countDown()
        }
        done.await(2, TimeUnit.SECONDS)
    }
}
