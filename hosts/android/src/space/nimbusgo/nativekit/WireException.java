package space.nimbusgo.nativekit;

/** A message that can't be decoded: truncated, corrupt, or from another schema. */
public final class WireException extends RuntimeException {
    private static final long serialVersionUID = 1L;

    public WireException(String message) {
        super("wire: " + message);
    }
}
