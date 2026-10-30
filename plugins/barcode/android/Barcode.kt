package space.nimbusgo.nativekit

import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning

/** barcode: Google Play services' scanner, which needs no camera permission. */
internal object BarcodePlugin {
    private val names = mapOf(Barcode.FORMAT_QR_CODE to "qr", Barcode.FORMAT_EAN_13 to "ean13", Barcode.FORMAT_EAN_8 to "ean8",
        Barcode.FORMAT_CODE_128 to "code128", Barcode.FORMAT_CODE_39 to "code39", Barcode.FORMAT_UPC_E to "upce",
        Barcode.FORMAT_PDF417 to "pdf417", Barcode.FORMAT_DATA_MATRIX to "datamatrix")

    fun handle(call: PluginCall) {
        if (call.method != "scan") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        GmsBarcodeScanning.getClient(a).startScan()
            .addOnSuccessListener { call.resolve(mapOf("value" to (it.rawValue ?: ""), "format" to (names[it.format] ?: "other"))) }
            .addOnCanceledListener { call.reject(Plugins.CANCELLED, "no code") }
            .addOnFailureListener { call.reject(Plugins.UNAVAILABLE, it.toString()) }
    }
}
