package space.nimbusgo.nativekit

import android.content.Context
import android.os.Bundle
import android.os.CancellationSignal
import android.os.ParcelFileDescriptor
import android.print.PageRange
import android.print.PrintAttributes
import android.print.PrintDocumentAdapter
import android.print.PrintDocumentInfo
import android.print.PrintManager
import android.util.Base64
import android.webkit.WebView
import android.webkit.WebViewClient
import java.io.FileOutputStream

/** printing: HTML through a web view that's never shown, and a PDF as it is. */
internal object PrintingPlugin {
    private var page: WebView? = null // kept alive until the dialog has what it needs

    fun handle(call: PluginCall) {
        if (call.method != "print") return Plugins.unknownMethod(call)
        val a = Plugins.activity ?: return call.reject(Plugins.FAILED, "no screen to open it from")
        val name = call.string("name") ?: "Document"
        val printer = a.getSystemService(Context.PRINT_SERVICE) as PrintManager
        val pdf = call.string("pdf")
        val html = call.string("html")
        when {
            !pdf.isNullOrEmpty() -> {
                printer.print(name, PdfAdapter(name, Base64.decode(pdf, Base64.DEFAULT)), null)
                call.resolve()
            }
            !html.isNullOrEmpty() -> {
                val v = WebView(a)
                page = v
                v.webViewClient = object : WebViewClient() {
                    override fun onPageFinished(w: WebView, url: String) {
                        printer.print(name, w.createPrintDocumentAdapter(name), PrintAttributes.Builder().build())
                        page = null
                        call.resolve()
                    }
                }
                v.loadDataWithBaseURL(null, html, "text/html", "utf-8", null)
            }
            else -> call.reject(Plugins.FAILED, "nothing to print")
        }
    }

    private class PdfAdapter(val name: String, val bytes: ByteArray) : PrintDocumentAdapter() {
        override fun onLayout(old: PrintAttributes?, new: PrintAttributes?, c: CancellationSignal?, r: LayoutResultCallback, e: Bundle?) {
            if (c?.isCanceled == true) return r.onLayoutCancelled()
            r.onLayoutFinished(PrintDocumentInfo.Builder(name).setContentType(PrintDocumentInfo.CONTENT_TYPE_DOCUMENT).build(), true)
        }

        override fun onWrite(pages: Array<out PageRange>?, d: ParcelFileDescriptor, c: CancellationSignal?, r: WriteResultCallback) {
            try {
                FileOutputStream(d.fileDescriptor).use { it.write(bytes) }
                r.onWriteFinished(arrayOf(PageRange.ALL_PAGES))
            } catch (e: java.io.IOException) {
                r.onWriteFailed(e.message)
            }
        }
    }
}
