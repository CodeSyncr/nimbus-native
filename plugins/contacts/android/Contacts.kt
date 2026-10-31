package space.nimbusgo.nativekit

import android.Manifest
import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.ContactsContract.CommonDataKinds.Email
import android.provider.ContactsContract.CommonDataKinds.Phone
import android.provider.ContactsContract.Contacts
import org.json.JSONArray
import org.json.JSONObject
import kotlin.concurrent.thread

/** contacts: the system picker, and the provider with permission. */
internal object ContactsPlugin {
    private fun values(uri: Uri, column: String, select: String?, args: Array<String>?): MutableMap<String, MutableList<String>> {
        val out = HashMap<String, MutableList<String>>()
        Plugins.context.contentResolver.query(uri, arrayOf(Phone.CONTACT_ID, column), select, args, null)?.use {
            while (it.moveToNext()) out.getOrPut(it.getString(0)) { ArrayList() }.add(it.getString(1) ?: "")
        }
        return out
    }

    fun handle(call: PluginCall) {
        val c = Plugins.context
        when (call.method) {
            "pick" -> {
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
                ResultAsker.start(a, Intent(Intent.ACTION_PICK, Phone.CONTENT_URI)) { result, data ->
                    val uri = data?.data
                    if (result != Activity.RESULT_OK || uri == null) return@start call.reject(Plugins.CANCELLED, "no contact")
                    c.contentResolver.query(uri, arrayOf(Phone.DISPLAY_NAME, Phone.NUMBER), null, null, null)?.use {
                        if (it.moveToFirst()) return@start call.resolve(JSONObject().put("name", it.getString(0) ?: "")
                            .put("phones", JSONArray().put(it.getString(1) ?: "")).put("emails", JSONArray()))
                    }
                    call.reject(Plugins.FAILED, "that contact couldn't be read")
                }
            }
            "request" -> {
                if (c.checkSelfPermission(Manifest.permission.READ_CONTACTS) == PackageManager.PERMISSION_GRANTED) return call.resolve(mapOf("granted" to true))
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
                PermissionAsker.ask(a, Manifest.permission.READ_CONTACTS) { call.resolve(mapOf("granted" to it)) }
            }
            "list" -> {
                if (c.checkSelfPermission(Manifest.permission.READ_CONTACTS) != PackageManager.PERMISSION_GRANTED) return call.reject(Plugins.DENIED, "contacts are not allowed")
                thread {
                    val phones = values(Phone.CONTENT_URI, Phone.NUMBER, null, null)
                    val emails = values(Email.CONTENT_URI, Email.ADDRESS, null, null)
                    val out = JSONArray()
                    c.contentResolver.query(Contacts.CONTENT_URI, arrayOf(Contacts._ID, Contacts.DISPLAY_NAME), null, null, null)?.use {
                        while (it.moveToNext()) {
                            val id = it.getString(0)
                            out.put(JSONObject().put("name", it.getString(1) ?: "").put("phones", JSONArray(phones[id] ?: emptyList<String>()))
                                .put("emails", JSONArray(emails[id] ?: emptyList<String>())))
                        }
                    }
                    call.resolve(JSONObject().put("contacts", out))
                }
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
