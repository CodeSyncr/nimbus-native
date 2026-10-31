import AVKit
import UIKit

/// video: the system's player as a native view (ui.Native).
enum VideoPlugin {
    static func boot() { NativeViews.register("video") { emit in VideoHost(emit) } }
    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private final class VideoHost: NSObject, NimbusNativeView {
    private let controller = AVPlayerViewController()
    private let player = AVPlayer()
    private let emit: (String, String) -> Void
    private var url = ""
    private var loop = false
    private var status: NSKeyValueObservation?

    var view: UIView { controller.view }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        super.init()
        controller.player = player
        controller.view.backgroundColor = .black
        NotificationCenter.default.addObserver(self, selector: #selector(ended(_:)), name: .AVPlayerItemDidPlayToEndTime, object: nil)
    }

    deinit { NotificationCenter.default.removeObserver(self) }

    @objc private func ended(_ n: Notification) {
        guard (n.object as? AVPlayerItem) === player.currentItem else { return }
        if loop {
            player.seek(to: .zero)
            player.play()
        } else {
            emit("end", "{}")
        }
    }

    func update(_ p: [String: Any]) {
        controller.showsPlaybackControls = p["controls"] as? Bool ?? false
        player.isMuted = p["muted"] as? Bool ?? false
        loop = p["loop"] as? Bool ?? false
        if let s = p["url"] as? String, s != url, let u = URL(string: s) {
            url = s
            let item = AVPlayerItem(url: u)
            status = item.observe(\.status) { [weak self] i, _ in
                guard i.status == .readyToPlay else { return }
                DispatchQueue.main.async { self?.emit("ready", "{\"seconds\":\(i.duration.seconds.isFinite ? i.duration.seconds : 0)}") }
            }
            player.replaceCurrentItem(with: item)
        }
        if p["paused"] as? Bool ?? false { player.pause() } else { player.play() }
    }
}
