package router

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// Context pool.
	contextPool sync.Pool
	// Content-Type.
	ContentTypeJSON = mime.TypeByExtension(".json")
	ContentTypeHTML = mime.TypeByExtension(".html")
	ContentTypeJS   = mime.TypeByExtension(".js")
	ContentTypeCSS  = mime.TypeByExtension(".css")
	// Hash pool.
	md5Pool    sync.Pool
	sha1Pool   sync.Pool
	sha256Pool sync.Pool
	sha512Pool sync.Pool
	// Random bytes, user for Context.RandomXXX
	randBytes []byte
	random    = rand.New(rand.NewSource(time.Now().Unix()))
)

func init() {
	// Context pool.
	contextPool.New = func() interface{} {
		c := new(Context)
		return c
	}
	// Content-Type.
	md5Pool.New = func() interface{} {
		return &hashBuffer{
			hash: md5.New(),
			buf:  make([]byte, 0, md5.Size*2),
			sum:  make([]byte, md5.Size),
		}
	}
	sha1Pool.New = func() interface{} {
		return &hashBuffer{
			hash: sha1.New(),
			buf:  make([]byte, 0, sha1.Size*2),
			sum:  make([]byte, sha1.Size),
		}
	}
	sha256Pool.New = func() interface{} {
		return &hashBuffer{
			hash: sha256.New(),
			buf:  make([]byte, 0, sha256.Size*2),
			sum:  make([]byte, sha256.Size),
		}
	}
	sha512Pool.New = func() interface{} {
		return &hashBuffer{
			hash: sha512.New(),
			buf:  make([]byte, 0, sha512.Size*2),
			sum:  make([]byte, sha512.Size),
		}
	}
	// Random bytes.
	for i := '0'; i <= '9'; i++ {
		randBytes = append(randBytes, byte(i))
	}
	for i := 'a'; i <= 'z'; i++ {
		randBytes = append(randBytes, byte(i))
	}
	for i := 'A'; i <= 'Z'; i++ {
		randBytes = append(randBytes, byte(i))
	}
}

func SetRandomBytes(b []byte) {
	randBytes = make([]byte, len(b))
	copy(randBytes, b)
}

// Use for hash operation.
type hashBuffer struct {
	hash hash.Hash
	buf  []byte
	sum  []byte
}

// Return hex string of s hash result.
func (h *hashBuffer) Hash(s string) string {
	h.buf = h.buf[:0]
	h.buf = append(h.buf, s...)
	h.hash.Reset()
	h.hash.Write(h.buf)
	h.hash.Sum(h.sum[:0])
	h.buf = h.buf[:h.hash.Size()*2]
	hex.Encode(h.buf, h.sum)
	return string(h.buf)
}

// Use hash of p to hash s.
func hashPoolHash(p *sync.Pool, s string) string {
	h := p.Get().(*hashBuffer)
	s = h.Hash(s)
	p.Put(h)
	return s
}

// Keep context data in the handler chain.
type Context struct {
	Req *http.Request
	Res http.ResponseWriter
	// The values of the parameter route, in the order of registration.
	Param []string
	// Keep user data in the handler chain.
	Data interface{}
	// A cache that you might use.
	Buff bytes.Buffer
}

// Set Content-Type and statusCode, convert data to JSON and write to body,
func (c *Context) WriteJSON(statusCode int, data interface{}) error {
	c.Res.WriteHeader(statusCode)
	c.Res.Header().Set("Content-Type", ContentTypeJSON)
	enc := json.NewEncoder(c.Res)
	return enc.Encode(data)
}

// Set Content-Type and statusCode, write to text body,
func (c *Context) WriteHTML(statusCode int, text string) error {
	c.Res.WriteHeader(statusCode)
	c.Res.Header().Set("Content-Type", ContentTypeHTML)
	_, err := io.WriteString(c.Res, text)
	return err
}

// Return n length random string in range of randBytes.
func (c *Context) RandomString(n int) string {
	var str strings.Builder
	for i := 0; i < n; i++ {
		str.WriteByte(randBytes[random.Intn(len(randBytes))])
	}
	return str.String()
}

// Return n length random number string in range of 0-9.
func (c *Context) RandomNumber(n int) string {
	var str strings.Builder
	for i := 0; i < n; i++ {
		str.WriteByte(randBytes[random.Intn(10)])
	}
	return str.String()
}

// Return hex string of s MD5 result.
func (c *Context) MD5(s string) string {
	return hashPoolHash(&md5Pool, s)
}

// Return hex string of s SHA1 result.
func (c *Context) SHA1(s string) string {
	return hashPoolHash(&sha1Pool, s)
}

// Return hex string of s SHA256 result.
func (c *Context) SHA256(s string) string {
	return hashPoolHash(&sha256Pool, s)
}

// Return hex string of s SHA512 result.
func (c *Context) SHA512(s string) string {
	return hashPoolHash(&sha512Pool, s)
}

// Json response of page query.
type PageData struct {
	// Total data.
	Total int64 `json:"total"`
	// Data list.
	Data interface{} `json:"data"`
}

// Page query conditions.
type PageQuery struct {
	// Field name used for sort data.
	Order string
	// ASC or DESC.
	Sort string
	// Begin of data.
	Begin int64
	// Total of data.
	Total int64
}

// Try to parse url queries value to q.
// Example: "/users?order=id&sort=desc&begin=1&total=10".
// It return query name if fail to parse begin or total.
func (c *Context) ParsePageQuery(q *PageQuery) string {
	order := c.Req.FormValue("order")
	if order != "" {
		q.Order = order
	}
	sort := c.Req.FormValue("sort")
	if sort != "" {
		q.Sort = sort
	}
	val := c.Req.FormValue("begin")
	if val != "" {
		begin, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return "begin"
		}
		q.Begin = begin
	}
	val = c.Req.FormValue("total")
	if val != "" {
		total, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return "total"
		}
		q.Total = total
	}
	return ""
}

// Return header["Authorization"] Bearer token.
func (c *Context) BearerToken() string {
	// 没有header
	token := c.Req.Header.Get("Authorization")
	if token == "" {
		return ""
	}
	const bearerTokenPrefix = "Bearer "
	if !strings.HasPrefix(token, bearerTokenPrefix) {
		return ""
	}
	return token[len(bearerTokenPrefix):]
}
