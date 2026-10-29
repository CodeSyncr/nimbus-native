package space.nimbusgo.nativekit

/**
 * Encodes the Nimbus Native wire format (see wire/wire.go). Generated code calls these. It uses
 * no Android APIs, so it also runs in the desktop conformance test.
 */
class WireWriter {
    private var buf = ByteArray(64)
    private var n = 0

    fun bool(v: Boolean) = put(if (v) 1 else 0)

    /** Unsigned LEB128; v is treated as an unsigned 64-bit value. */
    fun uint(v: Long) {
        var x = v
        while (java.lang.Long.compareUnsigned(x, 0x80) >= 0) {
            put((x and 0x7f).toInt() or 0x80)
            x = x ushr 7
        }
        put(x.toInt())
    }

    /** Zigzag, then varint. */
    fun sint(v: Long) = uint((v shl 1) xor (v shr 63))

    fun f32(v: Float) = fixed(v.toRawBits().toLong() and 0xffffffffL, 4)

    fun f64(v: Double) = fixed(v.toRawBits(), 8)

    fun string(v: String) = bytes(v.toByteArray(Charsets.UTF_8))

    fun bytes(v: ByteArray) {
        uint(v.size.toLong())
        grow(v.size)
        v.copyInto(buf, n)
        n += v.size
    }

    fun toByteArray(): ByteArray = buf.copyOf(n)

    private fun fixed(v: Long, size: Int) {
        for (i in 0 until size) put((v ushr (8 * i)).toInt() and 0xff)
    }

    private fun put(b: Int) {
        grow(1)
        buf[n++] = b.toByte()
    }

    private fun grow(more: Int) {
        if (n + more > buf.size) buf = buf.copyOf(maxOf(buf.size * 2, n + more))
    }

    companion object {
        /** Map keys go on the wire sorted; strings by their UTF-8 bytes, like Go. */
        val UTF8_ORDER: Comparator<String> = Comparator { a, b ->
            val x = a.toByteArray(Charsets.UTF_8)
            val y = b.toByteArray(Charsets.UTF_8)
            for (i in 0 until minOf(x.size, y.size)) {
                val c = (x[i].toInt() and 0xff) - (y[i].toInt() and 0xff)
                if (c != 0) return@Comparator c
            }
            x.size - y.size
        }
    }
}
