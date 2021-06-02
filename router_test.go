package router

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
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
	header http.Header
	buffer bytes.Buffer
	funcs  []string
	param  []string
}

func (r *testHandler) Header() (h http.Header) {
	return r.header
}

func (r *testHandler) Write(b []byte) (n int, err error) {
	return r.buffer.Write(b)
}

func (r *testHandler) WriteString(s string) (n int, err error) {
	return r.buffer.WriteString(s)
}

func (r *testHandler) WriteHeader(int) {
}

func (r *testHandler) Reset() {
	r.header = make(http.Header)
	r.buffer.Reset()
	r.funcs = make([]string, 0)
}

func (r *testHandler) Intercept(c *Context) bool {
	r.funcs = append(r.funcs, "Intercept")
	return true
}

func (r *testHandler) Notfound(c *Context) bool {
	r.funcs = append(r.funcs, "Notfound")
	return true
}

func (r *testHandler) Handle1(c *Context) bool {
	r.funcs = append(r.funcs, "Handle1")
	r.param = append(r.param, c.Param...)
	return true
}

func (r *testHandler) Handle2(c *Context) bool {
	r.funcs = append(r.funcs, "Handle2")
	return true
}

func testHttpGet(url string, handler *testHandler, router http.Handler) {
	handler.Reset()
	q, _ := http.NewRequest(http.MethodGet, url, nil)
	handler.Reset()
	q.Header.Add("Accept-Encoding", "gzip")
	router.ServeHTTP(handler, q)
}

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

func testFatalError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func testMustAdd(t *testing.T, r *rootRoute, s string) {
	_, err := r.Add(s)
	if err != nil {
		t.Fatal(err)
	}
}

func testMustNotAdd(t *testing.T, r *rootRoute, s string) {
	_, err := r.Add(s)
	if err == nil {
		t.FailNow()
	} else {
		t.Log(err)
	}
}

func Test_Add_Match(t *testing.T) {
	root := new(rootRoute)
	// Test add.
	{
		// Add same route.
		testMustAdd(t, root, "/00")
		testMustAdd(t, root, "/00")
		// Add different static route with same prefix '/'.
		testMustAdd(t, root, "/01")
		testMustAdd(t, root, "/1/0")
		testMustAdd(t, root, "/11/:/1")
		testMustAdd(t, root, "/111/*")
		// Add param route and static route to '/' at the same time.
		testMustNotAdd(t, root, "/:")
		testMustNotAdd(t, root, "/*")
		testMustNotAdd(t, root, "/1/:")
		testMustNotAdd(t, root, "/1/*")
		// Add route after a all match route.
		testMustAdd(t, root, "/2/*")
		testMustNotAdd(t, root, "/2/*/1")
		testMustNotAdd(t, root, "/2/*/:")
		testMustNotAdd(t, root, "/2/*/*")
		// print
		testPrintRoute(&root.route, []string{}, t)
	}
	// Test match
	{
		var ctx Context
		ctx.Req = new(http.Request)
		ctx.Req.URL = new(url.URL)
		var route *Route
		// Static
		testMustAdd(t, root, "/4/5/6")
		ctx.Req.URL.Path = "/4/5/6"
		route = root.Match(&ctx)
		if route == nil {
			t.FailNow()
		}
		ctx.Req.URL.Path = "/4/5/5"
		route = root.Match(&ctx)
		if route != nil {
			t.FailNow()
		}
		// Param
		ctx.Param = ctx.Param[:0]
		testMustAdd(t, root, "/3/:/5/:/*")
		ctx.Req.URL.Path = "/3/4/5/6/7/8"
		route = root.Match(&ctx)
		if route == nil || len(ctx.Param) != 3 || ctx.Param[0] != "4" || ctx.Param[1] != "6" || ctx.Param[2] != "7/8" {
			t.FailNow()
		}
		ctx.Param = ctx.Param[:0]
		ctx.Req.URL.Path = "/3/4/5/6"
		route = root.Match(&ctx)
		if route == nil || len(ctx.Param) != 2 || ctx.Param[0] != "4" || ctx.Param[1] != "6" {
			t.FailNow()
		}
	}
}

func Test_Remove(t *testing.T) {
	var root rootRoute
	var route *Route
	var handle = func(c *Context) bool { return true }
	// Add
	route, _ = root.Add("/1")
	route.Handle = append(route.Handle, handle)
	route, _ = root.Add("/1/:")
	route.Handle = append(route.Handle, handle)
	route, _ = root.Add("/1/:/3")
	route.Handle = append(route.Handle, handle)
	// Remove a non existent route。
	if root.Remove("/12") {
		t.FailNow()
	}
	// Remove
	if !root.Remove("/1/:") {
		t.FailNow()
	}
	var ctx Context
	ctx.Req = new(http.Request)
	ctx.Req.URL = new(url.URL)
	// Test match removed route.
	ctx.Req.URL.Path = "/1/2"
	route = root.Match(&ctx)
	if route != nil {
		t.FailNow()
	}
	// Test match removed route's sub route.
	ctx.Req.URL.Path = "/1/2/3"
	route = root.Match(&ctx)
	if route != nil {
		t.FailNow()
	}
	// Test match parent.
	ctx.Req.URL.Path = "/1"
	route = root.Match(&ctx)
	if route == nil {
		t.FailNow()
	}
}

