package space.nimbusgo.nativekit;

import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** Decodes the Nimbus Native wire format. Throws WireException on bad input. */
public final class WireReader {
    /** Reads one value; generated code passes lambdas for list and map elements. */
    public interface Read<T> {
        T read(WireReader r);
    }

    private final byte[] b;
    private int pos;

    public WireReader(byte[] bytes) {
        b = bytes;
    }

    public boolean bool() {
        need(1);
        int v = b[pos++] & 0xff;
        if (v > 1) throw new WireException("value out of range for its type");
        return v == 1;
    }

    /** Unsigned LEB128, with the same overflow rule as Go's binary.Uvarint. */
    public long uint() {
        long x = 0;
        int shift = 0;
        for (int i = 0; i < 10; i++) {
            need(1);
            int c = b[pos++] & 0xff;
            if (c < 0x80) {
                if (i == 9 && c > 1) break;
                return x | ((long) c << shift);
            }
            x |= (long) (c & 0x7f) << shift;
            shift += 7;
        }
        throw new WireException("varint overflows 64 bits");
    }

    public long sint() {
        long u = uint();
        return (u >>> 1) ^ -(u & 1);
    }

    public long uintMax(long max) {
        long v = uint();
        if (Long.compareUnsigned(v, max) > 0) throw new WireException("value out of range for its type");
        return v;
    }

    public long intRange(long min, long max) {
        long v = sint();
        if (v < min || v > max) throw new WireException("value out of range for its type");
        return v;
    }

    public float f32() {
        return Float.intBitsToFloat((int) fixed(4));
    }

    public double f64() {
        return Double.longBitsToDouble(fixed(8));
    }

    public String string() {
        return new String(bytes(), StandardCharsets.UTF_8);
    }

    public byte[] bytes() {
        int len = count();
        byte[] out = new byte[len];
        System.arraycopy(b, pos, out, 0, len);
        pos += len;
        return out;
    }

    /** A list or map length; a count beyond the remaining bytes is corrupt. */
    public int count() {
        long v = uint();
        if (Long.compareUnsigned(v, b.length - pos) > 0) throw new WireException("message ends early");
        return (int) v;
    }

    public <T> List<T> list(Read<T> element) {
        int len = count();
        List<T> out = new ArrayList<>(Math.min(len, 4096));
        for (int i = 0; i < len; i++) out.add(element.read(this));
        return out;
    }

    public <K, V> Map<K, V> map(Read<K> key, Read<V> value) {
        int len = count();
        Map<K, V> out = new HashMap<>(Math.min(len, 4096));
        for (int i = 0; i < len; i++) {
            K k = key.read(this);
            out.put(k, value.read(this));
        }
        return out;
    }

    /** Throws if bytes remain after a whole message. */
    public void done() {
        if (pos != b.length) throw new WireException("bytes left over after the message");
    }

    private long fixed(int n) {
        need(n);
        long v = 0;
        for (int i = 0; i < n; i++) v |= (long) (b[pos + i] & 0xff) << (8 * i);
        pos += n;
        return v;
    }

    private void need(int n) {
        if (b.length - pos < n) throw new WireException("message ends early");
    }
}
