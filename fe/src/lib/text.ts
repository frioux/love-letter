import { W, H } from './bitmap'
import baked from './font.json'

// Text is drawn from glyph bitmaps baked by tools/bakefont.py rather than by
// the browser, which can only rasterize antialiased text; thresholding that to
// one bit gives blobby or broken strokes at these sizes.

type Font = {
  advance: number
  height: number
  // Code point -> index of the glyph's first byte in bits.
  offsets: Map<number, number>
  // Per glyph: advance x height bits, row-major, MSB first, 1 = black.
  bits: Uint8Array
}

export const SIZES = Object.keys(baked).map(Number)

const fonts = new Map<number, Font>()

function font(size: number): Font {
  let f = fonts.get(size)
  if (!f) {
    const { advance, height, chars, glyphs } = baked[String(size) as keyof typeof baked]
    const glyphBytes = Math.ceil((advance * height) / 8)
    const offsets = new Map<number, number>()
    Array.from(chars).forEach((ch, i) => offsets.set(ch.codePointAt(0)!, i * glyphBytes))
    f = { advance, height, offsets, bits: Uint8Array.from(atob(glyphs), c => c.charCodeAt(0)) }
    fonts.set(size, f)
  }
  return f
}

// Greedy word wrap on spaces; explicit newlines force breaks. A word wider
// than the screen gets its own line and overflows.
function layout(text: string, columns: number): string[][] {
  const lines: string[][] = []
  for (const para of text.split('\n')) {
    let line: string[] = []
    for (const word of para.split(' ')) {
      const chars = Array.from(word)
      if (line.length > 0 && line.length + 1 + chars.length > columns) {
        lines.push(line)
        line = chars
      } else {
        if (line.length > 0) line.push(' ')
        line.push(...chars)
      }
    }
    lines.push(line)
  }
  return lines
}

const fallback = document.createElement('canvas').getContext('2d', { willReadFrequently: true })!

// Characters the font lacks (emoji, mostly) are drawn by the browser and
// thresholded: rough, but better than dropping them.
function fallbackGlyph(ch: string, f: Font, plot: (x: number, y: number) => void) {
  const { advance: w, height: h } = f
  fallback.canvas.width = w
  fallback.canvas.height = h
  fallback.fillStyle = '#fff'
  fallback.fillRect(0, 0, w, h)
  fallback.fillStyle = '#000'
  fallback.font = `${Math.round(h * 0.8)}px monospace`
  fallback.textBaseline = 'middle'
  fallback.textAlign = 'center'
  fallback.fillText(ch, w / 2, h / 2, w)
  const data = fallback.getImageData(0, 0, w, h).data
  for (let i = 0; i < w * h; i++) {
    // Luminance, so coloured emoji keep some shape.
    if (data[i * 4] * 0.3 + data[i * 4 + 1] * 0.6 + data[i * 4 + 2] * 0.1 < 128) plot(i % w, (i - (i % w)) / w)
  }
}

// Renders text to a 1-bit W*H bitmap (1 = white), black on white. Size must
// be one of SIZES.
export function renderText(text: string, size: number): Uint8Array {
  const f = font(size)
  const px = new Uint8Array(W * H).fill(1)
  layout(text, Math.floor(W / f.advance)).forEach((line, row) => {
    line.forEach((ch, col) => {
      const plot = (x: number, y: number) => {
        x += col * f.advance
        y += row * size
        if (x < W && y < H) px[y * W + x] = 0
      }
      const offset = f.offsets.get(ch.codePointAt(0)!)
      if (offset === undefined) {
        fallbackGlyph(ch, f, plot)
        return
      }
      for (let i = 0; i < f.advance * f.height; i++) {
        if (f.bits[offset + (i >> 3)] & (0x80 >> (i & 7))) plot(i % f.advance, (i - (i % f.advance)) / f.advance)
      }
    })
  })
  return px
}
