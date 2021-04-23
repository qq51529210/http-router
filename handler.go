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

// 处理请求的回调函数
type HandleFunc func(*Context) bool

// 用于处理一个静态文件
type FileHandler struct {
	File string // 文件路径/文件名
}

func (h *FileHandler) Handle(c *Context) bool {
	http.ServeFile(c.Response, c.Request, h.File)
	return true
}

var errSeekOffset = errors.New("seek: invalid offset")

// 实现io.ReadSeeker，http.ServeContent()需要的参数
type cacheSeeker struct {
	b []byte // 数据
	i int64  // seek下标
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

const (
	gzipCompress = iota
	zlibCompress
	deflateCompress
)

var (
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

// 用于处理一个数据缓存
type CacheHandler struct {
	ContentType  string    // response header Content-Type
	ModTime      time.Time // modify time
	Data         []byte    // 原始数据
	compressData [3][]byte // 压缩后的数据
}

func (h *CacheHandler) Handle(c *Context) bool {
	// 设置Content-type
	if h.ContentType != "" {
		c.Response.Header().Set("Content-Type", h.ContentType)
	}
	// 检查客户端是否支持压缩
	for _, s := range strings.Split(c.Request.Header.Get("Accept-Encoding"), ",") {
		switch s {
		case "*", "gzip": // 支持所有，或者gzip
			h.serveContent(c, gzipCompress)
			return true
		case "zlib": // 支持zlib
			h.serveContent(c, zlibCompress)
			return true
		case "deflate": // 支持deflate
			h.serveContent(c, deflateCompress)
			return true
		default:
			continue
		}
	}
	// 客户端不支持压缩
	http.ServeContent(c.Response, c.Request, "", h.ModTime, &cacheSeeker{b: h.Data})
	return true
}

// 压缩数据，并返回。每种算法只进行一次压缩，而且，如果压缩后的数据比原来的大，那么返回的是原来的数据。
func (h *CacheHandler) serveContent(c *Context, n int) {
	// 懒加载压缩数据
	if len(h.compressData[n]) < 1 {
		var buf bytes.Buffer
		w := compressFunc[n](&buf)
		// 不处理error，因为bytes.Buffer.Write不会返回error
		_, _ = w.Write(h.Data)
		_ = w.Close()
		h.compressData[n] = append(h.compressData[n], buf.Bytes()...)
	}
	// 返回压缩数据
	if len(h.compressData[n]) < len(h.Data) {
		c.Response.Header().Set("Content-Encoding", compressName[n])
		http.ServeContent(c.Response, c.Request, "", h.ModTime, &cacheSeeker{b: h.compressData[n]})
		return
	}
	// 压缩后的数据比压缩前还大，就返回压缩前的
	http.ServeContent(c.Response, c.Request, "", h.ModTime, &cacheSeeker{b: h.Data})
}
