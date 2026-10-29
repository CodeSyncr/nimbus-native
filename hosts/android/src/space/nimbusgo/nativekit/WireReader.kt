package space.nimbusgo.nativekit

/** Decodes the Nimbus Native wire format. Throws WireException on bad input. */
class WireReader(private val b: ByteArray) {
    private var pos = 0

    fun bool(): Boolean {
        need(1)
        val v = b[pos++].toInt() and 0xff
        if (v > 1) throw WireException("value out of range for its type")
        return v == 1
    }

    /** Unsigned LEB128, with the same overflow rule as Go's binary.Uvarint. */
    fun uint(): Long {
        var x = 0L
        var shift = 0
        for (i in 0 until 10) {
            need(1)
            val c = b[pos++].toInt() and 0xff
            if (c < 0x80) {
                if (i == 9 && c > 1) break
                return x or (c.toLong() shl shift)
            }
            x = x or ((c and 0x7f).toLong() shl shift)
            shift += 7
        }
        throw WireException("varint overflows 64 bits")
    }

    fun sint(): Long {
        val u = uint()
        return (u ushr 1) xor -(u and 1)
    }

    fun uintMax(max: Long): Long {
        val v = uint()
        if (java.lang.Long.compareUnsigned(v, max) > 0) throw WireException("value out of range for its type")
        return v
    }

    fun intRange(min: Long, max: Long): Long {
        val v = sint()
        if (v < min || v > max) throw WireException("value out of range for its type")
        return v
    }

    fun f32(): Float = Float.fromBits(fixed(4).toInt())

    fun f64(): Double = Double.fromBits(fixed(8))

    fun string(): String = String(bytes(), Charsets.UTF_8)

    fun bytes(): ByteArray {
        val len = count()
        val out = b.copyOfRange(pos, pos + len)
        pos += len
        return out
    }

    /** A list or map length; a count beyond the remaining bytes is corrupt. */
    fun count(): Int {
        val v = uint()
        if (java.lang.Long.compareUnsigned(v, (b.size - pos).toLong()) > 0) throw WireException("message ends early")
        return v.toInt()
    }

    fun <T> list(element: (WireReader) -> T): MutableList<T> {
        val len = count()
        val out = ArrayList<T>(minOf(len, 4096))
        repeat(len) { out.add(element(this)) }
        return out
    }

    fun <K, V> map(key: (WireReader) -> K, value: (WireReader) -> V): MutableMap<K, V> {
        val len = count()
        val out = HashMap<K, V>(minOf(len, 4096))
        repeat(len) {
            val k = key(this)
            out[k] = value(this)
        }
        return out
    }

    /** Throws if bytes remain after a whole message. */
    fun done() {
        if (pos != b.size) throw WireException("bytes left over after the message")
    }

    private fun fixed(n: Int): Long {
        need(n)
        var v = 0L
        for (i in 0 until n) v = v or ((b[pos + i].toLong() and 0xff) shl (8 * i))
        pos += n
        return v
    }

    private fun need(n: Int) {
        if (b.size - pos < n) throw WireException("message ends early")
    }
}
