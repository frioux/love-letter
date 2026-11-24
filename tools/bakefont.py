#!/usr/bin/env python3
"""Bake FreeMono into one-bit glyph bitmaps for the browser to draw text with.

Browsers only rasterize text antialiased, which looks awful thresholded to one
bit at these sizes.  Pillow rasterizes hinted monochrome glyphs, which is what
the screen has always shown, so we capture exactly those.

Usage: bakefont.py > fe/src/lib/font.json   (needs Pillow and FreeMono)
"""

import base64
import json
import sys

from PIL import Image, ImageDraw, ImageFont

FONT = '/usr/share/fonts/truetype/freefont/FreeMono.ttf'
# FreeMono's advance is 0.6em, so multiples of five give a whole-pixel grid.
SIZES = [10, 15, 20, 25, 30, 40]
RANGES = [
    (0x20, 0x7E),      # ASCII
    (0xA1, 0xFF),      # Latin-1
    (0x2010, 0x2027),  # dashes, curly quotes, bullets, ellipsis
    (0x20AC, 0x20AC),  # euro
    (0x2190, 0x2199),  # arrows
    (0x2600, 0x266F),  # weather, faces, hearts, suits, notes
    (0x2701, 0x27BF),  # dingbats
]


def render(font, ch, w, h):
    image = Image.new('1', (w, h), 1)
    ImageDraw.Draw(image).text((0, 0), ch, font=font, fill=0)
    return image


def pack(image):
    """Row-major, most significant bit first, 1 = black."""
    w, h = image.size
    px = image.load()
    out = bytearray((w * h + 7) // 8)
    for i in range(w * h):
        if px[i % w, i // w] == 0:
            out[i >> 3] |= 0x80 >> (i & 7)
    return bytes(out)


def main():
    candidates = [chr(c) for lo, hi in RANGES for c in range(lo, hi + 1)]
    out = {}
    for size in SIZES:
        font = ImageFont.truetype(FONT, size)
        draw = ImageDraw.Draw(Image.new('1', (1, 1)))
        advance = draw.textsize('MM', font=font)[0] - draw.textsize('M', font=font)[0]
        height = sum(font.getmetrics())
        missing = pack(render(font, '￿', advance, height))
        chars, glyphs = [], []
        for ch in candidates:
            # Glyphs that aren't one cell wide would wreck the fixed grid.
            if draw.textsize('M' + ch, font=font)[0] - draw.textsize('M', font=font)[0] != advance:
                continue
            glyph = pack(render(font, ch, advance, height))
            if glyph == missing:
                continue
            chars.append(ch)
            glyphs.append(glyph)
        out[size] = {
            'advance': advance,
            'height': height,
            'chars': ''.join(chars),
            'glyphs': base64.b64encode(b''.join(glyphs)).decode(),
        }
        print('%dpx: %dx%d cells, %d glyphs' % (size, advance, height, len(chars)), file=sys.stderr)
    json.dump(out, sys.stdout, ensure_ascii=False, separators=(',', ':'))


if __name__ == '__main__':
    main()
