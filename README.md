# love-letter

Love letter is a little web server that allows me to manage an eInk screen on a
pizero mounted to a little black walnut display I made for my wife for an
anniversary.

You can build the code by running `build.sh` which will build the frontend and backend
components with `npm` and `go` respectively.  The system assumes some hacks have been
applied to the [PaPiRus](https://github.com/PiSupply/PaPiRus) code on the pi zero:

```diff
diff --git a/papirus/epd.py b/papirus/epd.py
index 4e0c204..679fdb7 100644
--- a/papirus/epd.py
+++ b/papirus/epd.py
@@ -177,6 +177,11 @@ to use:
         if image.mode != "1":
             image = ImageOps.grayscale(image).convert("1", dither=Image.FLOYDSTEINBERG)
 
+        test_path = os.environ.get('TEST_IMAGE', '')
+        if test_path != '':
+            image.save(test_path, 'PNG')
+            return
+
         if image.mode != "1":
             raise EPDError('only single bit images are supported')
 
@@ -206,6 +211,8 @@ to use:
         self._command('C')
 
     def _command(self, c):
+        if os.environ.get('TEST_IMAGE', '') != '':
+           return
         if self._uselm75b:
             with open(os.path.join(self._epd_path, 'temperature'), 'wb') as f:
                 f.write(b(repr(self._lm75b.getTempC())))
```

This patch allows us to easily generate the image that will be written to the screen
without doing a lot of surgery.

## HACKING

The UI functions almost totally in isolation, so if you want to tweak
the UI changedir into `fe` and run the npm dev server:

```bash
$ cd fe
$ npm run dev
```

That will start up a server to automatically detect and recompile the frontend
components.
