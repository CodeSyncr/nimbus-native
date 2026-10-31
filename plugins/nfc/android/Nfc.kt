package space.nimbusgo.nativekit

import android.nfc.NdefRecord
import android.nfc.NfcAdapter
import android.nfc.Tag
import android.nfc.tech.Ndef
import android.os.Handler
import android.os.Looper
import org.json.JSONArray
import org.json.JSONObject

/** nfc: reader mode, while the app is in front. */
internal object NfcPlugin {
    private fun text(r: NdefRecord): String = r.toUri()?.toString() ?: run {
        val p = r.payload ?: return ""
        if (r.tnf == NdefRecord.TNF_WELL_KNOWN && r.type.contentEquals(NdefRecord.RTD_TEXT) && p.isNotEmpty()) {
            String(p, 1 + (p[0].toInt() and 63), p.size - 1 - (p[0].toInt() and 63))
        } else {
            String(p)
        }
    }

    fun handle(call: PluginCall) {
        val a = Plugins.activity
        val adapter = NfcAdapter.getDefaultAdapter(Plugins.context)
        when (call.method) {
            "available" -> call.resolve(mapOf("available" to (adapter?.isEnabled == true)))
            "read" -> {
                if (adapter == null || !adapter.isEnabled || a == null) return call.reject(Plugins.UNAVAILABLE, "this phone can't read NFC")
                val flags = NfcAdapter.FLAG_READER_NFC_A or NfcAdapter.FLAG_READER_NFC_B or NfcAdapter.FLAG_READER_NFC_F or NfcAdapter.FLAG_READER_NFC_V
                adapter.enableReaderMode(a, { tag: Tag ->
                    val records = JSONArray()
                    try {
                        Ndef.get(tag)?.use { n ->
                            n.connect()
                            n.ndefMessage?.records?.forEach { records.put(text(it)) }
                        }
                    } catch (e: Exception) {
                        // an ID with no readable records is still a tag
                    }
                    Handler(Looper.getMainLooper()).post { adapter.disableReaderMode(a) }
                    call.resolve(JSONObject().put("id", tag.id.joinToString("") { "%02x".format(it) }).put("records", records))
                }, flags, null)
            }
            else -> Plugins.unknownMethod(call)
        }
    }
}
