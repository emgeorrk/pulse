// sfsymbol2png renders an SF Symbol into a transparent PNG of the given
// pixel size (fitted, centered). Used by gen-icons.sh for glyphs the Vitals
// set lacks (the Settings gear).
import AppKit

let args = CommandLine.arguments
guard args.count == 4, let size = Int(args[3]) else {
    FileHandle.standardError.write("usage: sfsymbol2png <symbol-name> <out.png> <size>\n".data(using: .utf8)!)
    exit(2)
}
guard let base = NSImage(systemSymbolName: args[1], accessibilityDescription: nil),
      let img = base.withSymbolConfiguration(.init(pointSize: CGFloat(size), weight: .regular)) else {
    FileHandle.standardError.write("unknown symbol \(args[1])\n".data(using: .utf8)!)
    exit(1)
}
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
                           bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                           colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
let scale = min(CGFloat(size) / img.size.width, CGFloat(size) / img.size.height)
let w = img.size.width * scale, h = img.size.height * scale
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
NSGraphicsContext.current?.imageInterpolation = .high
img.draw(in: NSRect(x: (CGFloat(size) - w) / 2, y: (CGFloat(size) - h) / 2, width: w, height: h),
         from: .zero, operation: .sourceOver, fraction: 1.0)
NSGraphicsContext.restoreGraphicsState()
// An error thrown from top-level code under `swift <file>` (immediate mode)
// traps the whole swift-frontend process — the shell sees "Trace/BPT trap",
// not a diagnostic — so failures must be caught and reported here.
do {
    try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: args[2]))
} catch {
    FileHandle.standardError.write("cannot write \(args[2]): \(error.localizedDescription)\n".data(using: .utf8)!)
    exit(1)
}
