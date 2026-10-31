package space.nimbusgo.nativekit

import android.content.Context
import android.media.MediaPlayer
import android.net.Uri
import android.view.Gravity
import android.view.View
import android.widget.FrameLayout
import android.widget.MediaController
import android.widget.VideoView
import org.json.JSONObject

/** video: the platform's VideoView as a native view (ui.Native). */
internal object VideoPlugin {
    fun boot() {
        NativeViews.register("video") { c, emit -> VideoHost(c, emit) }
    }

    fun handle(call: PluginCall) = Plugins.unknownMethod(call)
}

private class VideoHost(c: Context, private val emit: (String, String) -> Unit) : NimbusNativeView {
    private val box = FrameLayout(c)
    private val video = VideoView(c)
    private var url = ""
    private var player: MediaPlayer? = null
    private var paused = false
    private var loop = false
    private var muted = false
    private var controls = false

    override val view: View get() = box

    init {
        box.setBackgroundColor(0xff000000.toInt())
        box.addView(video, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT, Gravity.CENTER))
        video.setOnPreparedListener { p ->
            player = p
            apply()
            emit("ready", JSONObject().put("seconds", p.duration / 1000.0).toString())
        }
        video.setOnCompletionListener { if (!loop) emit("end", "{}") }
    }

    private fun apply() {
        player?.isLooping = loop
        player?.setVolume(if (muted) 0f else 1f, if (muted) 0f else 1f)
        if (paused) video.pause() else video.start()
    }

    override fun update(props: JSONObject) {
        paused = props.optBoolean("paused")
        loop = props.optBoolean("loop")
        muted = props.optBoolean("muted")
        val show = props.optBoolean("controls")
        if (show != controls) {
            controls = show
            video.setMediaController(if (show) MediaController(box.context).also { it.setAnchorView(video) } else null)
        }
        val u = props.optString("url")
        if (u != url && u.isNotEmpty()) {
            url = u
            player = null
            video.setVideoURI(Uri.parse(u))
        } else if (player != null) {
            apply()
        }
    }
}
