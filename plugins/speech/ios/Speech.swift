import AVFoundation

/// speech: the system's synthesizer.
enum SpeechPlugin {
    private static let voice = AVSpeechSynthesizer()

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "say":
            let u = AVSpeechUtterance(string: call.string("text") ?? "")
            if let l = call.string("language"), !l.isEmpty { u.voice = AVSpeechSynthesisVoice(language: l) }
            if let r = (call.args["rate"] as? NSNumber)?.floatValue, r > 0 { u.rate = min(AVSpeechUtteranceDefaultSpeechRate * r, AVSpeechUtteranceMaximumSpeechRate) }
            voice.speak(u)
            call.resolve()
        case "speaking":
            call.resolve(["speaking": voice.isSpeaking])
        case "stop":
            voice.stopSpeaking(at: .immediate)
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
