package router

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	// "github.com/astaxie/beego"
	// beego_context "github.com/astaxie/beego/context"
	// "github.com/gin-gonic/gin"
)

func Test_diffString(t *testing.T) {
	s1, s2 := diffString("abc4", "abc123")
	if s1 != "4" || s2 != "123" {
		t.FailNow()
	}
	s1, s2 = diffString("abc4", "bc123")
	if s1 != "abc4" || s2 != "bc123" {
		t.FailNow()
	}
	s1, s2 = diffString("/0", "/")
	if s1 != "0" || s2 != "" {
		t.FailNow()
	}
}

func Test_splitPath(t *testing.T) {
	routePath, err := splitRoute("a/b/c/:/*/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(routePath) != 4 || routePath[0] != "/a/b/c/" || routePath[1] != ":" ||
		routePath[2] != "*" || routePath[3] != "a" {
		t.FailNow()
	}
	routePath, err = splitRoute(":/a/b/*123/")
	if err != nil {
		t.Fatal(err)
	}
	if len(routePath) != 4 || routePath[0] != "/" || routePath[1] != ":" ||
		routePath[2] != "a/b/" || routePath[3] != "*" {
		t.FailNow()
	}
}

type testHandler struct {
	header http.Header  // 接受http.ResponseWriter接口的数据
	buffer bytes.Buffer // 接受http.ResponseWriter接口的数据
	funcs  []string     // 函数被调用链
	param  []string     // 路由参数
}

// http.ResponseWriter接口
func (r *testHandler) Header() (h http.Header) {
	return r.header
}

// http.ResponseWriter接口
func (r *testHandler) Write(b []byte) (n int, err error) {
	return r.buffer.Write(b)
}

// http.ResponseWriter接口
func (r *testHandler) WriteString(s string) (n int, err error) {
	return r.buffer.WriteString(s)
}

// http.ResponseWriter接口
func (r *testHandler) WriteHeader(int) {
}

// 重置
func (r *testHandler) Reset() {
	r.header = make(http.Header)
	r.buffer.Reset()
	r.funcs = make([]string, 0)
}

// 拦截器
func (r *testHandler) Interceptor(c *Context) bool {
	r.funcs = append(r.funcs, "Interceptor")
	return true
}

// 匹配失败
func (r *testHandler) NotMatch(c *Context) bool {
	r.funcs = append(r.funcs, "NotMatch")
	return false
}

// Handle1
func (r *testHandler) Handle1(c *Context) bool {
	r.funcs = append(r.funcs, "Handle1")
	r.param = append(r.param, c.Param...)
	return true
}

// Handle2
func (r *testHandler) Handle2(c *Context) bool {
	r.funcs = append(r.funcs, "Handle2")
	return true
}

// 模拟http get请求
func testHttpGet(url string, handler *testHandler, router http.Handler) {
	handler.Reset()
	q, _ := http.NewRequest(http.MethodGet, url, nil)
	handler.Reset()
	q.Header.Add("Accept-Encoding", "gzip")
	router.ServeHTTP(handler, q)
}

// 打印route的所有路由
func testPrintRoute(route *Route, name []string, t *testing.T) {
	name = append(name, route.name)
	if route.param != nil {
		testPrintRoute(route.param, name, t)
		return
	}
	hasStatic := false
	for i := 0; i < len(route.static); i++ {
		if route.static[i] != nil {
			hasStatic = true
			testPrintRoute(route.static[i], name, t)
		}
	}
	if !hasStatic {
		t.Log(strings.Join(name, " > "))
	}
}

// 错误Fatal
func testFatalError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func Test_Router_Add_Match(t *testing.T) {
	var router MethodRouter
	var handler testHandler
	router.Interceptor = append(router.Interceptor, handler.Interceptor)
	router.NotMatch = append(router.NotMatch, handler.NotMatch)
	// add
	{
		// 不能出现错误
		addOK := func(route *Route, err error) {
			if err != nil {
				t.Log(err)
				t.FailNow()
			}
		}
		// 必须出现错误
		addErr := func(route *Route, err error) {
			if err == nil {
				t.FailNow()
			}
			t.Log(err)
		}
		// 添加相同的路径
		addOK(router.AddGet("/00"))
		addOK(router.AddGet("/00"))
		// 同一层目录下，添加不同的静态路径
		addOK(router.AddGet("/01"))
		addOK(router.AddGet("/1/0"))
		addOK(router.AddGet("/11/:/1"))
		addOK(router.AddGet("/111/*"))
		// 同一层目录下，不能同时添加静态路径和参数路径
		addErr(router.AddGet("/:"))
		addErr(router.AddGet("/*"))
		addErr(router.AddGet("/1/:"))
		addErr(router.AddGet("/1/*"))
		// 不能在全匹配后面添加任何的路径
		addOK(router.AddGet("/2/*"))
		addErr(router.AddGet("/2/*/1"))
		addErr(router.AddGet("/2/*/:"))
		addErr(router.AddGet("/2/*/*"))
		// 打印
		testPrintRoute(&router.route[methodGet], []string{}, t)
	}
	// match
	{
		// 静态路由
		router.AddGet("/4/5/6", handler.Handle1)
		testHttpGet("/4/5/6", &handler, &router)
		if len(handler.funcs) != 2 || handler.funcs[0] != "Interceptor" || handler.funcs[1] != "Handle1" {
			t.FailNow()
		}
		testHttpGet("/4/5/5", &handler, &router)
		if len(handler.funcs) != 2 || handler.funcs[0] != "Interceptor" || handler.funcs[1] != "NotMatch" {
			t.FailNow()
		}
		// 参数路由
		router.AddGet("/3/:/5/:/*", handler.Handle1, handler.Handle2)
		testHttpGet("/3/4/5/6/7/8", &handler, &router)
		if len(handler.funcs) != 3 || handler.funcs[0] != "Interceptor" || handler.funcs[1] != "Handle1" || handler.funcs[2] != "Handle2" {
			t.FailNow()
		}
		if len(handler.param) != 3 || handler.param[0] != "4" || handler.param[1] != "6" || handler.param[2] != "7/8" {
			t.FailNow()
		}
		// 匹配参数不完整
		testHttpGet("/2/3/4/5", &handler, &router)
		if len(handler.funcs) != 2 || handler.funcs[0] != "Interceptor" || handler.funcs[1] != "NotMatch" {
			t.FailNow()
		}
	}
}

func Test_Router_Remove(t *testing.T) {
	var handler testHandler
	var router MethodRouter
	router.NotMatch = append(router.NotMatch, handler.NotMatch)
	// 添加路由
	router.AddGet("/1", handler.Handle1)
	router.AddGet("/1/:", handler.Handle2)
	router.AddGet("/1/:/3", handler.Handle2)
	// 删除错误的路由
	if router.RemoveGet("/12") {
		t.FailNow()
	}
	// 删除路由
	if !router.RemoveGet("/1/:") {
		t.FailNow()
	}
	// 测试请求删除的路由
	testHttpGet("/1/2", &handler, &router)
	if len(handler.funcs) != 1 && handler.funcs[0] != "NotFound" {
		t.FailNow()
	}
	// 测试请求删除的路由的子路由
	testHttpGet("/1/2/3", &handler, &router)
	if len(handler.funcs) != 1 && handler.funcs[0] != "NotFound" {
		t.FailNow()
	}
	// 测试路由
	testHttpGet("/1", &handler, &router)
	if len(handler.funcs) != 1 && handler.funcs[0] != "Handle1" {
		t.FailNow()
	}
}

func Test_Router_AddStatic(t *testing.T) {
	var handler testHandler
	var router MethodRouter
	router.Interceptor = append(router.Interceptor, handler.Interceptor)
	router.NotMatch = append(router.NotMatch, handler.NotMatch)
	// 随机生成文件数据
	random := rand.New(rand.NewSource(time.Now().Unix()))
	fileData := make([]byte, random.Int31n(102400))
	random.Read(fileData)
	// 生成本地文件用于测试
	dirName := "test.dir"
	testFatalError(t, os.MkdirAll(dirName, os.ModePerm))
	// 测试完成后删除整个目录
	defer os.RemoveAll(dirName)
	// 写入文件
	testFatalError(t, ioutil.WriteFile(filepath.Join(dirName, "test.html"), fileData, os.ModePerm))
	testFatalError(t, ioutil.WriteFile(filepath.Join(dirName, "test.css"), fileData, os.ModePerm))
	testFatalError(t, ioutil.WriteFile(filepath.Join(dirName, "test.js"), fileData, os.ModePerm))
	// 路由，加载目录，缓存
	testFatalError(t, router.AddStatic(http.MethodGet, "/static", dirName, true))
	// 路由，加载目录，不缓存
	testFatalError(t, router.AddStatic(http.MethodGet, "/cache", dirName, false))
	// 测试
	testHttpGet("/static/test.html", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("html")) {
		t.FailNow()
	}
	testHttpGet("/cache/test.html", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("html")) {
		t.FailNow()
	}
	//
	testHttpGet("/static/test.css", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("css")) {
		t.FailNow()
	}
	testHttpGet("/cache/test.css", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("css")) {
		t.FailNow()
	}
	// js
	testHttpGet("/static/test.js", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("js")) {
		t.FailNow()
	}
	testHttpGet("/cache/test.js", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("js")) {
		t.FailNow()
	}
}

type testBenchmark struct {
	benchRouteCount                  int // 路径层级
	staticRoute, staticUrl           strings.Builder
	paramRoute, paramUrl             [3]strings.Builder
	staticParamRoute, staticParamUrl [3]strings.Builder
	paramStaticRoute, paramStaticUrl [3]strings.Builder
	myRouter                         MethodRouter
	// 	ginRouter                        *gin.Engine
	// 	beegoRouter                      *beego.ControllerRegister
}

func (t *testBenchmark) Init() {
	t.benchRouteCount = 10
	// 全静态根目录
	t.staticRoute.WriteString("/static")
	t.staticUrl.WriteString("/static")
	// 全参数根目录
	t.paramRoute[0].WriteString("/param")
	t.paramUrl[0].WriteString("/param")
	t.paramRoute[1].WriteString("/param")
	t.paramUrl[1].WriteString("/param")
	t.paramRoute[2].WriteString("/param")
	t.paramUrl[2].WriteString("/param")
	// 一半静态一半参数根目录
	t.staticParamRoute[0].WriteString("/static_param")
	t.staticParamUrl[0].WriteString("/static_param")
	t.staticParamRoute[1].WriteString("/static_param")
	t.staticParamUrl[1].WriteString("/static_param")
	t.staticParamRoute[2].WriteString("/static_param")
	t.staticParamUrl[2].WriteString("/static_param")
	// 一半参数一半静态根目录
	t.paramStaticRoute[0].WriteString("/param_static")
	t.paramStaticUrl[0].WriteString("/param_static")
	t.paramStaticRoute[1].WriteString("/param_static")
	t.paramStaticUrl[1].WriteString("/param_static")
	t.paramStaticRoute[2].WriteString("/param_static")
	t.paramStaticUrl[2].WriteString("/param_static")
	for i := 0; i < t.benchRouteCount; i++ {
		// 全静态
		t.staticRoute.WriteString(fmt.Sprintf("/static%d", i))
		t.staticUrl.WriteString(fmt.Sprintf("/static%d", i))
		// 全参数
		t.paramRoute[0].WriteString("/:")
		t.paramUrl[0].WriteString(fmt.Sprintf("/param%d", i))
		t.paramRoute[1].WriteString(fmt.Sprintf("/:param%d", i))
		t.paramUrl[1].WriteString(fmt.Sprintf("/param%d", i))
		t.paramRoute[2].WriteString(fmt.Sprintf("/:param%d", i))
		t.paramUrl[2].WriteString(fmt.Sprintf("/param%d", i))
		// 一半静态一半参数根目录
		t.staticParamRoute[0].WriteString(fmt.Sprintf("/static%d/:", i))
		t.staticParamUrl[0].WriteString(fmt.Sprintf("/static%d/param%d", i, i))
		t.staticParamRoute[1].WriteString(fmt.Sprintf("/static%d/:param%d", i, i))
		t.staticParamUrl[1].WriteString(fmt.Sprintf("/static%d/param%d", i, i))
		t.staticParamRoute[2].WriteString(fmt.Sprintf("/static%d/:param%d", i, i))
		t.staticParamUrl[2].WriteString(fmt.Sprintf("/static%d/param%d", i, i))
		// 一半参数一半静态根目录
		t.paramStaticRoute[0].WriteString(fmt.Sprintf("/:/static%d", i))
		t.paramStaticUrl[0].WriteString(fmt.Sprintf("/param%d/static%d", i, i))
		t.paramStaticRoute[1].WriteString(fmt.Sprintf("/:param%d/static%d", i, i))
		t.paramStaticUrl[1].WriteString(fmt.Sprintf("/param%d/static%d", i, i))
		t.paramStaticRoute[2].WriteString(fmt.Sprintf("/:param%d/static%d", i, i))
		t.paramStaticUrl[2].WriteString(fmt.Sprintf("/param%d/static%d", i, i))
	}
	// my
	t.myRouter.AddGet(t.staticRoute.String(), func(c *Context) bool { return true })
	t.myRouter.AddGet(t.paramRoute[0].String(), func(c *Context) bool { return true })
	t.myRouter.AddGet(t.staticParamRoute[0].String(), func(c *Context) bool { return true })
	t.myRouter.AddGet(t.paramStaticRoute[0].String(), func(c *Context) bool { return true })
	// 	// gin
	// 	gin.SetMode(gin.ReleaseMode)
	// 	t.ginRouter = gin.New()
	// 	t.ginRouter.GET(t.staticRoute.String(), func(c *gin.Context) {})
	// 	t.ginRouter.GET(t.paramRoute[1].String(), func(c *gin.Context) {})
	// 	t.ginRouter.GET(t.staticParamRoute[1].String(), func(c *gin.Context) {})
	// 	t.ginRouter.GET(t.paramStaticRoute[1].String(), func(c *gin.Context) {})
	// 	// beego
	// 	t.beegoRouter = beego.NewApp().Handlers
	// 	t.beegoRouter.Get(t.staticRoute.String(), func(c *beego_context.Context) {})
	// 	t.beegoRouter.Get(t.paramRoute[2].String(), func(c *beego_context.Context) {})
	// 	t.beegoRouter.Get(t.staticParamRoute[2].String(), func(c *beego_context.Context) {})
	// 	t.beegoRouter.Get(t.paramStaticRoute[2].String(), func(c *beego_context.Context) {})
}

func testNewBenchmark() *testBenchmark {
	p := new(testBenchmark)
	p.Init()
	return p
}

func (t *testBenchmark) benchmark(b *testing.B, url string, r http.Handler) {
	req, e := http.NewRequest(http.MethodGet, url, nil)
	if e != nil {
		b.Fatal(e)
	}
	h := new(testHandler)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(h, req)
	}
}

