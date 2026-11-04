import MetalKit
import QuartzCore
import UIKit
import simd

/// scene: a 3D scene Go describes (plugins/scene), drawn with Metal. Each
/// frame: if shadows are on, a depth-only pass from the first light; then
/// the scene, lit, textured, and shadowed by sampling that map around each
/// point. Spin turns here every frame; with nothing spinning it draws only
/// when something changes.
enum ScenePlugin {
    static func boot() {
        NativeViews.register("scene") { emit in SceneHost(emit) }
    }

    static func handle(_ call: PluginCall) {
        guard let key = call.string("key") else { return call.reject(Plugins.failed, "no key") }
        switch call.method {
        case "mesh":
            guard let v = Data(base64Encoded: call.string("vertices") ?? ""), let i = Data(base64Encoded: call.string("indices") ?? ""),
                  v.count >= 32, i.count >= 4 else { return call.reject(Plugins.failed, "an empty mesh") }
            SceneStore.shared.addMesh(key, v, i)
        case "texture":
            guard let d = Data(base64Encoded: call.string("data") ?? "") else { return call.reject(Plugins.failed, "no picture") }
            SceneStore.shared.addTexture(key, d)
        default:
            return Plugins.unknownMethod(call)
        }
        call.resolve()
    }
}

/// What Go has sent, by key, on the GPU, for every scene on screen. Main thread.
final class SceneStore {
    static let shared = SceneStore()
    static let arrived = Notification.Name("nimbus.scene.asset")
    let device = MTLCreateSystemDefaultDevice()
    private(set) var meshes: [String: (vertices: MTLBuffer, indices: MTLBuffer, count: Int)] = [:]
    private(set) var textures: [String: MTLTexture] = [:]

    func addMesh(_ key: String, _ v: Data, _ i: Data) {
        guard let device,
              let vb = v.withUnsafeBytes({ device.makeBuffer(bytes: $0.baseAddress!, length: v.count) }),
              let ib = i.withUnsafeBytes({ device.makeBuffer(bytes: $0.baseAddress!, length: i.count) }) else { return }
        meshes[key] = (vb, ib, i.count / 4)
        NotificationCenter.default.post(name: Self.arrived, object: nil)
    }

    func addTexture(_ key: String, _ data: Data) {
        guard let device else { return }
        let loader = MTKTextureLoader(device: device)
        let options: [MTKTextureLoader.Option: Any] = [.generateMipmaps: true, .SRGB: false, .origin: MTKTextureLoader.Origin.topLeft]
        guard let t = try? loader.newTexture(data: data, options: options) else { return }
        textures[key] = t
        NotificationCenter.default.post(name: Self.arrived, object: nil)
    }
}

