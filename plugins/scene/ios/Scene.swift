import MetalKit
import QuartzCore
import UIKit
import simd

/// scene: a 3D scene Go describes (plugins/scene), drawn with Metal. Each
/// node's Spin is animated here, every frame; with nothing spinning it only
/// draws when the scene changes.
enum ScenePlugin {
    static func boot() {
        NativeViews.register("scene") { emit in SceneHost(emit) }
    }

    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private let shaders = """
#include <metal_stdlib>
using namespace metal;

struct Vert { packed_float3 p; packed_float3 n; };
struct NodeU { float4x4 mvp; float4x4 model; float4x4 normal; float4 color; float4 params; };
struct SceneU { float4 eye; float4 ambient; float4 dirs[4]; float4 colors[4]; int count; };
struct Out { float4 pos [[position]]; float3 world; float3 n; };

vertex Out vmain(uint vid [[vertex_id]], const device Vert* v [[buffer(0)]], constant NodeU& u [[buffer(1)]]) {
    Out o;
    float4 p = float4(float3(v[vid].p), 1);
    o.pos = u.mvp * p;
    o.world = (u.model * p).xyz;
    o.n = (u.normal * float4(float3(v[vid].n), 0)).xyz;
    return o;
}

fragment float4 fmain(Out in [[stage_in]], constant NodeU& u [[buffer(1)]], constant SceneU& s [[buffer(2)]]) {
    float3 base = u.color.rgb;
    if (u.params.y > 0.5) { return u.color; }
    float3 n = normalize(in.n);
    float3 v = normalize(s.eye.xyz - in.world);
    float3 c = base * s.ambient.x;
    float shine = u.params.x;
    for (int i = 0; i < s.count; i++) {
        float3 l = -s.dirs[i].xyz;
        float diff = max(dot(n, l), 0.0);
        float3 h = normalize(l + v);
        float spec = shine > 0 ? pow(max(dot(n, h), 0.0), mix(8.0, 128.0, shine)) * shine : 0.0;
        c += (base * diff + spec) * s.colors[i].rgb;
    }
    return float4(c, u.color.a);
}
"""

private struct NodeU {
    var mvp: simd_float4x4, model: simd_float4x4, normal: simd_float4x4, color: SIMD4<Float>, params: SIMD4<Float>
}

private struct SceneU {
    var eye: SIMD4<Float>, ambient: SIMD4<Float>
    var dirs: (SIMD4<Float>, SIMD4<Float>, SIMD4<Float>, SIMD4<Float>)
    var colors: (SIMD4<Float>, SIMD4<Float>, SIMD4<Float>, SIMD4<Float>)
    var count: Int32
}

private struct Node {
    var id: String, mesh: String, color: SIMD4<Float>, unlit: Bool, shine: Float
    var base: simd_float4x4, local: simd_float4x4, axis: SIMD3<Float>, speed: Float
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

private func rgba(_ c: Int) -> MTLClearColor {
    MTLClearColor(red: Double((c >> 24) & 255) / 255, green: Double((c >> 16) & 255) / 255, blue: Double((c >> 8) & 255) / 255, alpha: Double(c & 255) / 255)
}

private final class SceneHost: NSObject, NimbusNativeView, MTKViewDelegate {
    private let mtk: MTKView
    private let emit: (String, String) -> Void
    private let queue: MTLCommandQueue?
    private var pipeline: MTLRenderPipelineState?
    private var depth: MTLDepthStencilState?
    private var meshes: [String: (vertices: MTLBuffer, indices: MTLBuffer, count: Int)] = [:]
    private var nodes: [Node] = []
    private var started: [String: CFTimeInterval] = [:]
    private var camera = matrix_identity_float4x4
    private var eye = SIMD3<Float>(0, 0, 5)
    private var fov: Float = 60, near: Float = 0.1, far: Float = 100, ambient: Float = 0.25
    private var lights: [(SIMD3<Float>, SIMD3<Float>)] = []

    var view: UIView { mtk }