// 测试benchmark是否都能匹配到
func Test_Benchmark(t *testing.T) {
	var handler testHandler
	testBench := testNewBenchmark()
	testBench.myRouter.NotMatch = append(testBench.myRouter.NotMatch, func(c *Context) bool {
		t.FailNow()
		return false
	})
	testHttpGet(testBench.staticUrl.String(), &handler, &testBench.myRouter)
	testHttpGet(testBench.paramUrl[0].String(), &handler, &testBench.myRouter)
	testHttpGet(testBench.staticParamUrl[0].String(), &handler, &testBench.myRouter)
	testHttpGet(testBench.paramStaticUrl[0].String(), &handler, &testBench.myRouter)
	// // gin
	// testBench.ginRouter.NoRoute(func(c *gin.Context) {
	// 	t.FailNow()
	// })
	// testHttpGet(testBench.staticUrl.String(), &handler, testBench.ginRouter)
	// testHttpGet(testBench.paramUrl[1].String(), &handler, testBench.ginRouter)
	// testHttpGet(testBench.staticParamUrl[1].String(), &handler, testBench.ginRouter)
	// testHttpGet(testBench.paramStaticUrl[1].String(), &handler, testBench.ginRouter)
	// // beego
	// beego.ErrorHandler("404", func(writer http.ResponseWriter, request *http.Request) {
	// 	t.FailNow()
	// })
	// testHttpGet(testBench.staticUrl.String(), &handler, testBench.beegoRouter)
	// testHttpGet(testBench.paramUrl[2].String(), &handler, testBench.beegoRouter)
	// testHttpGet(testBench.staticParamUrl[2].String(), &handler, testBench.beegoRouter)
	// testHttpGet(testBench.paramStaticUrl[2].String(), &handler, testBench.beegoRouter)
}

