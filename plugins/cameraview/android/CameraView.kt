package space.nimbusgo.nativekit

import android.Manifest
import android.annotation.SuppressLint
import android.app.Activity
import android.content.Context
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.ImageFormat
import android.graphics.Matrix
import android.graphics.RectF
import android.graphics.SurfaceTexture
import android.hardware.camera2.CameraCaptureSession
import android.hardware.camera2.CameraCharacteristics
import android.hardware.camera2.CameraDevice
import android.hardware.camera2.CameraManager
import android.hardware.camera2.CaptureRequest
import android.media.ImageReader
import android.os.Handler
import android.os.HandlerThread
import android.os.Looper
import android.util.Base64
import android.util.Size
import android.view.Surface
import android.view.TextureView
import android.view.View
import android.view.WindowManager
import android.widget.FrameLayout
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import kotlin.math.max

/** cameraview: Camera2 behind a native view (ui.Native). */
internal object CameraViewPlugin {
    internal var current: CameraHost? = null

    fun boot() {
        NativeViews.register("cameraview") { c, emit -> CameraHost(c, emit) }
    }

    fun handle(call: PluginCall) {
        when (call.method) {
            "request" -> {
                val c = Plugins.context
                if (c.checkSelfPermission(Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.CAMERA) { ok ->
                    call.resolve(mapOf("granted" to ok))
                    if (ok) current?.start()
                }
            }
            "take" -> current?.takeIfRunning()?.take(call) ?: call.reject(Plugins.UNAVAILABLE, "no camera is showing")
            else -> Plugins.unknownMethod(call)
        }
    }
}

internal class CameraHost(private val c: Context, private val emit: (String, String) -> Unit) : NimbusNativeView, TextureView.SurfaceTextureListener {
    private val texture = TextureView(c)
    private val box = object : FrameLayout(c) {
        override fun onWindowVisibilityChanged(v: Int) {
            super.onWindowVisibilityChanged(v)
            if (v == View.VISIBLE) start() else stop()
        }
    }
    private val main = Handler(Looper.getMainLooper())
    private val manager get() = c.getSystemService(Context.CAMERA_SERVICE) as CameraManager
    private var thread: HandlerThread? = null
    private var worker: Handler? = null
    private var device: CameraDevice? = null
    private var session: CameraCaptureSession? = null
    private var reader: ImageReader? = null
    private var surface: Surface? = null
    private var size = Size(1280, 720)
    private var sensor = 90
    private var front = false
    private var torch = false
    private var turn = 0 // a stale camera's callbacks see a different number
    private var running = false

    override val view: View get() = box

