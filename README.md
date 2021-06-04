# http-router

A http router written in GO。

## Route path
- Param "/users/:", add"/users/*" or "/users/root" will return error. 

- All match "/users/\*", add"/users/any_path" will return error. 

- Static "/users".

## Call chain cases
- intercept -> handle -> release. 

- intercept -> release. 

- intercept -> notfound -> release.

## Useage

```go
var router Router
var route *Route
var err error
// Add a route and handler.
route, err := router.Add(http.MethodGet, "/users", handleGetUsers)
if err != nil {
  panic(err)
}
route, err = router.AddGet("/users/:", handleGetUser)
if err != nil {
  panic(err)
}
route, err = router.AddPost("/goods", handlePostGoods)
if err != nil {
  panic(err)
}
// Find route.
route = router.Route("/users")
if route == nil {
  panic("bug")
}
// Note! Because '/' is root of 'u' and 'g', so it can be found!
route = router.Route("/")
if route == nil {
  panic("bug")
}
// Remove route, it may never be used.
if !router.Remove("/users/:") {
  panic("bug")
}
// Route not found return false.
if router.Remove("/bills") {
  panic("bug")
}
// Add static file and cache it.
err = router.AddStatic(http.MethodGet, "/", "index.html", true)
if err != nil{
  panic(err)
}
// Share cache by set handler.
route = router.Route("/")
if route == nil {
  panic("bug")
}
// Path /index and / use one handler.
_, err = router.AddGet("/index", route.Handle...)
if err != nil{
  panic(err)
}
// FileHandler
var fileHandler FileHandler
fileHandler.File = "/index.css"
_, err = router.AddGet("/index.css", fileHandler.Handle)
if err != nil{
  panic(err)
}
// CacheHandler
data, err := ioutil.ReadFile("index.js")
if err != nil {
  panic(err)
}
var cacheHandler CacheHandler
cacheHandler.ContentType = mime.TypeByExtension("js")
cacheHandler.ModTime = time.Now()
cacheHandler.Data = data
_, err = router.AddGet("/index.js", cacheHandler.Handle)
if err != nil{
  panic(err)
}
// Add all files of directory, remove html file extension.
_, err = router.AddStatic(http.MethodGet, "/", "html", false, "html")
if err != nil{
  panic(err)
}
```
Your business code may like this:

```go
func intercept1 (c *Context) bool {
  // First handler, init data.
  c.Data = initData()
  if c.Data == nil {
    return false
  }
  return true
}

func intercept2 (c *Context) bool {
  handleData(c.Data)
  return true
}

func handle1 (c *Context) bool {
  if handleData(c.Data, c.Param[0]) {
  	return false
  }
  return true
}

func handle2 (c *Context) bool {
  return true
}

func handle3 (c *Context) bool {
  return true
}

func release (c *Context) bool {
  // release data.
  releaseData(c.Data)
  return true
}

var router Router
// Must set notfound to handle no match case.
router.SetNotfound(Notfound)
router.SetRelease(release)
router.SetIntercept(intercept1, intercept2)
router.AddGet("/path1/:", handle1, handle2)
router.AddGet("/path2/:", handle3)
```

Call chains like this:

```go
func ServeHTTP(){
  context
  if !intercept(context){
    release(context)
  }
  if !match(){
    notfound(context)
    release(context)
  }
  handle(context)
  release(context)
}
```



## Benchmark

Compared with the popular framework beego and gin. 

In router_test.go file, beego and gin's code has been commented out because they import too many packages.

- /static0.../static9
- /param0.../param9
- /static0/param0.../static9/param9
- /static0/param0.../static9/param9

```golang
goos: darwin
goarch: amd64
pkg: github.com/qq51529210/http-router
Benchmark_Match_My_Static-4             23759918                46.1 ns/op             0 B/op          0 allocs/op
Benchmark_Match_Gin_Static-4            18812461                65.2 ns/op             0 B/op          0 allocs/op
Benchmark_Match_Beego_Static-4            377347              3183 ns/op             992 B/op          9 allocs/op
Benchmark_Match_My_Param-4              12076794                90.5 ns/op             0 B/op          0 allocs/op
Benchmark_Match_Gin_Param-4              6138040               176 ns/op               0 B/op          0 allocs/op
Benchmark_Match_Beego_Param-4             698432              1713 ns/op             352 B/op          3 allocs/op
Benchmark_Match_My_StaticParam-4         7942041               154 ns/op               0 B/op          0 allocs/op
Benchmark_Match_Gin_StaticParam-4        6018007               190 ns/op               0 B/op          0 allocs/op
Benchmark_Match_Beego_StaticParam-4       474066              2162 ns/op             352 B/op          3 allocs/op
Benchmark_Match_My_ParamStatic-4         7040661               163 ns/op               0 B/op          0 allocs/op
Benchmark_Match_Gin_ParamStatic-4        6079212               195 ns/op               0 B/op          0 allocs/op
Benchmark_Match_Beego_ParamStatic-4       452582              2227 ns/op             352 B/op          3 allocs/op
PASS
ok      github.com/qq51529210/web/router        17.925s
```
