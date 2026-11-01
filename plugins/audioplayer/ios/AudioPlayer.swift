import AVFoundation

/// audioplayer: AVPlayer, one for each sound.
enum AudioPlayerPlugin {
    private final class Sound {
        let player: AVPlayer
        var rate: Float = 1
        var loop = false
        var ended = false
        var file: URL?
        var watch: NSObjectProtocol?
        var status: NSKeyValueObservation?
        init(_ item: AVPlayerItem) { player = AVPlayer(playerItem: item) }
    }

    private static var sounds: [String: Sound] = [:]

    static func handle(_ call: PluginCall) {
        if call.method == "open" { return open(call) }
        guard let s = sounds[call.string("id") ?? ""] else { return call.reject(Plugins.notFound, "that player is closed") }
        switch call.method {
        case "play":
            try? AVAudioSession.sharedInstance().setCategory(.playback)
            try? AVAudioSession.sharedInstance().setActive(true)
            if s.ended { s.player.seek(to: .zero); s.ended = false }
            s.player.playImmediately(atRate: s.rate)
            call.resolve()
        case "pause":
            s.player.pause()
            call.resolve()
        case "seek":
            s.ended = false
            let t = CMTime(seconds: call.args["seconds"] as? Double ?? 0, preferredTimescale: 600)
            s.player.seek(to: t, toleranceBefore: .zero, toleranceAfter: .zero) { _ in call.resolve() }
        case "volume":
            s.player.volume = Float(call.args["volume"] as? Double ?? 1)
            call.resolve()
        case "rate":
            s.rate = Float(call.args["rate"] as? Double ?? 1)
            s.player.currentItem?.audioTimePitchAlgorithm = .timeDomain
            if s.player.rate != 0 { s.player.rate = s.rate }
            call.resolve()
        case "loop":
            s.loop = call.args["loop"] as? Bool ?? false
            call.resolve()
        case "status":
            let d = s.player.currentItem?.duration.seconds ?? 0
            call.resolve(["position": s.player.currentTime().seconds, "duration": d.isFinite ? d : 0,
                          "playing": s.player.rate != 0, "ended": s.ended])
        case "close":
            s.player.pause()
            if let w = s.watch { NotificationCenter.default.removeObserver(w) }
            if let f = s.file { try? FileManager.default.removeItem(at: f) }
            sounds[call.string("id") ?? ""] = nil
            call.resolve()
        default:
            Plugins.unknownMethod(call)
        }
    }

    private static func open(_ call: PluginCall) {
        var file: URL?
        var url = URL(string: call.string("url") ?? "")
        if let b = call.string("data"), let d = Data(base64Encoded: b) {
            let ext = (call.string("type") ?? "").contains("wav") ? "wav" : (call.string("type") ?? "").contains("mpeg") ? "mp3" : "m4a"
            let f = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + "." + ext)
            guard (try? d.write(to: f)) != nil else { return call.reject(Plugins.failed, "the sound couldn't be saved") }
            file = f
            url = f
        }
        guard let url, url.scheme != nil else { return call.reject(Plugins.failed, "nothing to play") }
        let item = AVPlayerItem(url: url)
        let s = Sound(item)
        s.file = file
        let id = UUID().uuidString
        sounds[id] = s
        s.watch = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: item, queue: .main) { _ in
            if s.loop {
                s.player.seek(to: .zero)
                s.player.playImmediately(atRate: s.rate)
            } else {
                s.ended = true
            }
        }
        s.status = item.observe(\.status) { i, _ in
            DispatchQueue.main.async {
                guard s.status != nil else { return }
                if i.status == .readyToPlay {
                    s.status = nil
                    call.resolve(["id": id])
                } else if i.status == .failed {
                    s.status = nil
                    sounds[id] = nil
                    call.reject(Plugins.failed, i.error?.localizedDescription ?? "it couldn't be played")
                }
            }
        }
    }
}
