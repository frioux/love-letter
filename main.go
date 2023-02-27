package main

import (
	"embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"image/color"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"

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

func fakeRender(s string, w io.Writer) error {
	c := gg.NewContext(200, 96)
	c.SetColor(color.White)
	c.DrawRectangle(0, 0, 200, 96)
	c.Fill()
	c.SetColor(color.Black)
	if err := c.LoadFontFace("/usr/share/fonts/truetype/freefont/FreeMono.ttf", 20); err != nil {
		return err
	}
	c.DrawStringWrapped(s, 0, 0, 0, 0, 200, 1.2, gg.AlignLeft)
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
	cmd := exec.Command(p, r.Form.Get("s"))

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
}

func renderer(rw http.ResponseWriter, r *http.Request) {
	s := r.URL.Query().Get("s")
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

	return http.ListenAndServe(":8080", mux)
}