func Test_Router_AddStatic(t *testing.T) {
	var handler testHandler
	var router Router
	router.SetIntercept(handler.Intercept)
	router.SetNotfound(handler.Notfound)
	// Generate random file data.
	random := rand.New(rand.NewSource(time.Now().Unix()))
	fileData := make([]byte, random.Int31n(102400))
	random.Read(fileData)
	// Create dir.
	dirName := "test.dir"
	testFatalError(t, os.MkdirAll(dirName, os.ModePerm))
	// Delete dir.
	defer os.RemoveAll(dirName)
	// Create files.
	testFatalError(t, ioutil.WriteFile(filepath.Join(dirName, "test.html"), fileData, os.ModePerm))
	testFatalError(t, ioutil.WriteFile(filepath.Join(dirName, "test.css"), fileData, os.ModePerm))
	testFatalError(t, ioutil.WriteFile(filepath.Join(dirName, "test.js"), fileData, os.ModePerm))
	// Add static file.
	testFatalError(t, router.AddStatic(http.MethodGet, "/static", dirName, true))
	// Add cache file.
	testFatalError(t, router.AddStatic(http.MethodGet, "/cache", dirName, false))
	// html
	testHttpGet("/static/test.html", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("html")) {
		t.FailNow()
	}
	testHttpGet("/cache/test.html", &handler, &router)
	if !strings.Contains(handler.header.Get("Content-Type"), mime.TypeByExtension("html")) {
		t.FailNow()
	}
	// css
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
	// How many levels of directory.
	benchRouteCount                  int
	staticRoute, staticUrl           strings.Builder
	paramRoute, paramUrl             [3]strings.Builder
	staticParamRoute, staticParamUrl [3]strings.Builder
	paramStaticRoute, paramStaticUrl [3]strings.Builder
	myRouter                         Router
	// ginRouter                        *gin.Engine
	// beegoRouter                      *beego.ControllerRegister
}

func (t *testBenchmark) Init() {
	t.benchRouteCount = 10
	// /static1.../static_n
	t.staticRoute.WriteString("/static")
	t.staticUrl.WriteString("/static")
	// /param1.../param_n
	t.paramRoute[0].WriteString("/param")
	t.paramUrl[0].WriteString("/param")
	t.paramRoute[1].WriteString("/param")
	t.paramUrl[1].WriteString("/param")
	t.paramRoute[2].WriteString("/param")
	t.paramUrl[2].WriteString("/param")
	// /static1/param1.../static_n/param_n
	t.staticParamRoute[0].WriteString("/static_param")
	t.staticParamUrl[0].WriteString("/static_param")
	t.staticParamRoute[1].WriteString("/static_param")
	t.staticParamUrl[1].WriteString("/static_param")
	t.staticParamRoute[2].WriteString("/static_param")
	t.staticParamUrl[2].WriteString("/static_param")
	// /param1/static1.../param_n/static_n
	t.paramStaticRoute[0].WriteString("/param_static")
	t.paramStaticUrl[0].WriteString("/param_static")
	t.paramStaticRoute[1].WriteString("/param_static")
	t.paramStaticUrl[1].WriteString("/param_static")
	t.paramStaticRoute[2].WriteString("/param_static")
	t.paramStaticUrl[2].WriteString("/param_static")
	for i := 0; i < t.benchRouteCount; i++ {
		// /static1.../static_n
		t.staticRoute.WriteString(fmt.Sprintf("/static%d", i))
		t.staticUrl.WriteString(fmt.Sprintf("/static%d", i))
		// /param1.../param_n
		t.paramRoute[0].WriteString("/:")
		t.paramUrl[0].WriteString(fmt.Sprintf("/param%d", i))
		t.paramRoute[1].WriteString(fmt.Sprintf("/:param%d", i))
		t.paramUrl[1].WriteString(fmt.Sprintf("/param%d", i))
		t.paramRoute[2].WriteString(fmt.Sprintf("/:param%d", i))
		t.paramUrl[2].WriteString(fmt.Sprintf("/param%d", i))
		// /static1/param1.../static_n/param_n
		t.staticParamRoute[0].WriteString(fmt.Sprintf("/static%d/:", i))
		t.staticParamUrl[0].WriteString(fmt.Sprintf("/static%d/param%d", i, i))
		t.staticParamRoute[1].WriteString(fmt.Sprintf("/static%d/:param%d", i, i))
		t.staticParamUrl[1].WriteString(fmt.Sprintf("/static%d/param%d", i, i))
		t.staticParamRoute[2].WriteString(fmt.Sprintf("/static%d/:param%d", i, i))
		t.staticParamUrl[2].WriteString(fmt.Sprintf("/static%d/param%d", i, i))
		// /param1/static1.../param_n/static_n
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
	// // gin
	// gin.SetMode(gin.ReleaseMode)
	// t.ginRouter = gin.New()
	// t.ginRouter.GET(t.staticRoute.String(), func(c *gin.Context) {})
	// t.ginRouter.GET(t.paramRoute[1].String(), func(c *gin.Context) {})
	// t.ginRouter.GET(t.staticParamRoute[1].String(), func(c *gin.Context) {})
	// t.ginRouter.GET(t.paramStaticRoute[1].String(), func(c *gin.Context) {})
	// // beego
	// t.beegoRouter = beego.NewApp().Handlers
	// t.beegoRouter.Get(t.staticRoute.String(), func(c *beego_context.Context) {})
	// t.beegoRouter.Get(t.paramRoute[2].String(), func(c *beego_context.Context) {})
	// t.beegoRouter.Get(t.staticParamRoute[2].String(), func(c *beego_context.Context) {})
	// t.beegoRouter.Get(t.paramStaticRoute[2].String(), func(c *beego_context.Context) {})
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

// Test match
func Test_Benchmark(t *testing.T) {
	var handler testHandler
	testBench := testNewBenchmark()
	testBench.myRouter.SetNotfound(func(c *Context) bool {
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
