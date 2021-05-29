package router

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type HandleFunc func(*Context) bool

// Handle static file.
type FileHandler struct {
	// Local file path.
	File string
}

// Can be use as HandlerFunc
func (h *FileHandler) Handle(c *Context) bool {
	http.ServeFile(c.Res, c.Req, h.File)
	return true
}

var errSeekOffset = errors.New("seek: invalid offset")

// Implements io.ReadSeeker, pass to http.ServeContent().
type cacheSeeker struct {
	// Data
	b []byte
	// Seek index.
	i int64
}

func (s *cacheSeeker) Seek(o int64, w int) (int64, error) {
	switch w {
	case io.SeekStart:
	case io.SeekCurrent:
		o += s.i
	case io.SeekEnd:
		o += int64(len(s.b))
	}
	if o < 0 {
		return 0, errSeekOffset
	}
	if o > int64(len(s.b)) {
		o = int64(len(s.b))
	}
	s.i = o
	return o, nil
}

func (s *cacheSeeker) Read(p []byte) (int, error) {
	if s.i >= int64(len(s.b)) {
		return 0, io.EOF
	}
	n := copy(p, s.b[s.i:])
	s.i += int64(n)
	return n, nil
}

func (s *cacheSeeker) Write(b []byte) (int, error) {
	s.b = append(s.b, b...)
	return len(b), nil
}

// Compression algorithm
const (
	gzipCompress = iota
	zlibCompress
	deflateCompress
)

var (
	// Create compressor functions.
	compressFunc = []func(io.Writer) io.WriteCloser{
		func(w io.Writer) io.WriteCloser {
			return gzip.NewWriter(w)
		},
		func(w io.Writer) io.WriteCloser {
			return zlib.NewWriter(w)
		},
		func(w io.Writer) io.WriteCloser {
			wc, _ := flate.NewWriter(w, flate.DefaultCompression)
			return wc
		},
	}
	compressName = []string{
		"gzip",
		"zlib",
		"deflate",
	}
)

// Handle memory cache.
type CacheHandler struct {
	ContentType string
	// Data modify time.
	ModTime time.Time
	// Origin data.
	Data           []byte
	compressedData [3][]byte
}

// Check client compressions and response compressed data.
// Can be use as HandlerFunc.
func (h *CacheHandler) Handle(c *Context) bool {
	if h.ContentType != "" {
		c.Res.Header().Set("Content-Type", h.ContentType)
	}
	// Check client compressions
	for _, s := range strings.Split(c.Req.Header.Get("Accept-Encoding"), ",") {
		switch s {
		case "*", "gzip":
			h.serveContent(c, gzipCompress)
			return true
		case "zlib":
			h.serveContent(c, zlibCompress)
			return true
		case "deflate":
			h.serveContent(c, deflateCompress)
			return true
		default:
			continue
		}
	}
	// Handler does not has client compressions.
	http.ServeContent(c.Res, c.Req, "", h.ModTime, &cacheSeeker{b: h.Data})
	return true
}

// Compress data and response. But if compressed data is bigger than origin data, return origin data.
// Compression is done when first called, and can not modify the compressed data by modify origin data.
func (h *CacheHandler) serveContent(c *Context, n int) {
	// Compress data if is empty.
	if len(h.compressedData[n]) < 1 {
		var buf bytes.Buffer
		w := compressFunc[n](&buf)
		w.Write(h.Data)
		w.Close()
		h.compressedData[n] = append(h.compressedData[n], buf.Bytes()...)
	}
	// Response compressed data.
	if len(h.compressedData[n]) < len(h.Data) {
		c.Res.Header().Set("Content-Encoding", compressName[n])
		http.ServeContent(c.Res, c.Req, "", h.ModTime, &cacheSeeker{b: h.compressedData[n]})
		return
	}
	// Response origin data.
	http.ServeContent(c.Res, c.Req, "", h.ModTime, &cacheSeeker{b: h.Data})
}
