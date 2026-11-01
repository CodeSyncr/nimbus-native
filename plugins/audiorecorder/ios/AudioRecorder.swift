import AVFoundation

/// audiorecorder: AVAudioRecorder, to an AAC file in the temporary folder.
enum AudioRecorderPlugin {
    private static var recorder: AVAudioRecorder?

    private static func finish() {
        recorder = nil
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
    }

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "request":
            AVAudioSession.sharedInstance().requestRecordPermission { call.resolve(["granted": $0]) }
        case "start":
            start(call)
        case "pause":
            recorder?.pause()
            call.resolve()
        case "resume":
            recorder?.record()
            call.resolve()
        case "status":
            guard let r = recorder else { return call.resolve(["recording": false, "seconds": 0, "level": 0]) }
            r.updateMeters()
            call.resolve(["recording": r.isRecording, "seconds": r.currentTime, "level": pow(10, Double(r.averagePower(forChannel: 0)) / 20)])
        case "stop":
            guard let r = recorder else { return call.reject(Plugins.failed, "nothing is recording") }
            r.stop()
            let data = try? Data(contentsOf: r.url)
            let seconds = AVURLAsset(url: r.url).duration.seconds
            try? FileManager.default.removeItem(at: r.url)
            finish()
            guard let data else { return call.reject(Plugins.failed, "the recording couldn't be read") }
            call.resolve(["data": data.base64EncodedString(), "type": "audio/mp4", "seconds": seconds.isFinite ? seconds : 0])
        case "cancel":
            recorder?.stop()
            recorder?.deleteRecording()
            finish()
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }

    private static func start(_ call: PluginCall) {
        let s = AVAudioSession.sharedInstance()
        guard s.recordPermission == .granted else { return call.reject(Plugins.denied, "the microphone isn't allowed: call Request first") }
        guard recorder == nil else { return call.reject(Plugins.failed, "it's already recording") }
        do {
            try s.setCategory(.playAndRecord, mode: .default, options: [.defaultToSpeaker])
            try s.setActive(true)
            let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".m4a")
            let r = try AVAudioRecorder(url: url, settings: [
                AVFormatIDKey: kAudioFormatMPEG4AAC, AVSampleRateKey: 44100, AVNumberOfChannelsKey: 1,
                AVEncoderAudioQualityKey: AVAudioQuality.high.rawValue,
            ])
            r.isMeteringEnabled = true
            guard r.record() else { return call.reject(Plugins.failed, "the microphone wouldn't start") }
            recorder = r
            call.resolve()
        } catch {
            call.reject(Plugins.failed, error.localizedDescription)
        }
    }
}
