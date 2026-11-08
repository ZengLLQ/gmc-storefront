package platform

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

type ProcessedImage struct {
	Mime    string
	Bytes   int
	Width   int
	Height  int
	SHA256  []byte
	RawJPEG []byte
}

type Storage interface {
	ProcessAndSave(id uuid.UUID, r io.Reader) (*ProcessedImage, error)
	SaveExisting(id uuid.UUID, rawJPEG []byte) error
	OpenVariant(id uuid.UUID, size string) (*os.File, error)
	Delete(id uuid.UUID) error
}

type LocalStorage struct {
	baseDir string
	sem     chan struct{}
}

func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &LocalStorage{
		baseDir: baseDir,
		sem:     make(chan struct{}, 2), // max 2 concurrent image jobs
	}, nil
}

func (s *LocalStorage) ProcessAndSave(id uuid.UUID, r io.Reader) (*ProcessedImage, error) {
	// Limit upload to 5 MB + 1 byte to detect oversized files
	const maxBytes = 5 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, NewAppError(http.StatusBadRequest, "bad_request", "File gambar kosong")
	}
	if len(data) > maxBytes {
		return nil, NewAppError(http.StatusBadRequest, "bad_request", "Ukuran file maksimal 5 MB")
	}

	// Validate magic bytes
	mime := DetectImageMagicBytes(data)
	if mime == "" {
		return nil, NewAppError(http.StatusBadRequest, "bad_request", "Format gambar harus JPEG, PNG, atau WebP")
	}

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, NewAppError(http.StatusBadRequest, "bad_request", "File gambar rusak atau tidak dapat dibaca")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 6000 || cfg.Height > 6000 {
		return nil, NewAppError(http.StatusBadRequest, "bad_request", "Dimensi gambar maksimal 6000x6000 piksel")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, NewAppError(http.StatusBadRequest, "bad_request", "Gagal memproses gambar")
	}

	dir := filepath.Join(s.baseDir, id.String())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Re-encode strips EXIF metadata completely and writes 400, 800, 1600 widths
	widths := map[string]int{
		"400.jpg":  400,
		"800.jpg":  800,
		"1600.jpg": 1600,
	}
	var primaryBytes []byte
	for name, targetW := range widths {
		resized := resizeNearest(img, targetW)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 86}); err != nil {
			return nil, err
		}
		if name == "800.jpg" {
			primaryBytes = buf.Bytes()
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0644); err != nil {
			return nil, err
		}
	}

	sum := sha256.Sum256(data)
	return &ProcessedImage{
		Mime:    mime,
		Bytes:   len(data),
		Width:   cfg.Width,
		Height:  cfg.Height,
		SHA256:  sum[:],
		RawJPEG: primaryBytes,
	}, nil
}

func (s *LocalStorage) SaveExisting(id uuid.UUID, rawJPEG []byte) error {
	dir := filepath.Join(s.baseDir, id.String())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, name := range []string{"400.jpg", "800.jpg", "1600.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), rawJPEG, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (s *LocalStorage) OpenVariant(id uuid.UUID, size string) (*os.File, error) {
	var filename string
	switch size {
	case "400", "400.jpg":
		filename = "400.jpg"
	case "800", "800.jpg":
		filename = "800.jpg"
	case "1600", "1600.jpg":
		filename = "1600.jpg"
	default:
		return nil, errors.New("invalid size")
	}
	return os.Open(filepath.Join(s.baseDir, id.String(), filename))
}

func (s *LocalStorage) Delete(id uuid.UUID) error {
	return os.RemoveAll(filepath.Join(s.baseDir, id.String()))
}

func DetectImageMagicBytes(b []byte) string {
	if len(b) < 12 {
		return ""
	}
	// JPEG: FF D8 FF
	if b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47 &&
		b[4] == 0x0D && b[5] == 0x0A && b[6] == 0x1A && b[7] == 0x0A {
		return "image/png"
	}
	// WebP: RIFF .... WEBP
	if string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

func resizeNearest(src image.Image, maxW int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxW {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		return dst
	}
	newW := maxW
	newH := (h * maxW) / w
	if newH < 1 {
		newH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := b.Min.Y + (y*h)/newH
		for x := 0; x < newW; x++ {
			srcX := b.Min.X + (x*w)/newW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func UploadURL(id uuid.UUID, size int) string {
	return fmt.Sprintf("/uploads/%s/%d.jpg", id.String(), size)
}
