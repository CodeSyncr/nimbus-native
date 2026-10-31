package space.nimbusgo.nativekit

import com.google.android.play.core.review.ReviewManagerFactory

/** appreview: Google Play's in-app review. */
internal object AppReviewPlugin {
    fun handle(call: PluginCall) {
        if (call.method != "request") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.resolve()
        val m = ReviewManagerFactory.create(a)
        m.requestReviewFlow().addOnCompleteListener { t ->
            if (t.isSuccessful) m.launchReviewFlow(a, t.result).addOnCompleteListener { call.resolve() }
            else call.resolve() // no Play Store here, or not now: nothing to show
        }
    }
}
