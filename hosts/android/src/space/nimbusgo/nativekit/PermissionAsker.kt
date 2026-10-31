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
    private var missing: (() -> Unit)? = null

    override fun onCreate(state: android.os.Bundle?) {
        super.onCreate(state)
        val i = intent
        if (i == null) {
            fragmentManager.beginTransaction().remove(this).commit() // rebuilt after a restart: nobody is waiting
            return
        }
        try {
            startActivityForResult(i, 1)
        } catch (e: android.content.ActivityNotFoundException) {
            done = null
            missing?.invoke()
            fragmentManager.beginTransaction().remove(this).commit()
        }
    }

    override fun onActivityResult(request: Int, result: Int, data: android.content.Intent?) {
        done?.invoke(result, data)
        done = null
        fragmentManager.beginTransaction().remove(this).commit()
    }

    internal companion object {
        /** missing runs if no app on the phone handles the intent. Asking beforehand doesn't work:
         *  since Android 11 other apps are hidden from an app that hasn't declared them. */
        fun start(activity: Activity, intent: android.content.Intent, missing: (() -> Unit)? = null, done: (Int, android.content.Intent?) -> Unit) {
            val asker = ResultAsker()
            asker.intent = intent
            asker.done = done
            asker.missing = missing
            activity.fragmentManager.beginTransaction().add(asker, "nimbus-result").commit()
        }
    }
}
