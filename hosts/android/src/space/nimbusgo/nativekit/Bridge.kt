package space.nimbusgo.nativekit

import android.os.Handler
import android.os.Looper
import java.nio.ByteBuffer
import java.nio.ByteOrder

/** The Kotlin side of the Nimbus Native C ABI (nn.h), bound by jni_android.c. */
object Bridge {
    /** Receives every message Go sends, on the main thread, in order. */
    fun interface Listener {
        fun onMessage(msg: ByteArray)
    }

    /**
     * Answers Go's synchronous questions (text measurement, native modules). Called on a Go thread,
     * never the main thread: it must not wait on main. Return null for "no answer".
     */
    fun interface HostHandler {
        fun onCall(req: ByteArray): ByteArray?
    }

    /** NN_ERR_* codes from nn.h. */
    const val ERR_ABI = 1
    const val ERR_STATE = 2
    const val ERR_PANIC = 3
    const val ERR_NO_HANDLER = 4
    const val ERR_ARG = 5
    const val ERR_SCHEMA = 6

    init {
        System.loadLibrary("nimbusapp") // runs JNI_OnLoad, which registers the natives below
    }

    private val main = Handler(Looper.getMainLooper())
    private val drainTask = Runnable { drain() }

    @Volatile private var listener: Listener? = null
    @Volatile private var hostHandler: HostHandler? = null

    @JvmStatic
    fun setListener(l: Listener?) {
        listener = l
    }

    /** Set before start. */
    @JvmStatic
    fun setHostHandler(h: HostHandler?) {
        hostHandler = h
    }

    /**
     * Boots the app's Go side. Call once, at launch, with the generated NimbusSchema.COMBINED; the
     * runtime refuses to start (ERR_SCHEMA) if the Go side was generated from a different schema.
     */
    @JvmStatic
    fun start(schema: Long) = check(nativeStart(schema))

    /** Native → Go, asynchronous. */
    @JvmStatic
    fun send(msg: ByteArray) = check(nativeSend(msg))

    /** Native → Go, synchronous. Throws BridgeException if Go fails or panics. */
    @JvmStatic
    fun call(req: ByteArray): ByteArray = nativeCall(req) ?: ByteArray(0)

    // Called by the runtime, from a Go thread, when its outbox stops being
    // empty. One post per batch, however many messages it holds.
    @JvmStatic
    fun onWake() {
        main.post(drainTask)
    }

    // Called by the runtime, from a Go thread.
    @JvmStatic
    fun onHostCall(req: ByteArray): ByteArray? = hostHandler?.onCall(req)

    private fun drain() {
        val batch = nativeDrain() ?: return
        val buf = ByteBuffer.wrap(batch).order(ByteOrder.LITTLE_ENDIAN)
        val l = listener
        while (buf.remaining() >= 4) {
            val msg = ByteArray(buf.int)
            buf.get(msg)
            l?.onMessage(msg)
        }
    }

    private fun check(rc: Int) {
        if (rc != 0) throw BridgeException(rc, null)
    }

    @JvmStatic private external fun nativeStart(schema: Long): Int

    @JvmStatic private external fun nativeSend(msg: ByteArray): Int

    @JvmStatic private external fun nativeDrain(): ByteArray?

    @JvmStatic private external fun nativeCall(req: ByteArray): ByteArray?
}

/** A failed call into the Go runtime. For a recovered Go panic, the message holds the panic and Go stack. */
class BridgeException(
    /** NN_ERR_* code from nn.h. */
    @JvmField val code: Int,
    detail: ByteArray?,
) : RuntimeException(describe(code, detail)) {
    private companion object {
        fun describe(code: Int, detail: ByteArray?): String {
            val what = when (code) {
                Bridge.ERR_ABI -> "ABI version mismatch"
                Bridge.ERR_STATE -> "runtime not started"
                Bridge.ERR_PANIC -> "Go panicked"
                Bridge.ERR_NO_HANDLER -> "no handler"
                Bridge.ERR_ARG -> "bad argument"
                Bridge.ERR_SCHEMA -> "wire schema mismatch; regenerate with nimbus native gen"
                else -> "error $code"
            }
            if (detail == null || detail.isEmpty()) return "nimbus-native: $what"
            return "nimbus-native: $what\n" + String(detail, Charsets.UTF_8)
        }
    }
}
