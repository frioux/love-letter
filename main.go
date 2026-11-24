package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// The panel is 200x96 at one bit per pixel.  Frames are exchanged with the
// browser in exactly the format the EPD fuse driver's LE/*_inverse files use:
// row-major, leftmost pixel in the most significant bit, 1 = white.
const frameSize = 200 * 96 / 8

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type screen struct {
	dir string

	// mu serializes panel updates; the driver has a single frame buffer.
	mu sync.Mutex
}

func (s *screen) current() ([]byte, error) {
	return os.ReadFile(filepath.Join(s.dir, "LE", "current_inverse"))
}

func (s *screen) show(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(filepath.Join(s.dir, "LE", "display_inverse"), frame, 0o666); err != nil {
		return err
	}

	// The panel's refresh timing depends on temperature, but a broken sensor
	// shouldn't stop us from updating with whatever the driver last had.
	if t, err := readTempC(); err != nil {
		fmt.Fprintf(os.Stderr, "couldn't read temperature: %s\n", err)
	} else if err := os.WriteFile(filepath.Join(s.dir, "temperature"), []byte(strconv.Itoa(t)), 0o666); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(s.dir, "command"), []byte("U"), 0o222)
}

func (s *screen) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		frame, err := s.current()
		if err != nil {
			fail(rw, err, http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/octet-stream")
		rw.Header().Set("Cache-Control", "no-store")
		rw.Write(frame)
	case http.MethodPut:
		frame, err := io.ReadAll(http.MaxBytesReader(rw, r.Body, frameSize))
		if err != nil {
			fail(rw, err, http.StatusBadRequest)
			return
		}
		if len(frame) != frameSize {
			fail(rw, fmt.Errorf("frame must be %d bytes, got %d", frameSize, len(frame)), http.StatusBadRequest)
			return
		}
		if err := s.show(frame); err != nil {
			fail(rw, err, http.StatusInternalServerError)
			return
		}
		fmt.Printf("updated screen for %s\n", r.RemoteAddr)
		rw.WriteHeader(http.StatusNoContent)
	default:
		rw.Header().Set("Allow", "GET, PUT")
		fail(rw, errors.New("method not allowed"), http.StatusMethodNotAllowed)
	}
}

func fail(rw http.ResponseWriter, err error, code int) {
	fmt.Fprintln(os.Stderr, err)
	http.Error(rw, err.Error(), code)
}

//go:embed fe/dist/*
var assets embed.FS

func run() error {
	addr := flag.String("addr", ":8080", "address to listen on")
	epd := flag.String("epd", "/dev/epd", "path to the EPD fuse mount")
	flag.Parse()

	mux := http.NewServeMux()

	mux.Handle("/api/screen", &screen{dir: *epd})
	sub, err := fs.Sub(assets, "fe/dist")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	fmt.Printf("listening on %s\n", *addr)
	return http.ListenAndServe(*addr, mux)
}
