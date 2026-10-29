package space.nimbusgo.orders

import android.app.Activity
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.widget.EditText
import android.widget.TextView
import space.nimbusgo.nativekit.NimbusView
import space.nimbusgo.nativekit.Renderer

/** The orders app's Android host, plus a scripted check against the running server. */
class MainActivity : Activity() {
    private lateinit var r: Renderer
    private val main = Handler(Looper.getMainLooper())

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        val nimbus = NimbusView(this, NimbusSchema.COMBINED)
        setContentView(nimbus)
        r = nimbus.renderer
        check()
    }

    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        @Suppress("DEPRECATION")
        if (!r.back()) super.onBackPressed()
    }

    private fun check() {
        var before = 0
        waitFor("orders from the server", { count() > 0 }) {
            before = count()
            log("list: ${text("count")}")
            press("new")
            main.postDelayed({
                press("place") // empty: the server says no
                waitFor("server errors", { text("error-customer") != null }) {
                    log("empty order: server errors on the form: customer \"${text("error-customer")}\", phone \"${text("error-phone")}\"")
                    type("field-customer", "Kabir Mehta")
                    type("field-phone", "90000 22222")
                    main.postDelayed({
                        press("place")
                        waitFor("the list to refresh", { count() == before + 1 }) {
                            main.postDelayed({
                                val closed = r.tagForTestID("field-customer") == 0
                                val top = text("order-${before + 1}")
                                log("valid order: modal closed $closed, list now \"${text("count")}\", new row \"$top\"")
                                log(if (closed && top == "Kabir Mehta") "FULLSTACK RESULT: PASS" else "FULLSTACK RESULT: CHECK")
                            }, 800)
                        }
                    }, 500)
                }
            }, 1000)
        }
    }

    private fun count() = text("count")?.substringBefore(' ')?.toIntOrNull() ?: 0
    private fun text(id: String) = (r.viewForTag(r.tagForTestID(id)) as? TextView)?.text?.toString()
    private fun press(id: String) = r.tagForTestID(id).let { if (it != 0) r.press(it) else log("FAIL: no $id") }
    private fun type(id: String, s: String) = (r.viewForTag(r.tagForTestID(id)) as? EditText)?.setText(s)

    private fun waitFor(what: String, cond: () -> Boolean, tries: Int = 150, then: () -> Unit) {
        if (cond()) return then()
        if (tries == 0) return log("FAIL: timed out waiting for $what")
        main.postDelayed({ waitFor(what, cond, tries - 1, then) }, 100)
    }

    private fun log(s: String) = Log.i("NimbusNativeApp", "[fullstack] $s").let {}
}
