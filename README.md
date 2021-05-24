# router

基于标准库http.Handler接口实现的路由。

## 两种路由

- MethodRouter，匹配http method和url的路由，用于api服务开发。
- PathRouter，只匹配url的路由，不关心http method，可以用于api代理网关开发。

## 注册路由
- 参数路由，使用":"表示，例："/users/:"。参数路由不需要命名，因为知道顺序即可。
- 全匹配路由，使用"*"表示，例："/users/*"。全匹配路由后面，不能继续添加子路由，否则返回错误。
- 静态路由，例："/users"。

注意，同一层的路由，只能有一种类型。比如，已有路由"/users/:int"，继续添加"/users/:float"或者"/users/root"会返回错误。

虽然同时提供了添加和删除路由的功能，但是不是同步的，在并发的时候修改路由，可能会造成匹配错误或者失败。

## 使用路由

api服务，添加一个路由"post /users"，处理添加用户的请求。

```golang
var router MethodRouter
_, err := router.AddPost("/users", handlePostUsers)
if err != nil{
  panic(err)
}
```
api服务，添加一个路由"get /"处理主页，预先加载缓存程序目录下的单页面静态文件"index.html"。

```golang
var router MethodRouter
_, err := router.AddStatic(http.MethodGet, "./index.html", "/", true)
if err != nil{
  panic(err)
}
// 使用FileHandler
var fileHandler FileHandler
fileHandler.File = "./index.html"
_, err = router.AddGet("/", fileHandler.Handle)
if err != nil{
  panic(err)
}
// 使用CacheHandler
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

api服务，添加一个路由"get /"处理程序目录"html"下的所有文件。

```golang
var router MethodRouter
_, err := router.AddStatic(http.MethodGet, "./html", "/", true, "html")
if err != nil{
  panic(err)
}
```

api网关，添加一个路由"/user"，处理用户微服务代理转发。

```golang
var router PathRouter
_, err := router.Add("/user", handleForwardUser)
if err != nil{
  panic(err)
}
```

获取已知路由"/index.html"，重新设置HandleFunc。这个在多路径指向一个资源时有用。

```golang
var router MethodRouter
// 处理静态缓存文件
_, err := router.AddStatic(http.MethodGet, "./index.html", "/index.html", true)
if err != nil{
  panic(err)
}
// route.Handle是一个CacheHandler
route := router.RouteGet("index.html")
if route == nil{
  panic("这个开发包出bug了！")
}
// 共用这个CacheHandler，这样，路由"/index.html"和"/"使用了同一份缓存
_, err = router.AddGet("/", route.Handle...)
if err != nil{
  panic(err)
}
```

删除已知路由"/users"，暂时还想不到使用的场景

```golang
var router PathRouter
_, err := router.Add("/users", handleForwardUser)
if err != nil{
  panic(err)
}
if !router.Remove("/users"){
  panic("这个开发包出bug了！")
}
```

详细的操作，代码有注释，也可以参考router_test.go中的代码。

## 测试

my与beego和gin框架的路由benchmark比较，四种路由，共10层目录。router_test.go的注释掉了benchmark相关的代码。

- 全静态，/static0.../static9
- 全参数，/param0.../param9
- 半静态半参数，/static0/param0.../static9/param9
- 半参数半静态，/static0/param0.../static9/param9

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
