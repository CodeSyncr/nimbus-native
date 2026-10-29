// Conformance host: decodes each case with the generated Kitchen types,
// re-encodes it, and writes [ok u8][len u32][bytes] per case.
import java.io.ByteArrayOutputStream
import java.io.File
import space.nimbusgo.nativekit.WireException
import test.wire.Kitchen

fun main(args: Array<String>) {
    val input = File(args[0]).readBytes()
    val out = ByteArrayOutputStream()
    var pos = 0
    fun u32(p: Int) = (input[p].toInt() and 0xff) or ((input[p + 1].toInt() and 0xff) shl 8) or
        ((input[p + 2].toInt() and 0xff) shl 16) or ((input[p + 3].toInt() and 0xff) shl 24)
    while (pos < input.size) {
        val n = u32(pos)
        pos += 4
        val msg = input.copyOfRange(pos, pos + n)
        pos += n
        val enc = try {
            Kitchen.encode(Kitchen.decode(msg))
        } catch (e: WireException) {
            null
        }
        out.write(if (enc == null) 0 else 1)
        val len = enc?.size ?: 0
        for (i in 0 until 4) out.write(len ushr (8 * i))
        if (enc != null) out.write(enc)
    }
    File(args[1]).writeBytes(out.toByteArray())
}
