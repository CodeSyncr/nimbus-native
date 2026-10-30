package space.nimbusgo.showcase

import android.app.Activity
import android.os.Bundle
import space.nimbusgo.nativekit.NimbusView

/** The showcase's Android host: one NimbusView. */
class MainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(NimbusView(this, NimbusSchema.COMBINED))
    }
}
