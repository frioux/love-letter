package main

import (
	"embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"image/color"
	"io"
	"io/ioutil"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"github.com/fogleman/gg"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func tryRender(s string, w io.Writer) error {
	err := realRender(s, w)
	if err == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "couldn't do realRender, falling back to fakeRender: %s\n", err)
	if err := fakeRender(s, w); err != nil {
		return err
	}

	return nil
}

// hackString replaces known bad characters with possible replacements.
//
// As far as I know this is a limitation of Pillows, the image library that
// papyrus uses to generate images.
func hackString(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '“' || r == '”' {
			return '"'
		}
		if r == '’' {
			return '\''
		}
		return r
	}, s)
}

func setFF(c *gg.Context, path string, points float64) error {
	fontBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return err
	}
	face := truetype.NewFace(f, &truetype.Options{
		Size: points,
		Hinting: font.HintingFull,
	})

	c.SetFontFace(face)

	return nil
}

func fakeRender(s string, w io.Writer) error {
	c := gg.NewContext(200, 96)
	c.SetColor(color.White)
	c.DrawRectangle(0, 0, 200, 96)
	c.Fill()
	c.SetColor(color.Black)
	if err := setFF(c, "/usr/share/fonts/truetype/freefont/FreeMono.ttf", 20); err != nil {
		return err
	}
	c.DrawStringWrapped(s, 1, 1, 0, 0, 200, 1, gg.AlignLeft)
	i := image.NewPaletted(image.Rect(0, 0, 200, 96), color.Palette{color.Black, color.White})

	draw.FloydSteinberg.Draw(i, i.Bounds(), c.Image(), image.ZP)
	return png.Encode(w, i)
}

func realRender(s string, w io.Writer) error {
	p, err := exec.LookPath("papirus-write")
	if err != nil {
		return err
	}

	cmd := exec.Command(p, s)
	tmp, err := os.CreateTemp("/tmp", "*.png")
	if err != nil {
		return err
	}
	defer tmp.Close()
	defer os.Remove(tmp.Name())

	cmd.Env = append(cmd.Env, "TEST_IMAGE="+tmp.Name())
	if err := cmd.Run(); err != nil {
		return err
	}

	if _, err := io.Copy(w, tmp); err != nil {
		return err
	}

	return nil
}

func save(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	p, err := exec.LookPath("papirus-write")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	cmd := exec.Command(p, hackString(r.Form.Get("s")))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}

func renderer(rw http.ResponseWriter, r *http.Request) {
	s := hackString(r.URL.Query().Get("s"))
	fmt.Println(s)
	tryRender(s, rw)
}

//go:embed fe/dist/*
var assets embed.FS

func run() error {
	mux := http.NewServeMux()

	mux.Handle("/render/", http.HandlerFunc(renderer))
	mux.Handle("/save/", http.HandlerFunc(save))
	sub, err := fs.Sub(assets, "fe/dist")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	fmt.Println("listening on :8080")
	return http.ListenAndServe(":8080", mux)
}
