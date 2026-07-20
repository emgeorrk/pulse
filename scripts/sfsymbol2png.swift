// sfsymbol2png renders an SF Symbol into a transparent PNG of the given
// pixel size (fitted, centered). Used by gen-icons.sh for glyphs the Vitals
// set lacks (the Settings gear).
//
// Centering is optical, not just geometric: after a bounding-box-centered
// draw the glyph is re-drawn shifted so its ink (alpha) centroid lands on
// the canvas center. Symmetric glyphs round to a zero shift; side-heavy ones
// (rectangle.portrait.and.arrow.right — a left-flush rectangle with a thin
// trailing arrow) would otherwise sit visibly left of the neighboring icons
// in the menu's icon column.
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

func render(offsetX: CGFloat) -> NSBitmapImageRep {
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
                               bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                               colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    let scale = min(CGFloat(size) / img.size.width, CGFloat(size) / img.size.height)
    let w = img.size.width * scale, h = img.size.height * scale
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    NSGraphicsContext.current?.imageInterpolation = .high
    img.draw(in: NSRect(x: (CGFloat(size) - w) / 2 + offsetX, y: (CGFloat(size) - h) / 2, width: w, height: h),
             from: .zero, operation: .sourceOver, fraction: 1.0)
    NSGraphicsContext.restoreGraphicsState()
    return rep
}

func alphaCentroidX(_ rep: NSBitmapImageRep) -> CGFloat {
    var sum = 0.0, sx = 0.0
    for y in 0..<rep.pixelsHigh {
        for x in 0..<rep.pixelsWide {
            let a = Double(rep.colorAt(x: x, y: y)?.alphaComponent ?? 0)
            sum += a
            sx += a * Double(x)
        }
    }
    return sum > 0 ? CGFloat(sx / sum) : CGFloat(size - 1) / 2
}

var rep = render(offsetX: 0)
let shift = ((CGFloat(size - 1) / 2) - alphaCentroidX(rep)).rounded()
if shift != 0 {
    rep = render(offsetX: shift)
}

// An error thrown from top-level code under `swift <file>` (immediate mode)
// traps the whole swift-frontend process — the shell sees "Trace/BPT trap",
// not a diagnostic — so failures must be caught and reported here.
do {
    try rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: args[2]))
} catch {
    FileHandle.standardError.write("cannot write \(args[2]): \(error.localizedDescription)\n".data(using: .utf8)!)
    exit(1)
}
