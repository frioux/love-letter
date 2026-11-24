import { W, H } from './bitmap'

// Drawing layer values.
export const CLEAR = 0
export const BLACK = 1
export const WHITE = 2

type Plot = (x: number, y: number) => void

function inBounds(x: number, y: number): boolean {
  return x >= 0 && x < W && y >= 0 && y < H
}

// Stamps a size x size square around (x, y).
function stamp(x: number, y: number, size: number, plot: Plot) {
  const off = size >> 1
  for (let dy = 0; dy < size; dy++) {
    for (let dx = 0; dx < size; dx++) {
      const px = x - off + dx
      const py = y - off + dy
      if (inBounds(px, py)) plot(px, py)
    }
  }
}

// Bresenham line with a square brush.
export function line(x0: number, y0: number, x1: number, y1: number, size: number, plot: Plot) {
  const dx = Math.abs(x1 - x0)
  const dy = -Math.abs(y1 - y0)
  const sx = x0 < x1 ? 1 : -1
  const sy = y0 < y1 ? 1 : -1
  let err = dx + dy
  for (;;) {
    stamp(x0, y0, size, plot)
    if (x0 === x1 && y0 === y1) return
    const e2 = 2 * err
    if (e2 >= dy) { err += dy; x0 += sx }
    if (e2 <= dx) { err += dx; y0 += sy }
  }
}

// Rectangle outline between two corners.
export function rect(x0: number, y0: number, x1: number, y1: number, size: number, plot: Plot) {
  line(x0, y0, x1, y0, size, plot)
  line(x1, y0, x1, y1, size, plot)
  line(x1, y1, x0, y1, size, plot)
  line(x0, y1, x0, y0, size, plot)
}

// 4-connected flood fill over a W*H bitmap of comparable values. Returns the
// indices of the filled region. Iterative.
export function floodRegion(px: Uint8Array, x: number, y: number): number[] {
  const target = px[y * W + x]
  const seen = new Uint8Array(W * H)
  const region: number[] = []
  const stack = [y * W + x]
  seen[stack[0]] = 1
  while (stack.length) {
    const i = stack.pop()!
    region.push(i)
    const cx = i % W
    const cy = (i - cx) / W
    const next = [
      cx > 0 ? i - 1 : -1,
      cx < W - 1 ? i + 1 : -1,
      cy > 0 ? i - W : -1,
      cy < H - 1 ? i + W : -1,
    ]
    for (const n of next) {
      if (n >= 0 && !seen[n] && px[n] === target) {
        seen[n] = 1
        stack.push(n)
      }
    }
  }
  return region
}