private let shaders = """
#include <metal_stdlib>
using namespace metal;

struct Vert { packed_float3 p; packed_float3 n; packed_float2 uv; };
struct NodeU { float4x4 model; float4x4 normal; float4 color; float4 params; };
struct SceneU { float4x4 vp; float4x4 lightVP; float4 eye; float4 ambient; float4 dirs[4]; float4 colors[4]; int4 count; };
struct Out { float4 pos [[position]]; float3 world; float3 n; float2 uv; float4 lp; };

vertex Out vmain(uint vid [[vertex_id]], const device Vert* v [[buffer(0)]], constant NodeU& u [[buffer(1)]], constant SceneU& s [[buffer(2)]]) {
    Out o;
    float4 w = u.model * float4(float3(v[vid].p), 1);
    o.pos = s.vp * w;
    o.world = w.xyz;
    o.n = (u.normal * float4(float3(v[vid].n), 0)).xyz;
    o.uv = float2(v[vid].uv);
    o.lp = s.lightVP * w;
    return o;
}

vertex float4 shadowv(uint vid [[vertex_id]], const device Vert* v [[buffer(0)]], constant NodeU& u [[buffer(1)]], constant SceneU& s [[buffer(2)]]) {
    return s.lightVP * u.model * float4(float3(v[vid].p), 1);
}

float lit(Out in, float3 n, float3 l, constant SceneU& s, depth2d<float> map, sampler cmp) {
    if (s.ambient.y < 0.5) return 1.0;
    float3 p = in.lp.xyz / in.lp.w;
    float2 at = float2(p.x * 0.5 + 0.5, 0.5 - p.y * 0.5); // Metal's textures run down
    if (at.x < 0 || at.x > 1 || at.y < 0 || at.y > 1 || p.z > 1) return 1.0;
    float bias = max(0.004 * (1.0 - dot(n, l)), 0.001);
    float2 texel = 1.0 / float2(map.get_width(), map.get_height());
    float sum = 0;
    for (int x = -1; x <= 1; x++)
        for (int y = -1; y <= 1; y++)
            sum += map.sample_compare(cmp, at + float2(x, y) * texel, p.z - bias);
    return sum / 9.0;
}

fragment float4 fmain(Out in [[stage_in]], bool front [[front_facing]], constant NodeU& u [[buffer(1)]], constant SceneU& s [[buffer(2)]],
                      texture2d<float> tex [[texture(0)]], sampler smp [[sampler(0)]],
                      depth2d<float> map [[texture(1)]], sampler cmp [[sampler(1)]]) {
    float4 base = tex.sample(smp, in.uv) * u.color;
    if (base.a < 0.02) discard_fragment();
    if (u.params.y > 0.5) return base;
    float3 n = normalize(in.n);
    if (!front) n = -n; // a double-sided node's back
    float3 v = normalize(s.eye.xyz - in.world);
    float3 c = base.rgb * s.ambient.x;
    float shine = u.params.x;
    for (int i = 0; i < s.count.x; i++) {
        float3 l = -s.dirs[i].xyz;
        float diff = max(dot(n, l), 0.0);
        float3 h = normalize(l + v);
        float spec = shine > 0 ? pow(max(dot(n, h), 0.0), mix(8.0, 128.0, shine)) * shine : 0.0;
        float shade = (i == 0 && diff > 0) ? lit(in, n, l, s, map, cmp) : 1.0;
        c += (base.rgb * diff + spec) * s.colors[i].rgb * shade;
    }
    return float4(c, base.a);
}
"""

private struct NodeU {
    var model: simd_float4x4, normal: simd_float4x4, color: SIMD4<Float>, params: SIMD4<Float>
}

private struct SceneU {
    var vp: simd_float4x4, lightVP: simd_float4x4
    var eye: SIMD4<Float>, ambient: SIMD4<Float>
    var dirs: (SIMD4<Float>, SIMD4<Float>, SIMD4<Float>, SIMD4<Float>)
    var colors: (SIMD4<Float>, SIMD4<Float>, SIMD4<Float>, SIMD4<Float>)
    var count: SIMD4<Int32>
}

private struct Node {
    var id: String, mesh: String, texture: String, color: SIMD4<Float>, unlit: Bool, shine: Float, double: Bool, cast: Bool
    var base: simd_float4x4, local: simd_float4x4, axis: SIMD3<Float>, speed: Float
    var parent: Int // 1 + the index of the spinning node it turns with, or 0
}

private func floats(_ o: Any?) -> [Float] { (o as? [Any] ?? []).map { ($0 as? NSNumber)?.floatValue ?? 0 } }

private func matrix(_ o: Any?) -> simd_float4x4 {
    let f = floats(o)
    guard f.count == 16 else { return matrix_identity_float4x4 }
    return simd_float4x4(SIMD4(f[0], f[1], f[2], f[3]), SIMD4(f[4], f[5], f[6], f[7]), SIMD4(f[8], f[9], f[10], f[11]), SIMD4(f[12], f[13], f[14], f[15]))
}

private func vec3(_ o: Any?) -> SIMD3<Float> {
    let d = o as? [String: Any] ?? [:]
    return SIMD3(Float(truncating: d["X"] as? NSNumber ?? 0), Float(truncating: d["Y"] as? NSNumber ?? 0), Float(truncating: d["Z"] as? NSNumber ?? 0))
}

private func json(_ v: SIMD3<Float>) -> [String: Any] { ["X": v.x, "Y": v.y, "Z": v.z] }