    init {
        box.setBackgroundColor(0xff000000.toInt())
        box.addView(texture, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
        texture.surfaceTextureListener = this
        CameraViewPlugin.current = this
    }

    fun takeIfRunning() = takeIf { running }

    override fun update(props: JSONObject) {
        CameraViewPlugin.current = this
        val f = props.optBoolean("front")
        val t = props.optBoolean("torch")
        if (f != front) {
            front = f
            torch = t
            stop()
            start()
        } else if (t != torch) {
            torch = t
            repeat()
        }
    }

    override fun onSurfaceTextureAvailable(s: SurfaceTexture, w: Int, h: Int) = start()
    override fun onSurfaceTextureSizeChanged(s: SurfaceTexture, w: Int, h: Int) = fit()
    override fun onSurfaceTextureDestroyed(s: SurfaceTexture): Boolean { stop(); return true }
    override fun onSurfaceTextureUpdated(s: SurfaceTexture) {}

    private fun fail(code: String) = main.post { emit("error", JSONObject().put("code", code).toString()) }

    @SuppressLint("MissingPermission")
    fun start() {
        if (device != null || !texture.isAvailable) return
        if (c.checkSelfPermission(Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED) { fail(Plugins.DENIED); return }
        val facing = if (front) CameraCharacteristics.LENS_FACING_FRONT else CameraCharacteristics.LENS_FACING_BACK
        val id = manager.cameraIdList.firstOrNull { manager.getCameraCharacteristics(it).get(CameraCharacteristics.LENS_FACING) == facing }
        if (id == null) { fail(Plugins.UNAVAILABLE); return }
        val info = manager.getCameraCharacteristics(id)
        val map = info.get(CameraCharacteristics.SCALER_STREAM_CONFIGURATION_MAP)!!
        sensor = info.get(CameraCharacteristics.SENSOR_ORIENTATION) ?: 90
        val area = { s: Size -> s.width.toLong() * s.height }
        size = map.getOutputSizes(SurfaceTexture::class.java).filter { it.width <= 1920 && it.height <= 1080 }.maxByOrNull(area) ?: Size(1280, 720)
        val still = map.getOutputSizes(ImageFormat.JPEG).filter { max(it.width, it.height) <= 4032 }.maxByOrNull(area) ?: size
        val t = HandlerThread("nimbus-camera").apply { start() }
        thread = t
        val w = Handler(t.looper)
        worker = w
        val mine = ++turn
        reader = ImageReader.newInstance(still.width, still.height, ImageFormat.JPEG, 2)
        manager.openCamera(id, object : CameraDevice.StateCallback() {
            override fun onOpened(d: CameraDevice) {
                if (mine != turn) return d.close()
                device = d
                main.post { fit() }
                texture.surfaceTexture?.setDefaultBufferSize(size.width, size.height)
                val s = Surface(texture.surfaceTexture)
                surface = s
                @Suppress("DEPRECATION")
                d.createCaptureSession(listOf(s, reader!!.surface), object : CameraCaptureSession.StateCallback() {
                    override fun onConfigured(cs: CameraCaptureSession) {
                        if (mine != turn) return
                        session = cs
                        running = true
                        repeat()
                        main.post { emit("ready", "{}") }
                    }

                    override fun onConfigureFailed(cs: CameraCaptureSession) = fail(Plugins.FAILED).let {}
                }, w)
            }

            override fun onDisconnected(d: CameraDevice) { d.close(); if (device === d) device = null }
            override fun onError(d: CameraDevice, e: Int) {
                d.close()
                if (device === d) device = null
                fail(Plugins.UNAVAILABLE)
            }
        }, w)
    }

    fun stop() {
        turn++
        running = false
        session?.close(); session = null
        device?.close(); device = null
        reader?.close(); reader = null
        surface?.release(); surface = null
        thread?.quitSafely(); thread = null; worker = null
    }

    private fun applyLight(b: CaptureRequest.Builder) {
        b.set(CaptureRequest.CONTROL_AF_MODE, CaptureRequest.CONTROL_AF_MODE_CONTINUOUS_PICTURE)
        b.set(CaptureRequest.CONTROL_AE_MODE, CaptureRequest.CONTROL_AE_MODE_ON)
        b.set(CaptureRequest.FLASH_MODE, if (torch) CaptureRequest.FLASH_MODE_TORCH else CaptureRequest.FLASH_MODE_OFF)
    }

    private fun repeat() {
        val d = device ?: return
        val cs = session ?: return
        val s = surface ?: return
        val b = d.createCaptureRequest(CameraDevice.TEMPLATE_PREVIEW)
        b.addTarget(s)
        applyLight(b)
        cs.setRepeatingRequest(b.build(), null, worker)
    }

    private fun rotation(): Int = (c.getSystemService(Context.WINDOW_SERVICE) as WindowManager).defaultDisplay.rotation

    /** Fills the view, cropping what's over, the way a phone's camera does. */
    private fun fit() {
        val w = texture.width.toFloat()
        val h = texture.height.toFloat()
        if (w == 0f || h == 0f) return
        val r = rotation()
        val m = Matrix()
        val cx = w / 2
        val cy = h / 2
        if (r == Surface.ROTATION_90 || r == Surface.ROTATION_270) {
            val view = RectF(0f, 0f, w, h)
            val buffer = RectF(0f, 0f, size.height.toFloat(), size.width.toFloat())
            buffer.offset(cx - buffer.centerX(), cy - buffer.centerY())
            m.setRectToRect(view, buffer, Matrix.ScaleToFit.FILL)
            val s = max(h / size.height, w / size.width)
            m.postScale(s, s, cx, cy)
            m.postRotate(90f * (r - 2), cx, cy)
        } else {
            val content = size.height.toFloat() / size.width
            val shown = w / h
            if (content > shown) m.postScale(content / shown, 1f, cx, cy) else m.postScale(1f, shown / content, cx, cy)
            if (r == Surface.ROTATION_180) m.postRotate(180f, cx, cy)
        }
        texture.setTransform(m)
    }

    fun take(call: PluginCall) {
        val d = device ?: return call.reject(Plugins.UNAVAILABLE, "no camera is showing")
        val cs = session ?: return call.reject(Plugins.UNAVAILABLE, "no camera is showing")
        val r = reader ?: return call.reject(Plugins.UNAVAILABLE, "no camera is showing")
        val w = worker
        val degrees = intArrayOf(0, 90, 180, 270)[rotation()]
        val turnBy = if (front) (sensor + degrees) % 360 else (sensor - degrees + 360) % 360
        r.setOnImageAvailableListener({ ir ->
            r.setOnImageAvailableListener(null, null)
            val image = ir.acquireLatestImage() ?: return@setOnImageAvailableListener call.reject(Plugins.FAILED, "no photo came back")
            val buffer = image.planes[0].buffer
            val bytes = ByteArray(buffer.remaining()).also { buffer.get(it) }
            image.close()
            deliver(call, bytes, turnBy)
        }, w)
        val b = d.createCaptureRequest(CameraDevice.TEMPLATE_STILL_CAPTURE)
        b.addTarget(r.surface)
        applyLight(b)
        cs.capture(b.build(), null, w)
    }

    private fun deliver(call: PluginCall, bytes: ByteArray, degrees: Int) {
        var image = BitmapFactory.decodeByteArray(bytes, 0, bytes.size) ?: return call.reject(Plugins.FAILED, "no photo came back")
        val m = Matrix()
        val longest = max(image.width, image.height)
        val limit = call.args.optInt("maxSize", 0)
        if (limit in 1 until longest) (limit.toFloat() / longest).let { m.postScale(it, it) }
        m.postRotate(degrees.toFloat())
        if (front) m.postScale(-1f, 1f)
        image = Bitmap.createBitmap(image, 0, 0, image.width, image.height, m, true)
        val q = call.args.optDouble("quality", 0.0).let { if (it > 0) it else 0.85 }
        val out = ByteArrayOutputStream()
        image.compress(Bitmap.CompressFormat.JPEG, (q * 100).toInt().coerceIn(1, 100), out)
        call.resolve(mapOf("data" to Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP), "width" to image.width, "height" to image.height))
    }
}
