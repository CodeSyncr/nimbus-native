import AVFoundation
import Speech

/// speechrecognition: the system's recognizer, fed from the microphone.
enum SpeechRecognitionPlugin {
    private static var engine: AVAudioEngine?
    private static var request: SFSpeechAudioBufferRecognitionRequest?
    private static var task: SFSpeechRecognitionTask?
    private static var text = ""
    private static var done = false

    private static func stop() {
        engine?.inputNode.removeTap(onBus: 0)
        engine?.stop()
        engine = nil
        request?.endAudio()
        request = nil
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            SFSpeechRecognizer.requestAuthorization { s in
                guard s == .authorized else { return call.resolve(["granted": false]) }
                AVAudioSession.sharedInstance().requestRecordPermission { call.resolve(["granted": $0]) }
            }
        case "start":
            let lang = call.string("language") ?? ""
            guard let r = lang.isEmpty ? SFSpeechRecognizer() : SFSpeechRecognizer(locale: Locale(identifier: lang)), r.isAvailable else {
                return call.reject(Plugins.unavailable, "speech recognition isn't available here")
            }
            guard SFSpeechRecognizer.authorizationStatus() == .authorized else { return call.reject(Plugins.denied, "speech recognition is not allowed") }
            stop()
            text = ""
            done = false
            do {
                try AVAudioSession.sharedInstance().setCategory(.playAndRecord, mode: .measurement, options: [.defaultToSpeaker])
                try AVAudioSession.sharedInstance().setActive(true)
                let e = AVAudioEngine()
                let req = SFSpeechAudioBufferRecognitionRequest()
                req.shouldReportPartialResults = true
                let format = e.inputNode.outputFormat(forBus: 0)
                e.inputNode.installTap(onBus: 0, bufferSize: 1024, format: format) { b, _ in req.append(b) }
                e.prepare()
                try e.start()
                engine = e
                request = req
                task = r.recognitionTask(with: req) { result, error in
                    DispatchQueue.main.async {
                        if let result { text = result.bestTranscription.formattedString }
                        if error != nil || result?.isFinal == true {
                            done = true
                            stop()
                        }
                    }
                }
                call.resolve()
            } catch {
                stop()
                call.reject(Plugins.failed, "the microphone couldn't start: \(error)")
            }
        case "read":
            call.resolve(["text": text, "done": done])
        case "stop":
            stop()
            task?.cancel()
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }
}