private func lookAt(_ eye: SIMD3<Float>, _ target: SIMD3<Float>) -> simd_float4x4 {
    var f = target - eye
    f = simd_length(f) > 0 ? simd_normalize(f) : SIMD3(0, 0, -1)
    let up: SIMD3<Float> = abs(f.y) > 0.999 ? SIMD3(0, 0, -1) : SIMD3(0, 1, 0)
    let s = simd_normalize(simd_cross(f, up)), u = simd_cross(s, f)
    return simd_float4x4(SIMD4(s.x, u.x, -f.x, 0), SIMD4(s.y, u.y, -f.y, 0), SIMD4(s.z, u.z, -f.z, 0),
                         SIMD4(-simd_dot(s, eye), -simd_dot(u, eye), simd_dot(f, eye), 1))
}

private final class SceneHost: NSObject, NimbusNativeView, MTKViewDelegate {
    private static let shadowSize = 2048
    private let mtk: MTKView
    private let emit: (String, String) -> Void
    private let queue: MTLCommandQueue?
    private var pipeline: MTLRenderPipelineState?, shadowPipeline: MTLRenderPipelineState?
    private var depth: MTLDepthStencilState?
    private var sampler: MTLSamplerState?, compare: MTLSamplerState?
    private var white: MTLTexture?, noShadow: MTLTexture?, shadowMap: MTLTexture?
    private var nodes: [Node] = []
    private var started: [String: CFTimeInterval] = [:]
    private var asked = Set<String>()

    // The scene, and where Orbit has taken the camera.
    private var goCamera = ""
    private var eye = SIMD3<Float>(0, 0, 5), target = SIMD3<Float>(0, 0, 0)
    private var orbit = false, yaw: Float = 0, pitch: Float = 0, dist: Float = 5
    private var fov: Float = 60, near: Float = 0.1, far: Float = 100, ambient: Float = 0.25
    private var shadows = false, lightVP = matrix_identity_float4x4
    private var lights: [(SIMD3<Float>, SIMD3<Float>)] = []