    init(_ emit: @escaping (String, String) -> Void) {
        self.emit = emit
        let device = MTLCreateSystemDefaultDevice()
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
        if let device {
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
            } catch {
                NSLog("scene: Metal pipeline: \(error)")
            }
            let ds = MTLDepthStencilDescriptor()
            ds.depthCompareFunction = .less
            ds.isDepthWriteEnabled = true
            depth = device.makeDepthStencilState(descriptor: ds)
        }
        mtk.addGestureRecognizer(UITapGestureRecognizer(target: self, action: #selector(tapped(_:))))
    }

    @objc private func tapped(_ g: UITapGestureRecognizer) {
        let p = g.location(in: mtk)
        let d: [String: Any] = ["X": p.x, "Y": p.y, "W": mtk.bounds.width, "H": mtk.bounds.height]
        if let b = try? JSONSerialization.data(withJSONObject: d) { emit("tap", String(decoding: b, as: UTF8.self)) }
    }

    func update(_ p: [String: Any]) {
        mtk.clearColor = rgba((p["background"] as? NSNumber)?.intValue ?? 0)
        camera = matrix(p["view"])
        eye = vec3(p["eye"])
        fov = (p["fov"] as? NSNumber)?.floatValue ?? 60
        near = (p["near"] as? NSNumber)?.floatValue ?? 0.1
        far = (p["far"] as? NSNumber)?.floatValue ?? 100
        ambient = (p["ambient"] as? NSNumber)?.floatValue ?? 0.25
        lights = (p["lights"] as? [[String: Any]] ?? []).prefix(4).map { l in
            let c = floats(l["color"])
            return (vec3(l["direction"]), c.count == 3 ? SIMD3(c[0], c[1], c[2]) : SIMD3(1, 1, 1))
        }
        let wanted = p["meshes"] as? [String: [String: Any]] ?? [:]
        meshes = meshes.filter { wanted[$0.key] != nil }
        if let device = mtk.device {
            for (key, m) in wanted where meshes[key] == nil {
                guard let v = Data(base64Encoded: m["vertices"] as? String ?? ""), let i = Data(base64Encoded: m["indices"] as? String ?? ""),
                      !v.isEmpty, !i.isEmpty,
                      let vb = v.withUnsafeBytes({ device.makeBuffer(bytes: $0.baseAddress!, length: v.count) }),
                      let ib = i.withUnsafeBytes({ device.makeBuffer(bytes: $0.baseAddress!, length: i.count) }) else { continue }
                meshes[key] = (vb, ib, (m["count"] as? NSNumber)?.intValue ?? i.count / 4)
            }
        }
        let now = CACurrentMediaTime()
        nodes = (p["nodes"] as? [[String: Any]] ?? []).map { n in
            let c = floats(n["color"])
            return Node(id: n["id"] as? String ?? "", mesh: n["mesh"] as? String ?? "",
                        color: c.count == 4 ? SIMD4(c[0], c[1], c[2], c[3]) : SIMD4(1, 1, 1, 1),
                        unlit: n["unlit"] as? Bool ?? false, shine: (n["shine"] as? NSNumber)?.floatValue ?? 0,
                        base: matrix(n["base"]), local: matrix(n["local"]), axis: vec3(n["axis"]),
                        speed: (n["speed"] as? NSNumber)?.floatValue ?? 0)
        }
        // A node keeps turning from where it was across updates.
        let ids = Set(nodes.map(\.id))
        started = started.filter { ids.contains($0.key) }
        for n in nodes where n.speed != 0 && started[n.id] == nil { started[n.id] = now }
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

    func draw(in v: MTKView) {
        guard let pipeline, let depth, let queue, let pass = v.currentRenderPassDescriptor, let drawable = v.currentDrawable,
              let cmd = queue.makeCommandBuffer(), let enc = cmd.makeRenderCommandEncoder(descriptor: pass) else { return }
        let size = v.drawableSize
        let vp = projection(Float(size.width / max(size.height, 1))) * camera
        var s = SceneU(eye: SIMD4(eye, 1), ambient: SIMD4(ambient, 0, 0, 0), dirs: (.zero, .zero, .zero, .zero), colors: (.zero, .zero, .zero, .zero), count: Int32(lights.count))
        withUnsafeMutableBytes(of: &s.dirs) { b in for (i, l) in lights.enumerated() { b.storeBytes(of: SIMD4(l.0, 0), toByteOffset: i * 16, as: SIMD4<Float>.self) } }
        withUnsafeMutableBytes(of: &s.colors) { b in for (i, l) in lights.enumerated() { b.storeBytes(of: SIMD4(l.1, 0), toByteOffset: i * 16, as: SIMD4<Float>.self) } }
        enc.setRenderPipelineState(pipeline)
        enc.setDepthStencilState(depth)
        enc.setCullMode(.back)
        enc.setFrontFacing(.counterClockwise)
        enc.setFragmentBytes(&s, length: MemoryLayout<SceneU>.stride, index: 2)
        let now = CACurrentMediaTime()
        for n in nodes {
            guard let m = meshes[n.mesh] else { continue }
            var spin = matrix_identity_float4x4
            if n.speed != 0, let t0 = started[n.id], simd_length(n.axis) > 0 {
                let a = Float(now - t0) * n.speed * .pi / 180
                spin = simd_float4x4(simd_quatf(angle: a, axis: simd_normalize(n.axis)))
            }
            let model = n.base * spin * n.local
            var u = NodeU(mvp: vp * model, model: model, normal: model.inverse.transpose, color: n.color,
                          params: SIMD4(n.shine, n.unlit ? 1 : 0, 0, 0))
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
