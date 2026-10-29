package space.nimbusgo.nativekit

/** A message that can't be decoded: truncated, corrupt, or from another schema. */
class WireException(message: String) : RuntimeException("wire: $message")