func Benchmark_Match_My_Static(b *testing.B) {
	testBench := testNewBenchmark()
	testBench.benchmark(b, testBench.staticUrl.String(), &testBench.myRouter)
}

// func Benchmark_Match_Gin_Static(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.staticUrl.String(), testBench.ginRouter)
// }

// func Benchmark_Match_Beego_Static(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.staticUrl.String(), testBench.beegoRouter)
// }

func Benchmark_Match_My_Param(b *testing.B) {
	testBench := testNewBenchmark()
	testBench.benchmark(b, testBench.paramUrl[0].String(), &testBench.myRouter)
}

// func Benchmark_Match_Gin_Param(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.paramUrl[1].String(), testBench.ginRouter)
// }

// func Benchmark_Match_Beego_Param(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.paramUrl[2].String(), testBench.beegoRouter)
// }

func Benchmark_Match_My_StaticParam(b *testing.B) {
	testBench := testNewBenchmark()
	testBench.benchmark(b, testBench.staticParamUrl[0].String(), &testBench.myRouter)
}

// func Benchmark_Match_Gin_StaticParam(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.staticParamUrl[1].String(), testBench.ginRouter)
// }

// func Benchmark_Match_Beego_StaticParam(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.staticParamUrl[2].String(), testBench.beegoRouter)
// }

func Benchmark_Match_My_ParamStatic(b *testing.B) {
	testBench := testNewBenchmark()
	testBench.benchmark(b, testBench.paramStaticUrl[0].String(), &testBench.myRouter)
}

// func Benchmark_Match_Gin_ParamStatic(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.paramStaticUrl[1].String(), testBench.ginRouter)
// }

// func Benchmark_Match_Beego_ParamStatic(b *testing.B) {
// 	testBench := testNewBenchmark()
// 	testBench.benchmark(b, testBench.paramStaticUrl[2].String(), testBench.beegoRouter)
// }
