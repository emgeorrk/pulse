// svg2png renders an SVG into a transparent PNG of the given pixel size
// using NSImage's native SVG support (macOS 11+). Used by gen-icons.sh as
// the fallback when rsvg-convert is not installed; qlmanage is unsuitable
// because it composites thumbnails onto a white background.
import AppKit

let args = CommandLine.arguments
guard args.count == 4, let size = Int(args[3]) else {
    FileHandle.standardError.write("usage: svg2png <in.svg> <out.png> <size>\n".data(using: .utf8)!)
    exit(2)
}
let data = try Data(contentsOf: URL(fileURLWithPath: args[1]))
guard let img = NSImage(data: data) else {
    FileHandle.standardError.write("cannot decode \(args[1])\n".data(using: .utf8)!)
    exit(1)
}
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
                           bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                           colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSGraphicsContext.current?.imageInterpolation = .high
img.draw(in: NSRect(x: 0, y: 0, width: size, height: size),
         from: .zero, operation: .sourceOver, fraction: 1.0)
NSGraphicsContext.restoreGraphicsState()
try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: args[2]))
