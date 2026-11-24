# love-letter

Love letter is a little web server that allows me to manage an eInk screen on a
pizero mounted to a little black walnut display I made for my wife for an
anniversary.

The UI is a small pixel editor that runs entirely in the browser: text is
drawn from FreeMono glyph bitmaps (baked by `tools/bakefont.py`, so it looks
exactly like it always has on the screen), and you can
draw over it with a few basic tools (pencil, line, rectangle, fill, eraser).
The server never renders anything; the browser sends it the finished 200x96
one-bit frame and the server hands that to the
[EPD fuse driver](https://github.com/repaper/gratis) at `/dev/epd`.  Nothing
else needs to be installed on the pi.

You can build the code by running `build.sh` which will build the frontend and backend
components with `npm` and `go` respectively.  To deploy, copy the resulting
`love-letter` binary to the pi and restart the service:

```bash
$ ./build.sh
$ scp love-letter pi@pizero:love-letter.new
$ ssh pi@pizero 'mv love-letter.new love-letter && sudo systemctl restart loveletter'
```

## API

 * `GET /api/screen` returns the frame currently on the screen.
 * `PUT /api/screen` displays the frame in the request body.

A frame is 2400 bytes: 96 rows of 25 bytes, top row first, leftmost pixel in
the most significant bit, 1 for white and 0 for black.

## HACKING

The UI functions almost totally in isolation, so if you want to tweak
the UI changedir into `fe` and run the npm dev server:

```bash
$ cd fe
$ npm run dev
```

That will start up a server to automatically detect and recompile the frontend
components.

The dev server proxies `/api` to `localhost:8080`, so to exercise loading and
saving without a pi, run the backend against a fake screen directory:

```bash
$ mkdir -p /tmp/epd/LE && head -c 2400 /dev/zero > /tmp/epd/LE/current_inverse
$ go run . -epd /tmp/epd
```

The frame the UI sends ends up in `/tmp/epd/LE/display_inverse`.
