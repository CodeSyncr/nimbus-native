package space.nimbusgo.nativekit

import android.app.Activity
import android.app.Application
import android.content.Context
import android.os.Bundle
import android.view.View
import com.google.android.gms.maps.CameraUpdateFactory
import com.google.android.gms.maps.GoogleMap
import com.google.android.gms.maps.MapView
import com.google.android.gms.maps.model.LatLng
import com.google.android.gms.maps.model.MarkerOptions
import com.google.android.gms.maps.model.PolylineOptions
import org.json.JSONArray
import org.json.JSONObject

/** googlemaps: each map is a native view (ui.Native). The key is in the app's manifest. */
internal object GoogleMapsPlugin {
    fun boot() {
        NativeViews.register("googlemaps") { c, emit -> MapHost(c, emit) }
    }

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}

private fun place(o: JSONObject?): LatLng? = o?.let { LatLng(it.optDouble("lat"), it.optDouble("lng")) }

private class MapHost(c: Context, private val emit: (String, String) -> Unit) : NimbusNativeView {
    private val map = MapView(c)
    private var google: GoogleMap? = null
    private var pending: JSONObject? = null
    private var lastCamera = ""
    private var lastShapes = ""
    private val density = c.resources.displayMetrics.density

    override val view: View get() = map

    init {
        map.onCreate(null)
        map.onStart()
        map.onResume()
        (c.applicationContext as Application).registerActivityLifecycleCallbacks(object : Application.ActivityLifecycleCallbacks {
            override fun onActivityResumed(a: Activity) = map.onResume()
            override fun onActivityPaused(a: Activity) = map.onPause()
            override fun onActivityStarted(a: Activity) = map.onStart()
            override fun onActivityStopped(a: Activity) = map.onStop()
            override fun onActivityCreated(a: Activity, b: Bundle?) {}
            override fun onActivitySaveInstanceState(a: Activity, b: Bundle) {}
            override fun onActivityDestroyed(a: Activity) {}
        })
        map.getMapAsync { g ->
            google = g
            g.setOnMapClickListener { emit("tap", JSONObject().put("at", point(it)).toString()) }
            g.setOnMarkerClickListener { m ->
                emit("marker", JSONObject().put("id", m.tag as? String ?: "").toString())
                false
            }
            g.setOnCameraIdleListener {
                val p = g.cameraPosition
                emit("move", JSONObject().put("at", point(p.target)).put("zoom", p.zoom.toDouble()).toString())
            }
            pending?.let { update(it) }
        }
    }

    private fun point(l: LatLng) = JSONObject().put("lat", l.latitude).put("lng", l.longitude)

    @android.annotation.SuppressLint("MissingPermission") // Go asks for the blue dot; the system checks
    override fun update(props: JSONObject) {
        val g = google
        if (g == null) {
            pending = props
            return
        }
        val camera = "${props.opt("center")}${props.opt("zoom")}"
        if (camera != lastCamera) {
            place(props.optJSONObject("center"))?.let {
                val move = CameraUpdateFactory.newLatLngZoom(it, props.optDouble("zoom", 12.0).toFloat())
                if (lastCamera.isEmpty()) g.moveCamera(move) else g.animateCamera(move)
            }
            lastCamera = camera
        }
        try {
            g.isMyLocationEnabled = props.optBoolean("myLocation")
        } catch (e: SecurityException) {
            // no permission yet: no dot
        }
        val shapes = "${props.opt("markers")}${props.opt("lines")}"
        if (shapes == lastShapes) return
        lastShapes = shapes
        g.clear()
        val markers = props.optJSONArray("markers") ?: JSONArray()
        for (i in 0 until markers.length()) {
            val m = markers.getJSONObject(i)
            val at = place(m.optJSONObject("at")) ?: continue
            g.addMarker(MarkerOptions().position(at).title(m.optString("title").ifEmpty { null }))?.tag = m.optString("id")
        }
        val lines = props.optJSONArray("lines") ?: JSONArray()
        for (i in 0 until lines.length()) {
            val l = lines.getJSONObject(i)
            val pts = l.getJSONArray("points")
            val o = PolylineOptions().width(l.optDouble("width", 4.0).toFloat() * density).color(Renderer.argb(l.optLong("color")))
            for (k in 0 until pts.length()) place(pts.getJSONObject(k))?.let { o.add(it) }
            g.addPolyline(o)
        }
    }
}
