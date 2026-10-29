package space.nimbusgo.nativekit;

import java.nio.charset.StandardCharsets;

/**
 * Encodes the Nimbus Native wire format (see wire/wire.go). Generated code calls
 * these; plain Java, API 26-safe, so it also runs in the desktop conformance test.
 */
public final class WireWriter {
    private byte[] buf = new byte[64];
    private int n;

    public void bool(boolean v) {
        put(v ? 1 : 0);
    }

    /** Unsigned LEB128; v is treated as an unsigned 64-bit value. */
    public void uint(long v) {
        while (Long.compareUnsigned(v, 0x80) >= 0) {
            put((int) (v & 0x7f) | 0x80);
            v >>>= 7;
        }
        put((int) v);
    }

    /** Zigzag, then varint. */
    public void sint(long v) {
        uint((v << 1) ^ (v >> 63));
    }

    public void f32(float v) {
        fixed(Float.floatToRawIntBits(v) & 0xffffffffL, 4);
    }

    public void f64(double v) {
        fixed(Double.doubleToRawLongBits(v), 8);
    }

    public void string(String v) {
        bytes(v.getBytes(StandardCharsets.UTF_8));
    }

    public void bytes(byte[] v) {
        uint(v.length);
        grow(v.length);
        System.arraycopy(v, 0, buf, n, v.length);
        n += v.length;
    }

    public byte[] toByteArray() {
        byte[] out = new byte[n];
        System.arraycopy(buf, 0, out, 0, n);
        return out;
    }

    /** Map keys go on the wire sorted; strings by their UTF-8 bytes, like Go. */
    public static int compareUtf8(String a, String b) {
        byte[] x = a.getBytes(StandardCharsets.UTF_8), y = b.getBytes(StandardCharsets.UTF_8);
        for (int i = 0; i < x.length && i < y.length; i++) {
            int c = (x[i] & 0xff) - (y[i] & 0xff);
            if (c != 0) return c;
        }
        return x.length - y.length;
    }

    private void fixed(long v, int bytes) {
        for (int i = 0; i < bytes; i++) put((int) (v >>> (8 * i)) & 0xff);
    }

    private void put(int b) {
        grow(1);
        buf[n++] = (byte) b;
    }

    private void grow(int more) {
        if (n + more <= buf.length) return;
        byte[] next = new byte[Math.max(buf.length * 2, n + more)];
        System.arraycopy(buf, 0, next, 0, n);
        buf = next;
    }
}