    var view: UIView { mtk }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        let device = SceneStore.shared.device
        mtk = MTKView(frame: .zero, device: device)
        queue = device?.makeCommandQueue()
        super.init()
        mtk.colorPixelFormat = .bgra8Unorm
        mtk.depthStencilPixelFormat = .depth32Float
        mtk.sampleCount = 4
        mtk.isOpaque = false
        mtk.backgroundColor = .clear
        mtk.delegate = self
        mtk.isPaused = true
        mtk.enableSetNeedsDisplay = true
        if let device { setUp(device) }
        mtk.addGestureRecognizer(UITapGestureRecognizer(target: self, action: #selector(tapped(_:))))
        let pan = UIPanGestureRecognizer(target: self, action: #selector(panned(_:)))
        pan.maximumNumberOfTouches = 1
        mtk.addGestureRecognizer(pan)
        mtk.addGestureRecognizer(UIPinchGestureRecognizer(target: self, action: #selector(pinched(_:))))
        NotificationCenter.default.addObserver(self, selector: #selector(arrived), name: SceneStore.arrived, object: nil)
    }

    deinit { NotificationCenter.default.removeObserver(self) }

    private func setUp(_ device: MTLDevice) {
        do {
            let lib = try device.makeLibrary(source: shaders, options: nil)
            let d = MTLRenderPipelineDescriptor()
            d.vertexFunction = lib.makeFunction(name: "vmain")
            d.fragmentFunction = lib.makeFunction(name: "fmain")
            d.colorAttachments[0].pixelFormat = mtk.colorPixelFormat
            d.colorAttachments[0].isBlendingEnabled = true
            d.colorAttachments[0].sourceRGBBlendFactor = .sourceAlpha
            d.colorAttachments[0].destinationRGBBlendFactor = .oneMinusSourceAlpha
            d.colorAttachments[0].sourceAlphaBlendFactor = .one
            d.colorAttachments[0].destinationAlphaBlendFactor = .oneMinusSourceAlpha
            d.depthAttachmentPixelFormat = mtk.depthStencilPixelFormat
            d.rasterSampleCount = mtk.sampleCount
            pipeline = try device.makeRenderPipelineState(descriptor: d)
            let s = MTLRenderPipelineDescriptor()
            s.vertexFunction = lib.makeFunction(name: "shadowv")
            s.depthAttachmentPixelFormat = .depth32Float
            shadowPipeline = try device.makeRenderPipelineState(descriptor: s)
        } catch {
            NSLog("scene: Metal pipeline: \(error)")
        }
        let ds = MTLDepthStencilDescriptor()
        ds.depthCompareFunction = .lessEqual
        ds.isDepthWriteEnabled = true
        depth = device.makeDepthStencilState(descriptor: ds)
        let sd = MTLSamplerDescriptor()
        sd.minFilter = .linear; sd.magFilter = .linear; sd.mipFilter = .linear
        sd.sAddressMode = .repeat; sd.tAddressMode = .repeat
        sampler = device.makeSamplerState(descriptor: sd)
        let cd = MTLSamplerDescriptor()
        cd.minFilter = .nearest; cd.magFilter = .nearest
        cd.sAddressMode = .clampToEdge; cd.tAddressMode = .clampToEdge
        cd.compareFunction = .lessEqual
        compare = device.makeSamplerState(descriptor: cd)
        let wd = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: .rgba8Unorm, width: 1, height: 1, mipmapped: false)
        white = device.makeTexture(descriptor: wd)
        var px: UInt32 = 0xffffffff
        white?.replace(region: MTLRegionMake2D(0, 0, 1, 1), mipmapLevel: 0, withBytes: &px, bytesPerRow: 4)
        let nd = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: .depth32Float, width: 1, height: 1, mipmapped: false)
        nd.usage = [.shaderRead, .renderTarget]
        nd.storageMode = .private
        noShadow = device.makeTexture(descriptor: nd)
    }

    // MARK: Gestures

    @objc private func tapped(_ g: UITapGestureRecognizer) {
        let p = g.location(in: mtk)
        let d: [String: Any] = ["X": p.x, "Y": p.y, "W": mtk.bounds.width, "H": mtk.bounds.height, "Eye": json(eye), "Target": json(target)]
        if let b = try? JSONSerialization.data(withJSONObject: d) { emit("tap", String(decoding: b, as: UTF8.self)) }
    }

    @objc private func panned(_ g: UIPanGestureRecognizer) {
        guard orbit else { return }
        let t = g.translation(in: mtk)
        g.setTranslation(.zero, in: mtk)
        yaw -= Float(t.x) * 0.4
        pitch = min(85, max(-85, pitch + Float(t.y) * 0.4))
        moved()
    }

    @objc private func pinched(_ g: UIPinchGestureRecognizer) {
        guard orbit, g.scale > 0 else { return }
        dist = min(far / 2, max(near * 4, dist / Float(g.scale)))
        g.scale = 1
        moved()
    }

    private func moved() {
        let y = yaw * .pi / 180, p = pitch * .pi / 180
        eye = target + SIMD3(dist * cos(p) * sin(y), dist * sin(p), dist * cos(p) * cos(y))
        mtk.setNeedsDisplay()
    }

    // MARK: The scene

    @objc private func arrived() { use() }

    private var props: [String: Any] = [:]

    func update(_ p: [String: Any]) {
        props = p
        asked.removeAll()
        orbit = p["orbit"] as? Bool ?? false
        fov = (p["fov"] as? NSNumber)?.floatValue ?? 60
        near = (p["near"] as? NSNumber)?.floatValue ?? 0.1
        far = (p["far"] as? NSNumber)?.floatValue ?? 100
        ambient = (p["ambient"] as? NSNumber)?.floatValue ?? 0.25
        let c = (p["background"] as? NSNumber)?.intValue ?? 0
        mtk.clearColor = MTLClearColor(red: Double((c >> 24) & 255) / 255, green: Double((c >> 16) & 255) / 255,
                                       blue: Double((c >> 8) & 255) / 255, alpha: Double(c & 255) / 255)
        // Go's camera when it changes; Orbit's otherwise.
        let key = "\(p["eye"] ?? "")\(p["target"] ?? "")"
        if key != goCamera {
            goCamera = key
            eye = vec3(p["eye"]); target = vec3(p["target"])
            let d = eye - target
            dist = max(simd_length(d), 1e-3)
            yaw = atan2(d.x, d.z) * 180 / .pi
            pitch = asin(min(1, max(-1, d.y / dist))) * 180 / .pi
        }
        lights = (p["lights"] as? [[String: Any]] ?? []).prefix(4).map { l in
            let c = floats(l["color"])
            return (vec3(l["direction"]), c.count == 3 ? SIMD3(c[0], c[1], c[2]) : SIMD3(1, 1, 1))
        }
        let shadow = p["shadow"] as? [String: Any]
        shadows = shadow != nil
        if let shadow { lightVP = matrix(shadow["vp"]) }
        if shadows, shadowMap == nil, let device = SceneStore.shared.device {
            let d = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: .depth32Float, width: Self.shadowSize, height: Self.shadowSize, mipmapped: false)
            d.usage = [.shaderRead, .renderTarget]
            d.storageMode = .private
            shadowMap = device.makeTexture(descriptor: d)
        }
        let now = CACurrentMediaTime()
        nodes = (p["nodes"] as? [[String: Any]] ?? []).map { n in
            let c = floats(n["color"])
            return Node(id: n["id"] as? String ?? "", mesh: n["mesh"] as? String ?? "", texture: n["texture"] as? String ?? "",
                        color: c.count == 4 ? SIMD4(c[0], c[1], c[2], c[3]) : SIMD4(1, 1, 1, 1),
                        unlit: n["unlit"] as? Bool ?? false, shine: (n["shine"] as? NSNumber)?.floatValue ?? 0,
                        double: n["double"] as? Bool ?? false, cast: n["cast"] as? Bool ?? false,
                        base: matrix(n["base"]), local: matrix(n["local"]), axis: vec3(n["axis"]),
                        speed: (n["speed"] as? NSNumber)?.floatValue ?? 0, parent: (n["parent"] as? NSNumber)?.intValue ?? 0)
        }
        // A node keeps turning from where it was across updates.
        let ids = Set(nodes.map(\.id))
        started = started.filter { ids.contains($0.key) }
        for n in nodes where n.speed != 0 && started[n.id] == nil { started[n.id] = now }
        use()
    }

    /// Asks Go for what nobody has sent yet, and draws.
    private func use() {
        let store = SceneStore.shared
        var missing: [String] = []
        for n in nodes {
            if !n.mesh.isEmpty, store.meshes[n.mesh] == nil, asked.insert(n.mesh).inserted { missing.append(n.mesh) }
            if !n.texture.isEmpty, store.textures[n.texture] == nil, asked.insert(n.texture).inserted { missing.append(n.texture) }
        }
        if !missing.isEmpty, let b = try? JSONSerialization.data(withJSONObject: ["keys": missing]) {
            emit("need", String(decoding: b, as: UTF8.self))
        }
        let moving = nodes.contains { $0.speed != 0 }
        mtk.enableSetNeedsDisplay = !moving
        mtk.isPaused = !moving
        mtk.setNeedsDisplay()
    }

    func mtkView(_ v: MTKView, drawableSizeWillChange size: CGSize) { v.setNeedsDisplay() }

    private func projection(_ aspect: Float) -> simd_float4x4 {
        let ys = 1 / tan(fov * .pi / 360)
        let xs = ys / aspect
        let zs = far / (near - far)
        return simd_float4x4(SIMD4(xs, 0, 0, 0), SIMD4(0, ys, 0, 0), SIMD4(0, 0, zs, -1), SIMD4(0, 0, zs * near, 0))
    }

    /// Where each node is now: parents come first, so a node turning with a
    /// spinning one starts from where that one has turned to.
    private func models(_ now: CFTimeInterval) -> [simd_float4x4] {
        var out: [simd_float4x4] = []
        out.reserveCapacity(nodes.count)
        for n in nodes {
            var spin = matrix_identity_float4x4
            if n.speed != 0, let t0 = started[n.id], simd_length(n.axis) > 0 {
                spin = simd_float4x4(simd_quatf(angle: Float(now - t0) * n.speed * .pi / 180, axis: simd_normalize(n.axis)))
            }
            let parent = n.parent > 0 && n.parent <= out.count ? out[n.parent - 1] : matrix_identity_float4x4
            out.append(parent * n.base * spin * n.local)
        }
        return out
    }

    func draw(in v: MTKView) {
        guard let pipeline, let shadowPipeline, let depth, let queue, let sampler, let compare, let white, let noShadow,
              let pass = v.currentRenderPassDescriptor, let drawable = v.currentDrawable, let cmd = queue.makeCommandBuffer() else { return }
        let store = SceneStore.shared
        let size = v.drawableSize
        var s = SceneU(vp: projection(Float(size.width / max(size.height, 1))) * lookAt(eye, target), lightVP: lightVP,
                       eye: SIMD4(eye, 1), ambient: SIMD4(ambient, shadows ? 1 : 0, 0, 0),
                       dirs: (.zero, .zero, .zero, .zero), colors: (.zero, .zero, .zero, .zero), count: SIMD4(Int32(lights.count), 0, 0, 0))
        withUnsafeMutableBytes(of: &s.dirs) { b in for (i, l) in lights.enumerated() { b.storeBytes(of: SIMD4(l.0, 0), toByteOffset: i * 16, as: SIMD4<Float>.self) } }
        withUnsafeMutableBytes(of: &s.colors) { b in for (i, l) in lights.enumerated() { b.storeBytes(of: SIMD4(l.1, 0), toByteOffset: i * 16, as: SIMD4<Float>.self) } }
        let placed = models(CACurrentMediaTime())

        var map = noShadow
        if shadows, let shadowMap {
            let sp = MTLRenderPassDescriptor()
            sp.depthAttachment.texture = shadowMap
            sp.depthAttachment.loadAction = .clear
            sp.depthAttachment.storeAction = .store
            sp.depthAttachment.clearDepth = 1
            if let enc = cmd.makeRenderCommandEncoder(descriptor: sp) {
                enc.setRenderPipelineState(shadowPipeline)
                enc.setDepthStencilState(depth)
                enc.setCullMode(.none)
                enc.setDepthBias(0.0005, slopeScale: 1.75, clamp: 0.01)
                enc.setVertexBytes(&s, length: MemoryLayout<SceneU>.stride, index: 2)
                for (i, n) in nodes.enumerated() where n.cast {
                    guard let m = store.meshes[n.mesh] else { continue }
                    var u = NodeU(model: placed[i], normal: matrix_identity_float4x4, color: .zero, params: .zero)
                    enc.setVertexBuffer(m.vertices, offset: 0, index: 0)
                    enc.setVertexBytes(&u, length: MemoryLayout<NodeU>.stride, index: 1)
                    enc.drawIndexedPrimitives(type: .triangle, indexCount: m.count, indexType: .uint32, indexBuffer: m.indices, indexBufferOffset: 0)
                }
                enc.endEncoding()
                map = shadowMap
            }
        }

        guard let enc = cmd.makeRenderCommandEncoder(descriptor: pass) else { return }
        enc.setRenderPipelineState(pipeline)
        enc.setDepthStencilState(depth)
        enc.setFrontFacing(.counterClockwise)
        enc.setVertexBytes(&s, length: MemoryLayout<SceneU>.stride, index: 2)
        enc.setFragmentBytes(&s, length: MemoryLayout<SceneU>.stride, index: 2)
        enc.setFragmentSamplerState(sampler, index: 0)
        enc.setFragmentSamplerState(compare, index: 1)
        enc.setFragmentTexture(map, index: 1)
        for (i, n) in nodes.enumerated() {
            guard let m = store.meshes[n.mesh] else { continue }
            let mm = placed[i]
            var u = NodeU(model: mm, normal: mm.inverse.transpose, color: n.color, params: SIMD4(n.shine, n.unlit ? 1 : 0, 0, 0))
            enc.setCullMode(n.double ? .none : .back)
            enc.setFragmentTexture(n.texture.isEmpty ? white : (store.textures[n.texture] ?? white), index: 0)
            enc.setVertexBuffer(m.vertices, offset: 0, index: 0)
            enc.setVertexBytes(&u, length: MemoryLayout<NodeU>.stride, index: 1)
            enc.setFragmentBytes(&u, length: MemoryLayout<NodeU>.stride, index: 1)
            enc.drawIndexedPrimitives(type: .triangle, indexCount: m.count, indexType: .uint32, indexBuffer: m.indices, indexBufferOffset: 0)
        }
        enc.endEncoding()
        cmd.present(drawable)
        cmd.commit()
    }
}
