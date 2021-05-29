package router

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	methodGet = iota
	methodHead
	methodPost
	methodPut
	methodPatch
	methodDelete
	methodConnect
	methodOptions
	methodTrace
	methodMax
)

// Match http method and url, useful for web application.
// Route path example:
// Param route "/:", no need to know name, because we know the order.
// All match route "/*", add any path after this route will return error.
// Static route "/users".
type MethodRouter struct {
	// Route table.
	route [methodMax]Route
	// Intercept chain before match route.
	Intercept []HandleFunc
	// NotMatch chain if match route failed.
	NotMatch []HandleFunc
}

// Implements http.Handler
func (r *MethodRouter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	c := contextPool.Get().(*Context)
	c.Req = req
	c.Res = res
	c.Param = c.Param[:0]
	c.Data = nil
	// Intercept chain.
	for i := 0; i < len(r.Intercept); i++ {
		if !r.Intercept[i](c) {
			contextPool.Put(c)
			return
		}
	}
	// Try to match route.
	route := r.root(req.Method)
	if route != nil {
		route = route.match(c)
		if route != nil && len(route.Handle) > 0 {
			for _, h := range route.Handle {
				if !h(c) {
					break
				}
			}
			contextPool.Put(c)
			return
		}
	}
	// No match.
	for i := 0; i < len(r.NotMatch); i++ {
		if !r.NotMatch[i](c) {
			break
		}
	}
	contextPool.Put(c)
}

// Try to add a route.
func (r *MethodRouter) Add(method, path string, handleFunc ...HandleFunc) (*Route, error) {
	root := r.root(method)
	if root == nil {
		return nil, fmt.Errorf("invalid http method '%s'", method)
	}
	return r.add(root, path, handleFunc...)
}

func (r *MethodRouter) AddGet(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodGet], path, handleFunc...)
}

func (r *MethodRouter) AddHead(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodHead], path, handleFunc...)
}

func (r *MethodRouter) AddPost(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodPost], path, handleFunc...)
}

func (r *MethodRouter) AddPut(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodPut], path, handleFunc...)
}

func (r *MethodRouter) AddPatch(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodPatch], path, handleFunc...)
}

func (r *MethodRouter) AddDelete(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodDelete], path, handleFunc...)
}

func (r *MethodRouter) AddConnect(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodConnect], path, handleFunc...)
}

func (r *MethodRouter) AddOptions(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodOptions], path, handleFunc...)
}

func (r *MethodRouter) AddTrace(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodTrace], path, handleFunc...)
}

// Try to add a local static file route handler.
// If file is a directory, it will add all files belong to this directory,
// and file extension in removeFileExt list will be removed.
// Example: "index.html" -> "index".
// If cache is true, use CachaHandler, else use FileHandler.
func (r *MethodRouter) AddStatic(method, route, file string, cache bool, removeFileExt ...string) error {
	fi, err := os.Stat(file)
	if err != nil {
		return err
	}
	// 文件
	if !fi.IsDir() {
		// 静态文件处理
		fi, err := os.Stat(file)
		if err != nil {
			return err
		}
		// 路由路径是否去掉扩展名
		for _, ext := range removeFileExt {
			if ext == "" {
				continue
			}
			if ext[0] != '.' {
				ext = "." + ext
			}
			route = strings.TrimSuffix(route, ext)
		}
		// 是否缓存
		if !cache {
			h := new(FileHandler)
			h.File = route
			_, err = r.Add(method, route, h.Handle)
			return err
		} else {
			d, err := ioutil.ReadFile(file)
			if err != nil {
				return err
			}
			h := new(CacheHandler)
			h.ContentType = mime.TypeByExtension(filepath.Ext(fi.Name()))
			h.ModTime = fi.ModTime()
			h.Data = d
			_, err = r.Add(method, route, h.Handle)
			return err
		}
	}
	// 目录
	fis, err := ioutil.ReadDir(file)
	if err != nil {
		return err
	}
	// 添加所有子文件
	for i := 0; i < len(fis); i++ {
		err = r.AddStatic(method, path.Join(route, fis[i].Name()), filepath.Join(file, fis[i].Name()), cache, removeFileExt...)
		if err != nil {
			return err
		}
	}
	return nil
}

// Try to find Route from method route table by path. Return nil if not found.
func (r *MethodRouter) Route(method, path string) *Route {
	root := r.root(method)
	if root == nil {
		return nil
	}
	return root.get(path)
}

func (r *MethodRouter) RouteGet(path string) *Route {
	return r.route[methodGet].get(path)
}

