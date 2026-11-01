package space.nimbusgo.nativekit

import android.Manifest
import android.app.Activity
import android.content.ContentProviderOperation
import android.content.ContentUris
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.ContactsContract
import android.provider.ContactsContract.CommonDataKinds.Email
import android.provider.ContactsContract.CommonDataKinds.Phone
import android.provider.ContactsContract.CommonDataKinds.StructuredName
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
            "request" -> request(call)
            "list" -> {
                if (c.checkSelfPermission(Manifest.permission.READ_CONTACTS) != PackageManager.PERMISSION_GRANTED) return call.reject(Plugins.DENIED, "contacts are not allowed")
                thread {
                    val phones = values(Phone.CONTENT_URI, Phone.NUMBER, null, null)
                    val emails = values(Email.CONTENT_URI, Email.ADDRESS, null, null)
                    val out = JSONArray()
                    c.contentResolver.query(Contacts.CONTENT_URI, arrayOf(Contacts._ID, Contacts.DISPLAY_NAME), null, null, null)?.use {
                        while (it.moveToNext()) {
                            val id = it.getString(0)
                            out.put(JSONObject().put("id", id).put("name", it.getString(1) ?: "").put("phones", JSONArray(phones[id] ?: emptyList<String>()))
                                .put("emails", JSONArray(emails[id] ?: emptyList<String>())))
                        }
                    }
                    call.resolve(JSONObject().put("contacts", out))
                }
            }
            "add" -> {
                val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
                val i = Intent(ContactsContract.Intents.Insert.ACTION).setType(ContactsContract.RawContacts.CONTENT_TYPE)
                    .putExtra(ContactsContract.Intents.Insert.NAME, call.string("name") ?: "")
                call.args.optJSONArray("phones")?.takeIf { it.length() > 0 }?.let { i.putExtra(ContactsContract.Intents.Insert.PHONE, it.getString(0)) }
                call.args.optJSONArray("emails")?.takeIf { it.length() > 0 }?.let { i.putExtra(ContactsContract.Intents.Insert.EMAIL, it.getString(0)) }
                ResultAsker.start(a, i, { call.reject(Plugins.UNAVAILABLE, "this phone has no contacts app") }) { _, _ ->
                    call.resolve(mapOf("saved" to true))
                }
            }
            "create", "update", "delete" -> {
                if (c.checkSelfPermission(Manifest.permission.WRITE_CONTACTS) != PackageManager.PERMISSION_GRANTED) return call.reject(Plugins.DENIED, "contacts aren't allowed: call Request first")
                thread {
                    try {
                        write(call)
                    } catch (e: Exception) {
                        call.reject(Plugins.FAILED, e.toString())
                    }
                }
            }
            else -> Plugins.unknownMethod(call)
        }
    }

    private val needed = listOf(Manifest.permission.READ_CONTACTS, Manifest.permission.WRITE_CONTACTS)

    private fun request(call: PluginCall) {
        val next = needed.firstOrNull { Plugins.context.checkSelfPermission(it) != PackageManager.PERMISSION_GRANTED }
            ?: return call.resolve(mapOf("granted" to true))
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no activity to ask from")
        PermissionAsker.ask(a, next) { ok -> if (ok) request(call) else call.resolve(mapOf("granted" to false)) }
    }

    private fun write(call: PluginCall) {
        val r = Plugins.context.contentResolver
        val id = call.string("id") ?: ""
        if (call.method == "delete") {
            val n = r.delete(ContactsContract.RawContacts.CONTENT_URI, "${ContactsContract.RawContacts.CONTACT_ID} = ?", arrayOf(id))
            return if (n == 0) call.reject(Plugins.NOT_FOUND, "no such contact") else call.resolve(mapOf("id" to id))
        }
        val ops = ArrayList<ContentProviderOperation>()
        val data = ContactsContract.Data.CONTENT_URI
        val row = { kind: String, raw: Long?, back: Int ->
            ContentProviderOperation.newInsert(data).withValue(ContactsContract.Data.MIMETYPE, kind).let {
                if (raw != null) it.withValue(ContactsContract.Data.RAW_CONTACT_ID, raw) else it.withValueBackReference(ContactsContract.Data.RAW_CONTACT_ID, back)
            }
        }
        var raw: Long? = null
        if (call.method == "update") {
            r.query(ContactsContract.RawContacts.CONTENT_URI, arrayOf(ContactsContract.RawContacts._ID), "${ContactsContract.RawContacts.CONTACT_ID} = ?", arrayOf(id), null)?.use {
                if (it.moveToFirst()) raw = it.getLong(0)
            }
            if (raw == null) return call.reject(Plugins.NOT_FOUND, "no such contact")
            val kinds = arrayOf(StructuredName.CONTENT_ITEM_TYPE, Phone.CONTENT_ITEM_TYPE, Email.CONTENT_ITEM_TYPE)
            ops.add(ContentProviderOperation.newDelete(data)
                .withSelection("${ContactsContract.Data.RAW_CONTACT_ID} = ? AND ${ContactsContract.Data.MIMETYPE} IN (?, ?, ?)", arrayOf(raw.toString(), *kinds)).build())
        } else {
            ops.add(ContentProviderOperation.newInsert(ContactsContract.RawContacts.CONTENT_URI)
                .withValue(ContactsContract.RawContacts.ACCOUNT_TYPE, null).withValue(ContactsContract.RawContacts.ACCOUNT_NAME, null).build())
        }
        ops.add(row(StructuredName.CONTENT_ITEM_TYPE, raw, 0).withValue(StructuredName.DISPLAY_NAME, call.string("name") ?: "").build())
        val phones = call.args.optJSONArray("phones")
        for (i in 0 until (phones?.length() ?: 0)) ops.add(row(Phone.CONTENT_ITEM_TYPE, raw, 0).withValue(Phone.NUMBER, phones!!.getString(i)).withValue(Phone.TYPE, Phone.TYPE_MOBILE).build())
        val emails = call.args.optJSONArray("emails")
        for (i in 0 until (emails?.length() ?: 0)) ops.add(row(Email.CONTENT_ITEM_TYPE, raw, 0).withValue(Email.ADDRESS, emails!!.getString(i)).withValue(Email.TYPE, Email.TYPE_OTHER).build())
        val results = r.applyBatch(ContactsContract.AUTHORITY, ops)
        if (call.method == "update") return call.resolve(mapOf("id" to id))
        val rawId = ContentUris.parseId(results[0].uri!!)
        var made = ""
        r.query(ContactsContract.RawContacts.CONTENT_URI, arrayOf(ContactsContract.RawContacts.CONTACT_ID), "${ContactsContract.RawContacts._ID} = ?", arrayOf(rawId.toString()), null)?.use {
            if (it.moveToFirst()) made = it.getString(0)
        }
        call.resolve(mapOf("id" to made))
    }
}
