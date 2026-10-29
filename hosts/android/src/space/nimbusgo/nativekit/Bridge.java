package space.nimbusgo.nativekit;

import android.os.Handler;
import android.os.Looper;

/** The Java side of the Nimbus Native C ABI (nn.h), bound by jni_android.c. */
public final class Bridge {
    /** Receives every message Go sends, on the main thread, in order. */
    public interface Listener {
        void onMessage(byte[] msg);
    }

    /** NN_ERR_* codes from nn.h. */
    public static final int ERR_ABI = 1, ERR_STATE = 2, ERR_PANIC = 3, ERR_NO_HANDLER = 4;

    static {
        System.loadLibrary("nimbusapp"); // runs JNI_OnLoad, which registers the natives below
    }

    private static final Handler main = new Handler(Looper.getMainLooper());
    private static volatile Listener listener;

    private Bridge() {}

    public static void setListener(Listener l) {
        listener = l;
    }

    /** Boots the app's Go side. Call once, at launch. */
    public static void start() {
        check(nativeStart());
    }

    /** Native → Go, asynchronous. */
    public static void send(byte[] msg) {
        check(nativeSend(msg));
    }

    /** Native → Go, synchronous. Throws IllegalStateException if Go fails or panics. */
    public static byte[] call(byte[] req) {
        return nativeCall(req);
    }

    // Called by the runtime, from Go threads.
    static void onReceive(byte[] msg) {
        main.post(() -> {
            Listener l = listener;
            if (l != null) l.onMessage(msg);
        });
    }

    private static void check(int rc) {
        if (rc != 0) throw new IllegalStateException("nimbus-native error " + rc);
    }

    private static native int nativeStart();
    private static native int nativeSend(byte[] msg);
    private static native byte[] nativeCall(byte[] req);
}