func (r *MethodRouter) RouteHead(path string) *Route {
	return r.route[methodHead].get(path)
}

func (r *MethodRouter) RoutePost(path string) *Route {
	return r.route[methodPost].get(path)
}

func (r *MethodRouter) RoutePut(path string) *Route {
	return r.route[methodPut].get(path)
}

func (r *MethodRouter) RoutePatch(path string) *Route {
	return r.route[methodPatch].get(path)
}

func (r *MethodRouter) RouteDelete(path string) *Route {
	return r.route[methodDelete].get(path)
}

func (r *MethodRouter) RouteConnect(path string) *Route {
	return r.route[methodConnect].get(path)
}

func (r *MethodRouter) RouteOptions(path string) *Route {
	return r.route[methodOptions].get(path)
}

func (r *MethodRouter) RouteTrace(path string) *Route {
	return r.route[methodTrace].get(path)
}

// Try to remove Route from method route table by path. Return false if not found.
func (r *MethodRouter) Remove(method, path string) bool {
	root := r.root(method)
	if root == nil {
		return false
	}
	return root.remove(path)
}

func (r *MethodRouter) RemoveGet(path string) bool {
	return r.route[methodGet].remove(path)
}

func (r *MethodRouter) RemoveHead(path string) bool {
	return r.route[methodHead].remove(path)
}

func (r *MethodRouter) RemovePost(path string) bool {
	return r.route[methodPost].remove(path)
}

func (r *MethodRouter) RemovePut(path string) bool {
	return r.route[methodPut].remove(path)
}

func (r *MethodRouter) RemovePatch(path string) bool {
	return r.route[methodPatch].remove(path)
}

func (r *MethodRouter) RemoveDelete(path string) bool {
	return r.route[methodDelete].remove(path)
}

func (r *MethodRouter) RemoveConnect(path string) bool {
	return r.route[methodConnect].remove(path)
}

func (r *MethodRouter) RemoveOptions(path string) bool {
	return r.route[methodOptions].remove(path)
}

func (r *MethodRouter) RemoveTrace(path string) bool {
	return r.route[methodTrace].remove(path)
}

// Return root Route from method table.
func (r *MethodRouter) root(method string) *Route {
	if method[0] == 'G' {
		return &r.route[methodGet]
	}
	if method[0] == 'H' {
		return &r.route[methodHead]
	}
	if method[0] == 'D' {
		return &r.route[methodDelete]
	}
	if method[0] == 'C' {
		return &r.route[methodConnect]
	}
	if method[0] == 'O' {
		return &r.route[methodOptions]
	}
	if method[0] == 'T' {
		return &r.route[methodTrace]
	}
	if method[1] == 'O' {
		return &r.route[methodPost]
	}
	if method[1] == 'U' {
		return &r.route[methodPut]
	}
	if method[1] == 'A' {
		return &r.route[methodPatch]
	}
	return nil
}

// Add route path and
func (r *MethodRouter) add(root *Route, path string, handleFunc ...HandleFunc) (*Route, error) {
	route, err := root.add(path)
	if err != nil {
		return nil, err
	}
	route.Handle = handleFunc
	return route, nil
}

// Match http url, useful for api gateway.
type PathRouter struct {
	route     Route
	Intercept []HandleFunc
	NotMatch  []HandleFunc
}

func (r *PathRouter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	c := contextPool.Get().(*Context)
	c.Req = req
	c.Res = res
	c.Param = c.Param[:0]
	c.Data = nil
	// Intercept chain.
	for i := 0; i < len(r.Intercept); i++ {
		if !r.Intercept[i](c) {
			contextPool.Put(c)
			return
		}
	}
	// Try to match route.
	route := r.route.match(c)
	if route != nil && len(route.Handle) > 0 {
		for _, h := range route.Handle {
			if !h(c) {
				break
			}
		}
		contextPool.Put(c)
		return
	}
	// Not match
	for i := 0; i < len(r.NotMatch); i++ {
		if !r.NotMatch[i](c) {
			break
		}
	}
	contextPool.Put(c)
}

// Try to add a route.
func (r *PathRouter) Add(path string, handleFunc ...HandleFunc) (*Route, error) {
	route, err := r.route.add(path)
	if err != nil {
		return nil, err
	}
	route.Handle = handleFunc
	return route, nil
}

// Try to return a Route.
func (r *PathRouter) Route(path string) *Route {
	return r.route.get(path)
}

// Try to remove a Route.
func (r *PathRouter) Remove(path string) bool {
	return r.route.remove(path)
}
