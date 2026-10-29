// Conformance host: decodes each case with the generated Kitchen types,
// re-encodes it, and writes [ok u8][len u32][bytes] per case.
import java.io.ByteArrayOutputStream;
import java.nio.file.Files;
import java.nio.file.Paths;
import space.nimbusgo.nativekit.WireException;
import test.wire.Kitchen;

public class Main {
    public static void main(String[] args) throws Exception {
        byte[] in = Files.readAllBytes(Paths.get(args[0]));
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        int pos = 0;
        while (pos < in.length) {
            int n = u32(in, pos);
            pos += 4;
            byte[] msg = new byte[n];
            System.arraycopy(in, pos, msg, 0, n);
            pos += n;
            byte[] enc;
            try {
                enc = Kitchen.encode(Kitchen.decode(msg));
            } catch (WireException e) {
                enc = null;
            }
            out.write(enc == null ? 0 : 1);
            int len = enc == null ? 0 : enc.length;
            for (int i = 0; i < 4; i++) out.write(len >>> (8 * i));
            if (enc != null) out.write(enc);
        }
        Files.write(Paths.get(args[1]), out.toByteArray());
    }

    private static int u32(byte[] b, int p) {
        return (b[p] & 0xff) | (b[p + 1] & 0xff) << 8 | (b[p + 2] & 0xff) << 16 | (b[p + 3] & 0xff) << 24;
    }
}
