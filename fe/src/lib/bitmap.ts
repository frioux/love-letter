// Pixel helpers shared by the editor. A "bitmap" here is a Uint8Array of
// W*H pixels with 1 = white and 0 = black.

export const W = 200
export const H = 96
export const BYTES = (W / 8) * H // 2400

// Row-major, leftmost pixel in the most significant bit, 1 = white.
export function pack(px: Uint8Array): Uint8Array {
  const out = new Uint8Array(BYTES)
  for (let i = 0; i < W * H; i++) {
    if (px[i]) out[i >> 3] |= 0x80 >> (i & 7)
  }
  return out
}

export function unpack(bytes: Uint8Array): Uint8Array {
  const px = new Uint8Array(W * H)
  for (let i = 0; i < W * H; i++) {
    px[i] = (bytes[i >> 3] >> (7 - (i & 7))) & 1
  }
  return px
}
