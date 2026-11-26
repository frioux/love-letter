# love-letter

Love letter is a little web server that allows me to manage an eInk screen on a
pizero mounted to a little black walnut display I made for my wife for an
anniversary.

The UI is a small pixel editor that runs entirely in the browser.  You type a
message, optionally draw over it with a few basic tools (pencil, line,
rectangle, fill, eraser), and send it to the screen.  You can also load
whatever is currently on the screen back into the editor.

The canvas zooms on its own (the buttons under it, or a two-finger pinch and
drag) so the tools stay on screen.  On a phone, turn on Cursor to stop your
finger covering the pixel you are after: dragging moves a crosshair like a
trackpad, a tap plots a dot, and holding Draw with your other thumb draws
while you drag.  Ctrl/Cmd+Z undoes; Ctrl/Cmd+Shift+Z or Ctrl/Cmd+Y redoes.

The server never renders anything: the browser sends it the finished 200x96
one-bit frame and the server hands that to the
[EPD fuse driver](https://github.com/repaper/gratis) at `/dev/epd`.  That
driver (the `epd-fuse` service) is the only thing the pi needs besides the
`love-letter` binary itself.

## Building and deploying

`build.sh` builds the frontend with `npm` and then the backend, with the
frontend embedded in it, with `go`.  The result is a single `love-letter`
binary for the pi zero.  To deploy, copy it to the pi and restart the service:

```bash
$ ./build.sh
$ scp love-letter pi@pizero:love-letter.new
$ ssh pi@pizero 'mv love-letter.new love-letter && sudo systemctl restart loveletter'
```

The service is a minimal systemd unit at
`/etc/systemd/system/loveletter.service`:

```ini
[Unit]
Description=Love Letter

[Service]
ExecStart=/home/pi/love-letter
Restart=always
StartLimitBurst=0

[Install]
WantedBy=default.target
```

It listens on `:8080`; pass `-addr` to change that.

## API

 * `GET /api/screen` returns the frame currently on the screen.
 * `PUT /api/screen` displays the frame in the request body.

A frame is 2400 bytes: 96 rows of 25 bytes, top row first, leftmost pixel in
the most significant bit, 1 for white and 0 for black.

## HACKING

The UI functions almost totally in isolation, so if you want to tweak the UI
changedir into `fe` and run the npm dev server:

```bash
$ cd fe
$ npm install
$ npm run dev
```

That will start up a server to automatically detect and recompile the frontend
components.

The dev server proxies `/api` to `localhost:8080`, so to exercise loading and
saving without a pi, run the backend against a fake screen directory.  The
backend embeds `fe/dist`, so the frontend has to have been built at least once
(`./build.sh`, or `npm run build` in `fe`) before it will compile:

```bash
$ mkdir -p /tmp/epd/LE && head -c 2400 /dev/zero > /tmp/epd/LE/current_inverse
$ go run . -epd /tmp/epd
```

The frame the UI sends ends up in `/tmp/epd/LE/display_inverse`.

### The font

Browsers can only rasterize antialiased text, which looks blobby or broken when
thresholded to one bit at these sizes.  So text is drawn from FreeMono glyph
bitmaps in `fe/src/lib/font.json`, which were baked with
[Pillow](https://python-pillow.org/) and look exactly like the text the screen
showed back when the server rendered it.  To change the sizes or characters
available, edit `tools/bakefont.py` and rerun it on the pi, which has FreeMono
and the old Pillow (5.4) the script is written against; it uses `textsize`,
which Pillow 10 removed:

```bash
$ python3 tools/bakefont.py > fe/src/lib/font.json
```

Characters that aren't in the baked set fall back to a rough browser-drawn
glyph.
