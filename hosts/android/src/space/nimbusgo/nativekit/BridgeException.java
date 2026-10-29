package space.nimbusgo.nativekit;

import java.nio.charset.StandardCharsets;

/** A failed call into the Go runtime. For a recovered Go panic, the message holds the panic and Go stack. */
public final class BridgeException extends RuntimeException {
    /** NN_ERR_* code from nn.h. */
    public final int code;

    BridgeException(int code, byte[] detail) {
        super(describe(code, detail));
        this.code = code;
    }

    private static String describe(int code, byte[] detail) {
        String what;
        switch (code) {
            case Bridge.ERR_ABI: what = "ABI version mismatch"; break;
            case Bridge.ERR_STATE: what = "runtime not started"; break;
            case Bridge.ERR_PANIC: what = "Go panicked"; break;
            case Bridge.ERR_NO_HANDLER: what = "no handler"; break;
            case Bridge.ERR_ARG: what = "bad argument"; break;
            default: what = "error " + code;
        }
        if (detail == null || detail.length == 0) return "nimbus-native: " + what;
        return "nimbus-native: " + what + "\n" + new String(detail, StandardCharsets.UTF_8);
    }
}
