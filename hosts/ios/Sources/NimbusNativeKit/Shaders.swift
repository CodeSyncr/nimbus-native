import Metal
import UIKit

/// Runs canvas shaders (ui.Shader, as Metal) into pictures: the whole
/// shader over a box, at a size in pixels, for a canvas to draw inside its
/// path. Each pipeline is compiled once, from its source.
enum Shaders {
    private static let device = MTLCreateSystemDefaultDevice()
    private static let queue = device?.makeCommandQueue()
    private static var pipelines: [String: MTLRenderPipelineState?] = [:]

    private static func pipeline(_ src: String) -> MTLRenderPipelineState? {
        if let p = pipelines[src] { return p }
        var made: MTLRenderPipelineState?
        do {
            guard let device else { throw NSError(domain: "Shaders", code: 1) }
            let lib = try device.makeLibrary(source: src, options: nil)
            let d = MTLRenderPipelineDescriptor()
            d.vertexFunction = lib.makeFunction(name: "nn_vertex")
            d.fragmentFunction = lib.makeFunction(name: "nn_fragment")
            d.colorAttachments[0].pixelFormat = .rgba8Unorm
            made = try device.makeRenderPipelineState(descriptor: d)
        } catch {
            NSLog("NimbusNative shader: \(error)")
        }
        pipelines[src] = made
        return made
    }

    /// The shader run over box (x, y, w, h in the canvas's units) at w×h
    /// pixels, premultiplied; nil where it can't run.
    static func picture(_ src: String, _ uniforms: [Float], box: SIMD4<Float>, width: Int, height: Int) -> CGImage? {
        guard width > 0, height > 0, let device, let queue, let p = pipeline(src) else { return nil }
        let td = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: .rgba8Unorm, width: width, height: height, mipmapped: false)
        td.usage = [.renderTarget]
        td.storageMode = .private
        let row = width * 4
        guard let tex = device.makeTexture(descriptor: td),
              let out = device.makeBuffer(length: row * height, options: .storageModeShared),
              let cmd = queue.makeCommandBuffer() else { return nil }
        let pass = MTLRenderPassDescriptor()
        pass.colorAttachments[0].texture = tex
        pass.colorAttachments[0].loadAction = .clear
        pass.colorAttachments[0].clearColor = MTLClearColor(red: 0, green: 0, blue: 0, alpha: 0)
        pass.colorAttachments[0].storeAction = .store
        guard let enc = cmd.makeRenderCommandEncoder(descriptor: pass) else { return nil }
        enc.setRenderPipelineState(p)
        var u = uniforms.isEmpty ? [Float(0)] : uniforms
        enc.setFragmentBytes(&u, length: u.count * 4, index: 0)
        var b = box
        enc.setFragmentBytes(&b, length: 16, index: 1)
        enc.drawPrimitives(type: .triangle, vertexStart: 0, vertexCount: 3)
        enc.endEncoding()
        // The simulator keeps textures on the GPU: copy the pixels out through a buffer.
        guard let blit = cmd.makeBlitCommandEncoder() else { return nil }
        blit.copy(from: tex, sourceSlice: 0, sourceLevel: 0, sourceOrigin: MTLOrigin(x: 0, y: 0, z: 0),
                  sourceSize: MTLSize(width: width, height: height, depth: 1), to: out, destinationOffset: 0,
                  destinationBytesPerRow: row, destinationBytesPerImage: row * height)
        blit.endEncoding()
        cmd.commit()
        cmd.waitUntilCompleted()
        let data = Data(bytes: out.contents(), count: row * height)
        guard let provider = CGDataProvider(data: data as CFData) else { return nil }
        return CGImage(width: width, height: height, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: row,
                       space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue),
                       provider: provider, decode: nil, shouldInterpolate: true, intent: .defaultIntent)
    }
}
