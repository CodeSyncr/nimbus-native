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
