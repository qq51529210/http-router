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

type HandleFunc func(*Context) bool

// Notfound response status code 404.
func Notfound(c *Context) bool {
	c.Res.WriteHeader(http.StatusNotFound)
	return true
}

// Match http method and url path.
// Route path example:
// Param route: "/:", no need to know name, because we know the order.
// All match route: "/*", add any path after this route will return error.
// Static route: "/users".
// Chain cases:
// before -> after
// before -> notfound -> after
// before -> handler -> after
type Router struct {
	// Root route table.
	// 0=get, 1=head, 2=delete, 3=connect, 4=options,
	// 5=trace, 6=post, 7=put, 8=patch
	rootRoute [9]rootRoute
	// Called before match.
	before []HandleFunc
	// Called if not match.
	notfound []HandleFunc
	// Called anyway.
	after []HandleFunc
}

func (r *Router) SetBefore(handleFunc ...HandleFunc) {
	r.before = handleFunc
}

func (r *Router) SetNotfound(handleFunc ...HandleFunc) {
	r.notfound = handleFunc
}

func (r *Router) SetAfter(handleFunc ...HandleFunc) {
	r.after = handleFunc
}

// Implements http.Handler
func (r *Router) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	c := contextPool.Get().(*Context)
	c.Req = req
	c.Res = res
	c.Param = c.Param[:0]
	c.Data = nil
	// Before.
	for _, h := range r.before {
		if !h(c) {
			// After.
			for _, h := range r.after {
				if !h(c) {
					break
				}
			}
			return
		}
	}
	// Try to match route.
	rootRoute := r.root(req.Method)
	if rootRoute != nil {
		route := rootRoute.Match(c)
		if route != nil && len(route.Handle) > 0 {
			// Handler.
			for _, h := range route.Handle {
				if !h(c) {
					break
				}
			}
			// After.
			for _, h := range r.after {
				if !h(c) {
					break
				}
			}
			return
		}
	}
	// Notfound.
	for _, h := range r.notfound {
		if !h(c) {
			break
		}
	}
	// After.
	for _, h := range r.after {
		if !h(c) {
			break
		}
	}
}

// Try to add a route.
func (r *Router) Add(method, path string, handleFunc ...HandleFunc) (*Route, error) {
	root := r.root(method)
	if root == nil {
		return nil, fmt.Errorf("invalid http method '%s'", method)
	}
	route, err := root.Add(path)
	if err != nil {
		return nil, err
	}
	route.Handle = handleFunc
	return route, nil
}

func (r *Router) AddGet(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodGet, path, handleFunc...)
}

func (r *Router) AddHead(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodHead, path, handleFunc...)
}

func (r *Router) AddPost(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodPost, path, handleFunc...)
}

func (r *Router) AddPut(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodPut, path, handleFunc...)
}

func (r *Router) AddPatch(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodPatch, path, handleFunc...)
}

func (r *Router) AddDelete(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodDelete, path, handleFunc...)
}

func (r *Router) AddConnect(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodConnect, path, handleFunc...)
}

func (r *Router) AddOptions(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodOptions, path, handleFunc...)
}

func (r *Router) AddTrace(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.Add(http.MethodTrace, path, handleFunc...)
}

// Try to add a local static file route handler.
// If file is a directory, it will add all files belong to this directory,
// and file extension in removeFileExt list will be removed.
// Example: "index.html" -> "index".
// If cache is true, use CachaHandler, else use FileHandler.
func (r *Router) AddStatic(method, route, file string, cache bool, removeFileExt ...string) error {
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
func (r *Router) Route(method, path string) *Route {
	root := r.root(method)
	if root == nil {
		return nil
	}
	return root.Find(path)
}

func (r *Router) RouteGet(path string) *Route {
	return r.Route(http.MethodGet, path)
}

func (r *Router) RouteHead(path string) *Route {
	return r.Route(http.MethodHead, path)
}

func (r *Router) RoutePost(path string) *Route {
	return r.Route(http.MethodPost, path)
}

func (r *Router) RoutePut(path string) *Route {
	return r.Route(http.MethodPut, path)
}

func (r *Router) RoutePatch(path string) *Route {
	return r.Route(http.MethodPatch, path)
}

func (r *Router) RouteDelete(path string) *Route {
	return r.Route(http.MethodDelete, path)
}

func (r *Router) RouteConnect(path string) *Route {
	return r.Route(http.MethodConnect, path)
}

func (r *Router) RouteOptions(path string) *Route {
	return r.Route(http.MethodOptions, path)
}

func (r *Router) RouteTrace(path string) *Route {
	return r.Route(http.MethodTrace, path)
}

// Try to remove Route from method route table by path. Return false if not found.
func (r *Router) Remove(method, path string) bool {
	root := r.root(method)
	if root == nil {
		return false
	}
	return root.Remove(path)
}

func (r *Router) RemoveGet(path string) bool {
	return r.Remove(http.MethodGet, path)
}

func (r *Router) RemoveHead(path string) bool {
	return r.Remove(http.MethodHead, path)
}

func (r *Router) RemovePost(path string) bool {
	return r.Remove(http.MethodPost, path)
}

func (r *Router) RemovePut(path string) bool {
	return r.Remove(http.MethodPut, path)
}

func (r *Router) RemovePatch(path string) bool {
	return r.Remove(http.MethodPatch, path)
}

func (r *Router) RemoveDelete(path string) bool {
	return r.Remove(http.MethodDelete, path)
}

func (r *Router) RemoveConnect(path string) bool {
	return r.Remove(http.MethodConnect, path)
}

func (r *Router) RemoveOptions(path string) bool {
	return r.Remove(http.MethodOptions, path)
}

func (r *Router) RemoveTrace(path string) bool {
	return r.Remove(http.MethodTrace, path)
}

// Return root Route from method table.
func (r *Router) root(method string) *rootRoute {
	if method[0] == 'G' {
		return &r.rootRoute[0]
	}
	if method[0] == 'H' {
		return &r.rootRoute[1]
	}
	if method[0] == 'D' {
		return &r.rootRoute[2]
	}
	if method[0] == 'C' {
		return &r.rootRoute[3]
	}
	if method[0] == 'O' {
		return &r.rootRoute[4]
	}
	if method[0] == 'T' {
		return &r.rootRoute[5]
	}
	if method[1] == 'O' {
		return &r.rootRoute[6]
	}
	if method[1] == 'U' {
		return &r.rootRoute[7]
	}
	if method[1] == 'A' {
		return &r.rootRoute[8]
	}
	return nil
}
