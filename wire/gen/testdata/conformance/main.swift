// Conformance host: decodes each case with the generated Kitchen types,
// re-encodes it, and writes [ok u8][len u32][bytes] per case.
import Foundation

let args = CommandLine.arguments
let input = [UInt8](FileManager.default.contents(atPath: args[1])!)
var out: [UInt8] = []

func u32(_ p: Int) -> Int { Int(input[p]) | Int(input[p + 1]) << 8 | Int(input[p + 2]) << 16 | Int(input[p + 3]) << 24 }
func put32(_ v: Int) { for i in 0..<4 { out.append(UInt8(truncatingIfNeeded: v >> (8 * i))) } }

var pos = 0
while pos < input.count {
    let n = u32(pos)
    pos += 4
    let msg = Array(input[pos..<pos + n])
    pos += n
    do {
        let enc = try Kitchen.Message.decode(msg).encode()
        out.append(1)
        put32(enc.count)
        out.append(contentsOf: enc)
    } catch {
        out.append(0)
        put32(0)
    }
}
FileManager.default.createFile(atPath: args[2], contents: Data(out))
