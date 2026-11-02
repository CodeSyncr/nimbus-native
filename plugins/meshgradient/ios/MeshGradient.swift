import MetalKit
import UIKit

/// meshgradient: the grid Go cut, drawn by Metal as triangles that blend
/// their corners' colors. It draws only when the gradient changes.
enum MeshGradientPlugin {
    static func boot() {
        NativeViews.register("meshgradient") { _ in MeshGradientHost() }
    }

    static func handle(_ call: PluginCall) { Plugins.unknownMethod(call) }
}

private let meshShaders = """
#include <metal_stdlib>
using namespace metal;
struct Out { float4 pos [[position]]; float4 color; };
vertex Out vmain(uint vid [[vertex_id]], const device float2* p [[buffer(0)]], const device float4* c [[buffer(1)]]) {
    Out o;
    o.pos = float4(p[vid].x * 2 - 1, 1 - p[vid].y * 2, 0, 1);
    o.color = c[vid];
    return o;
}
fragment float4 fmain(Out in [[stage_in]]) { return float4(in.color.rgb * in.color.a, in.color.a); }
"""

private final class MeshGradientHost: NSObject, NimbusNativeView, MTKViewDelegate {
    private let mtk = MTKView(frame: .zero, device: MTLCreateSystemDefaultDevice())
    private let queue: MTLCommandQueue?
    private var pipeline: MTLRenderPipelineState?
    private var positions: MTLBuffer?, colors: MTLBuffer?, indices: MTLBuffer?
    private var count = 0

    var view: UIView { mtk }

    override init() {
        queue = mtk.device?.makeCommandQueue()
        super.init()
        mtk.colorPixelFormat = .bgra8Unorm
        mtk.sampleCount = 4
        mtk.isOpaque = false
        mtk.backgroundColor = .clear
        mtk.clearColor = MTLClearColor(red: 0, green: 0, blue: 0, alpha: 0)
        mtk.isPaused = true
        mtk.enableSetNeedsDisplay = true
        mtk.delegate = self
        if let device = mtk.device, let lib = try? device.makeLibrary(source: meshShaders, options: nil) {
            let d = MTLRenderPipelineDescriptor()
            d.vertexFunction = lib.makeFunction(name: "vmain")
            d.fragmentFunction = lib.makeFunction(name: "fmain")
            d.colorAttachments[0].pixelFormat = mtk.colorPixelFormat
            d.colorAttachments[0].isBlendingEnabled = true
            d.colorAttachments[0].sourceRGBBlendFactor = .one // premultiplied in the shader
            d.colorAttachments[0].destinationRGBBlendFactor = .oneMinusSourceAlpha
            d.colorAttachments[0].sourceAlphaBlendFactor = .one
            d.colorAttachments[0].destinationAlphaBlendFactor = .oneMinusSourceAlpha
            d.rasterSampleCount = mtk.sampleCount
            pipeline = try? device.makeRenderPipelineState(descriptor: d)
        }
    }

    func update(_ p: [String: Any]) {
        guard let device = mtk.device,
              let pos = Data(base64Encoded: p["positions"] as? String ?? ""), let col = Data(base64Encoded: p["colors"] as? String ?? "") else { return }
        let w = (p["w"] as? NSNumber)?.intValue ?? 0, h = (p["h"] as? NSNumber)?.intValue ?? 0
        guard w > 0, h > 0, pos.count == (w + 1) * (h + 1) * 8, col.count == pos.count * 2 else { return }
        var ix: [UInt32] = []
        ix.reserveCapacity(w * h * 6)
        for y in 0..<h {
            for x in 0..<w {
                let a = UInt32(y * (w + 1) + x), b = a + 1, d = a + UInt32(w + 1), e = d + 1
                ix += [a, d, b, b, d, e]
            }
        }
        positions = pos.withUnsafeBytes { device.makeBuffer(bytes: $0.baseAddress!, length: pos.count) }
        colors = col.withUnsafeBytes { device.makeBuffer(bytes: $0.baseAddress!, length: col.count) }
        indices = device.makeBuffer(bytes: ix, length: ix.count * 4)
        count = ix.count
        mtk.setNeedsDisplay()
    }

    func mtkView(_ v: MTKView, drawableSizeWillChange size: CGSize) { v.setNeedsDisplay() }

    func draw(in v: MTKView) {
        guard let pipeline, let queue, let positions, let colors, let indices, count > 0,
              let pass = v.currentRenderPassDescriptor, let drawable = v.currentDrawable,
              let cmd = queue.makeCommandBuffer(), let enc = cmd.makeRenderCommandEncoder(descriptor: pass) else { return }
        enc.setRenderPipelineState(pipeline)
        enc.setVertexBuffer(positions, offset: 0, index: 0)
        enc.setVertexBuffer(colors, offset: 0, index: 1)
        enc.drawIndexedPrimitives(type: .triangle, indexCount: count, indexType: .uint32, indexBuffer: indices, indexBufferOffset: 0)
        enc.endEncoding()
        cmd.present(drawable)
        cmd.commit()
    }
}
