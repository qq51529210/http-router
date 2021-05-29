# http-router

A http router written in GO。

## Routers

- MethodRouter, match http method and url, can be used for web application development.
- PathRouter, only match url, can be used for gateway proxy development.

## Route path
- Param "/:", no need to know name, because we know the order.
- All match "/*", add any path after this route will return error.
- Static "/users".

注意，同一层的路由，只能有一种类型。比如，已有路由"/users/:int"，继续添加"/users/:float"或者"/users/root"会返回错误。

虽然同时提供了添加和删除路由的功能，但是不是同步的，在并发的时候修改路由，可能会造成匹配错误或者失败。

## Useage

Handle user register.

```go
var router MethodRouter
_, err := router.AddPost("/users", handlePostUsers)
if err != nil{
  panic(err)
}
```
Handle static file.

```go
var router MethodRouter
_, err := router.AddStatic(http.MethodGet, "./index.html", "/", true)
if err != nil{
  panic(err)
}
// Use FileHandler
var fileHandler FileHandler
fileHandler.File = "./index.html"
_, err = router.AddGet("/", fileHandler.Handle)
if err != nil{
  panic(err)
}
// Use CacheHandler
data, err := ioutil.ReadFile("./index.html")
if err != nil {
  panic(err)
}
var cacheHandler CacheHandler
cacheHandler.ContentType = mime.TypeByExtension("html")
cacheHandler.ModTime = time.Now()
cacheHandler.Data = data
_, err = router.AddGet("/", cacheHandler.Handle)
if err != nil{
  panic(err)
}
```

Add all files of a directory, remove file extension.

```go
var router MethodRouter
_, err := router.AddStatic(http.MethodGet, "./html", "/", true, "html")
if err != nil{
  panic(err)
}
```

Api gateway.

```go
var router PathRouter
_, err := router.Add("/user", handleForwardUser)
if err != nil{
  panic(err)
}
```

Reset handler chain.

```go
var router MethodRouter
// Static file.
_, err := router.AddStatic(http.MethodGet, "./index.html", "/index.html", true)
if err != nil{
  panic(err)
}
// route.Handle is a CacheHandler.
route := router.RouteGet("index.html")
if route == nil{
  panic("bug")
}
// Share this CacheHandler, so that route("/index.html") and route("/") can use same cache.
_, err = router.AddGet("/", route.Handle...)
if err != nil{
  panic(err)
}
```

Remove route, but it may never be used in the actual scene.

```go
var router PathRouter
_, err := router.Add("/users", handleForwardUser)
if err != nil{
  panic(err)
}
if !router.Remove("/users"){
  panic("bug")
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
pkg: github.com/qq51529210/web/router
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
