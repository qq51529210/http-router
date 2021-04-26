package router

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
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
	contextPool sync.Pool // context池
	//
	contentTypeJSON = mime.TypeByExtension(".json")
	contentTypeHTML = mime.TypeByExtension(".html")
	//
	md5Pool    sync.Pool
	sha1Pool   sync.Pool
	sha256Pool sync.Pool
	//
	randBytes []byte                                        // 随机字符
	random    = rand.New(rand.NewSource(time.Now().Unix())) // 随机数
)

// 初始化Context缓存
func init() {
	contextPool.New = func() interface{} {
		c := new(Context)
		return c
	}
	// 哈希池
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
	// 随机字符
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

// 设置RandomString产生的字符，不是同步的
func SetRandomBytes(b []byte) {
	randBytes = make([]byte, len(b))
	copy(randBytes, b)
}

// 用于做hash运算的缓存
type hashBuffer struct {
	hash hash.Hash
	buf  []byte
	sum  []byte
}

// hash并转成15进制
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

// 路由匹配的上下文数据，HandleFunc的参数
type Context struct {
	Request  *http.Request       // http.ServeHTTP接口参数
	Response http.ResponseWriter // http.ServeHTTP接口参数
	Param    []string            // 参数路由的值
	UserData interface{}         // 应用有时候需要在调用链中传递一些数据
}

// 将v格式化成JSON，设置Content-Type，写入response
func (c *Context) WriteJSON(statusCode int, data interface{}) error {
	c.Response.WriteHeader(statusCode)
	c.Response.Header().Set("Content-Type", contentTypeJSON)
	enc := json.NewEncoder(c.Response)
	return enc.Encode(data)
}

// 设置Content-Type，写入response
func (c *Context) WriteHTML(statusCode int, text string) error {
	c.Response.WriteHeader(statusCode)
	c.Response.Header().Set("Content-Type", contentTypeHTML)
	// http.ResponseWriter底层是bufio.Writer
	_, err := io.WriteString(c.Response, text)
	return err
}

// 返回随机字符串"a-z 0-9 A-Z"，n:长度
func (c *Context) RandomString(n int) string {
	var str strings.Builder
	for i := 0; i < n; i++ {
		str.WriteByte(randBytes[random.Intn(len(randBytes))])
	}
	return str.String()
}

// 随机数字字符串"0-9"，n:长度
func (c *Context) RandomNumber(n int) string {
	var str strings.Builder
	for i := 0; i < n; i++ {
		str.WriteByte(randBytes[random.Intn(10)])
	}
	return str.String()
}

// 对字符串s作md5的运算，然后返回16进制的字符串值
func (c *Context) MD5(s string) string {
	h := md5Pool.Get().(*hashBuffer)
	s = h.Hash(s)
	md5Pool.Put(h)
	return s
}

// 对字符串s作sha1的运算，然后返回16进制的字符串值
func (c *Context) SHA1(s string) string {
	h := sha1Pool.Get().(*hashBuffer)
	s = h.Hash(s)
	sha1Pool.Put(h)
	return s
}

// 对字符串s作sha256的运算，然后返回16进制的字符串值
func (c *Context) SHA256(s string) string {
	h := sha256Pool.Get().(*hashBuffer)
	s = h.Hash(s)
	sha256Pool.Put(h)
	return s
}

// 分页查询结果
type PageData struct {
	Total int64       `json:"total"` // 总数
	Data  interface{} `json:"data"`  // 数据
}

// 分页查询参数
type PageQuery struct {
	Order string // 排序的字段名
	Sort  string // 排序的方式
	Begin int64  // 分页起始
	Total int64  // 分页总数
}

// 解析分页查询的参数，解析begin和total错误时返回，比如，"/users?order=id&sort=desc&begin=1&total=10"
// var q PageQuer
// q.Order = "id"
// q.Sort = "asc"
// // 初始化字段，如果没有相应的参数，不会赋值的
// c.ParsePageQuery(&q)
func (c *Context) ParsePageQuery(q *PageQuery) error {
	order := c.Request.FormValue("order")
	if order != "" {
		q.Order = order
	}
	sort := c.Request.FormValue("sort")
	if sort != "" {
		q.Sort = sort
	}
	val := c.Request.FormValue("begin")
	if val != "" {
		begin, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		q.Begin = begin
	}
	val = c.Request.FormValue("total")
	if val != "" {
		total, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		q.Total = total
	}
	return nil
}

// 返回request.header.Authorization的token
func (c *Context) BearerToken() string {
	// 没有header
	token := c.Request.Header.Get("Authorization")
	if token == "" {
		return ""
	}
	const bearerTokenPrefix = "Bearer "
	if !strings.HasPrefix(token, bearerTokenPrefix) {
		return ""
	}
	return token[len(bearerTokenPrefix):]
}
