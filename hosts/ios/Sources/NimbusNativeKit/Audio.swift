import AVFoundation

/// audio: the microphone, as 16 kHz mono PCM for Go to read, and a player.
/// The microphone goes through Apple's voice processing (the noise
/// suppression and echo cancelling a call uses) on the front microphone.
/// Audio session changes can take a noticeable moment, so they run on one
/// serial queue. Needs NSMicrophoneUsageDescription in the app's Info.plist.
enum Audio {
    private static let queue = DispatchQueue(label: "space.nimbusgo.audio", qos: .userInitiated)
    private static let mic = Microphone()
    private static var player: AVAudioPlayer?

    static func handle(_ call: PluginCall) {
        switch call.method {
        case "requestMicrophone":
            guard Bundle.main.object(forInfoDictionaryKey: "NSMicrophoneUsageDescription") != nil else {
                return call.reject(Plugins.unavailable, "add NSMicrophoneUsageDescription to Info.plist")
            }
            AVAudioSession.sharedInstance().requestRecordPermission { call.resolve(["granted": $0]) }
        case "micStart":
            queue.async {
                do {
                    guard AVAudioSession.sharedInstance().recordPermission == .granted else {
                        return call.reject(Plugins.denied, "the microphone is not allowed")
                    }
                    try activate(near: true)
                    try mic.start()
                    call.resolve()
                } catch {
                    call.reject(Plugins.failed, "the microphone couldn't start: \(error)")
                }
            }
        case "micRead":
            call.resolve(["pcm": mic.take().base64EncodedString()])
        case "micStop":
            queue.async {
                mic.stop()
                call.resolve()
            }
        case "play":
            guard let b64 = call.string("clip"), let clip = Data(base64Encoded: b64) else {
                return call.reject(Plugins.failed, "clip is required")
            }
            queue.async {
                do {
                    try activate(near: false)
                    let p = try AVAudioPlayer(data: clip)
                    p.isMeteringEnabled = true
                    p.prepareToPlay()
                    guard p.play() else { return call.reject(Plugins.failed, "the speaker couldn't start") }
                    DispatchQueue.main.async {
                        player?.stop()
                        player = p
                        call.resolve()
                    }
                } catch {
                    call.reject(Plugins.failed, "the clip couldn't play: \(error)")
                }
            }
        case "playing":
            guard let p = player, p.isPlaying else {
                player = nil
                return call.resolve(["playing": false, "level": 0])
            }
            p.updateMeters()
            call.resolve(["playing": true, "level": max(0, (p.averagePower(forChannel: 0) + 40) / 40)])
        case "stop":
            player?.stop()
            player = nil
            call.resolve()
        case "release":
            player?.stop()
            player = nil
            queue.async {
                mic.stop()
                try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
                call.resolve()
            }
        default:
            Plugins.unknownMethod(call)
        }
    }

    private static func activate(near: Bool) throws {
        let session = AVAudioSession.sharedInstance()
        try session.setCategory(.playAndRecord, mode: .voiceChat, options: [.defaultToSpeaker, .allowBluetooth])
        try session.setActive(true)
        if near { preferNearMicrophone(session) }
    }

    /// The built-in microphone on the screen's side (the one a face is
    /// nearest), as a cardioid where the phone offers it. Headsets are left
    /// alone: their microphone is already at the mouth.
    private static func preferNearMicrophone(_ session: AVAudioSession) {
        guard let mic = session.availableInputs?.first(where: { $0.portType == .builtInMic }) else { return }
        if let current = session.currentRoute.inputs.first, current.portType != .builtInMic { return }
        try? session.setPreferredInput(mic)
        let sources = mic.dataSources ?? []
        guard let front = sources.first(where: { $0.orientation == .front }) ?? sources.first(where: { $0.orientation == .bottom }) else { return }
        if front.supportedPolarPatterns?.contains(.cardioid) == true {
            try? front.setPreferredPolarPattern(.cardioid)
        }
        try? mic.setPreferredDataSource(front)
    }
}

/// The microphone, converted to 16 kHz mono and held until Go reads it.
private final class Microphone {
    /// Thirty seconds; older audio is dropped if Go stops reading.
    private static let limit = 960_000

    private let lock = NSLock()
    private var engine: AVAudioEngine?
    private var pcm = Data()

    func start() throws {
        lock.lock()
        defer { lock.unlock() }
        guard engine == nil else { return }
        let engine = AVAudioEngine()
        let input = engine.inputNode
        // Noise suppression and echo cancelling, but not a call's level
        // control: that lifts a voice across the room to sound like the one
        // at the phone, and loudness is how a turn's speaker is told apart.
        try? input.setVoiceProcessingEnabled(true)
        if #available(iOS 17, *) { input.isVoiceProcessingAGCEnabled = false }
        let format = input.outputFormat(forBus: 0)
        guard format.sampleRate > 0,
              let target = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true),
              let converter = AVAudioConverter(from: format, to: target) else { throw AudioError.format }
        converter.downmix = true
        pcm.removeAll()
        input.installTap(onBus: 0, bufferSize: 1024, format: format) { [weak self] buffer, _ in
            self?.convert(buffer, converter, target)
        }
        engine.prepare()
        try engine.start()
        self.engine = engine
    }

    func stop() {
        lock.lock()
        defer { lock.unlock() }
        guard let engine else { return }
        engine.inputNode.removeTap(onBus: 0)
        engine.stop()
        self.engine = nil
        pcm.removeAll()
    }

    /// What has arrived since the last call.
    func take() -> Data {
        lock.lock()
        defer { lock.unlock() }
        let out = pcm
        pcm = Data()
        return out
    }

    private func convert(_ buffer: AVAudioPCMBuffer, _ converter: AVAudioConverter, _ target: AVAudioFormat) {
        let ratio = target.sampleRate / buffer.format.sampleRate
        let capacity = AVAudioFrameCount(Double(buffer.frameLength) * ratio + 32)
        guard let out = AVAudioPCMBuffer(pcmFormat: target, frameCapacity: capacity) else { return }
        var fed = false
        var error: NSError?
        converter.convert(to: out, error: &error) { _, status in
            if fed {
                status.pointee = .noDataNow
                return nil
            }
            fed = true
            status.pointee = .haveData
            return buffer
        }
        guard error == nil, let data = out.int16ChannelData, out.frameLength > 0 else { return }
        lock.lock()
        defer { lock.unlock() }
        pcm.append(UnsafeBufferPointer(start: data[0], count: Int(out.frameLength)))
        if pcm.count > Self.limit { pcm.removeFirst(pcm.count - Self.limit) }
    }

    private enum AudioError: Error { case format }
}
