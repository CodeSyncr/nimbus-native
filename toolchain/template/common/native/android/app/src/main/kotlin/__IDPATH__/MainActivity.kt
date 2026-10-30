package __ID__

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import space.nimbusgo.nativekit.NimbusView

/** The Android host. The app itself is Go (app/mobile), built into
 *  libnimbusapp.so by the nimbusGo task; this activity only shows it. */
class MainActivity : Activity() {
    private lateinit var nimbus: NimbusView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        nimbus = NimbusView(this, NimbusSchema.COMBINED)
        setContentView(nimbus)
        intent?.data?.let { nimbus.renderer.open(it.toString()) } // opened by a link
    }

    /** Links to the app (__MODULE__://orders/42) open their page. */
    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        intent.data?.let { nimbus.renderer.open(it.toString()) }
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        @Suppress("DEPRECATION")
        if (!nimbus.renderer.back()) super.onBackPressed()
    }
}
