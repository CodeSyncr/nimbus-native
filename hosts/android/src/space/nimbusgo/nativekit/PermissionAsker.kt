package space.nimbusgo.nativekit

import android.app.Activity
import android.content.pm.PackageManager

/** Asks for a permission from a plain Activity: the answer comes to a fragment, not the app's own code. */
@Suppress("DEPRECATION")
class PermissionAsker : android.app.Fragment() {
    private var permission = ""
    private var done: ((Boolean) -> Unit)? = null

    override fun onCreate(state: android.os.Bundle?) {
        super.onCreate(state)
        if (done == null) {
            fragmentManager.beginTransaction().remove(this).commit() // rebuilt after a restart: nobody is waiting
            return
        }
        requestPermissions(arrayOf(permission), 1)
    }

    override fun onRequestPermissionsResult(code: Int, permissions: Array<String>, results: IntArray) {
        done?.invoke(results.firstOrNull() == PackageManager.PERMISSION_GRANTED)
        done = null
        fragmentManager.beginTransaction().remove(this).commit()
    }

    internal companion object {
        fun ask(activity: Activity, permission: String, done: (Boolean) -> Unit) {
            val asker = PermissionAsker()
            asker.permission = permission
            asker.done = done
            activity.fragmentManager.beginTransaction().add(asker, "nimbus-permission").commit()
        }
    }
}

/** Starts another screen from a plain Activity and hands back its result, the same way. */
@Suppress("DEPRECATION")
class ResultAsker : android.app.Fragment() {
    private var intent: android.content.Intent? = null
    private var done: ((Int, android.content.Intent?) -> Unit)? = null

    override fun onCreate(state: android.os.Bundle?) {
        super.onCreate(state)
        val i = intent
        if (i == null) {
            fragmentManager.beginTransaction().remove(this).commit() // rebuilt after a restart: nobody is waiting
            return
        }
        startActivityForResult(i, 1)
    }

    override fun onActivityResult(request: Int, result: Int, data: android.content.Intent?) {
        done?.invoke(result, data)
        done = null
        fragmentManager.beginTransaction().remove(this).commit()
    }

    internal companion object {
        fun start(activity: Activity, intent: android.content.Intent, done: (Int, android.content.Intent?) -> Unit) {
            val asker = ResultAsker()
            asker.intent = intent
            asker.done = done
            activity.fragmentManager.beginTransaction().add(asker, "nimbus-result").commit()
        }
    }
}
