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

// 匹配方法和路径的路由
type MethodRouter struct {
	route       [methodMax]Route // 路由
	interceptor []HandleFunc     // 匹配前的拦截器
	notMatch    []HandleFunc     // 匹配失败
}

// 设置全局拦截函数
func (r *MethodRouter) SetInterceptor(handleFunc ...HandleFunc) {
	r.interceptor = filteHandleFunc(handleFunc...)
}

// 设置匹配失败处理函数
func (r *MethodRouter) SetNotMatch(handleFunc ...HandleFunc) {
	r.notMatch = filteHandleFunc(handleFunc...)
}

// 实现http.Handler接口
func (r *MethodRouter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// 上下文
	c := contextPool.Get().(*Context)
	c.Request = req
	c.Response = res
	c.Param = c.Param[:0]
	c.UserData = nil
	// 拦截
	for i := 0; i < len(r.interceptor); i++ {
		if !r.interceptor[i](c) {
			contextPool.Put(c)
			return
		}
	}
	// 匹配
	route := r.root(req.Method)
	if route != nil {
		route = route.match(c)
		// 匹配成功
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
	// 未匹配到
	for i := 0; i < len(r.notMatch); i++ {
		if !r.notMatch[i](c) {
			break
		}
	}
	contextPool.Put(c)
}

// 添加路由，method是http方法，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) Add(method, path string, handleFunc ...HandleFunc) (*Route, error) {
	root := r.root(method)
	if root == nil {
		return nil, fmt.Errorf("invalid http method '%s'", method)
	}
	return r.add(root, path, handleFunc...)
}

// 添加get方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddGet(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodGet], path, handleFunc...)
}

// 添加head方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddHead(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodHead], path, handleFunc...)
}

// 添加post方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddPost(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodPost], path, handleFunc...)
}

// 添加put方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddPut(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodPut], path, handleFunc...)
}

// 添加patch方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddPatch(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodPatch], path, handleFunc...)
}

// 添加delete方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddDelete(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodDelete], path, handleFunc...)
}

// 添加connect方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddConnect(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodConnect], path, handleFunc...)
}

// 添加options方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddOptions(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodOptions], path, handleFunc...)
}

// 添加trace方法的路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *MethodRouter) AddTrace(path string, handleFunc ...HandleFunc) (*Route, error) {
	return r.add(&r.route[methodTrace], path, handleFunc...)
}

// 添加路由处理静态文件，如果file是文件，路由path为route，如果是目录，则route是路由根目录。
// 内部使用的是FileHandler和CachaHandler处理。
// method是http方法。
// file是文件的本地路径，如果是目录，则循环加载目录下所有文件。
// cache是否将文件加载到内存。
// removeFileExt是否去掉文件后缀名（file是目录时有效），比如，index.html，生成的路由是index。
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

// 获取路由，返回nil表示找不到，method是http方法，path是路由路径
func (r *MethodRouter) Route(method, path string) *Route {
	root := r.root(method)
	if root == nil {
		return nil
	}
	return root.get(path)
}

// 获取get方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RouteGet(path string) *Route {
	return r.route[methodGet].get(path)
}

// 获取head方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RouteHead(path string) *Route {
	return r.route[methodHead].get(path)
}

// 获取post方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RoutePost(path string) *Route {
	return r.route[methodPost].get(path)
}

// 获取put方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RoutePut(path string) *Route {
	return r.route[methodPut].get(path)
}

// 获取patch方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RoutePatch(path string) *Route {
	return r.route[methodPatch].get(path)
}

// 获取delete方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RouteDelete(path string) *Route {
	return r.route[methodDelete].get(path)
}

// 获取connect方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RouteConnect(path string) *Route {
	return r.route[methodConnect].get(path)
}

// 获取options方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RouteOptions(path string) *Route {
	return r.route[methodOptions].get(path)
}

// 获取trace方法的路由，返回nil表示找不到，path是路由路径
func (r *MethodRouter) RouteTrace(path string) *Route {
	return r.route[methodTrace].get(path)
}

// 移除路由，成功返回true，method是http方法，path是路由路径
func (r *MethodRouter) Remove(method, path string) bool {
	root := r.root(method)
	if root == nil {
		return false
	}
	return root.remove(path)
}

// 移除get方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemoveGet(path string) bool {
	return r.route[methodGet].remove(path)
}

// 移除head方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemoveHead(path string) bool {
	return r.route[methodHead].remove(path)
}

// 移除post方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemovePost(path string) bool {
	return r.route[methodPost].remove(path)
}

// 移除put方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemovePut(path string) bool {
	return r.route[methodPut].remove(path)
}

// 移除patch方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemovePatch(path string) bool {
	return r.route[methodPatch].remove(path)
}

// 移除delete方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemoveDelete(path string) bool {
	return r.route[methodDelete].remove(path)
}

// 移除connect方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemoveConnect(path string) bool {
	return r.route[methodConnect].remove(path)
}

// 移除options方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemoveOptions(path string) bool {
	return r.route[methodOptions].remove(path)
}

// 移除trace方法的路由，成功返回true，path是路由路径
func (r *MethodRouter) RemoveTrace(path string) bool {
	return r.route[methodTrace].remove(path)
}

// 获取method的根路由
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

func (r *MethodRouter) add(root *Route, path string, handleFunc ...HandleFunc) (*Route, error) {
	route, err := root.add(path)
	if err != nil {
		return nil, err
	}
	route.Handle = handleFunc
	return route, nil
}

// 匹配路径的路由，可以用作api网关
type PathRouter struct {
	route       Route        // 路由
	interceptor []HandleFunc // 匹配前的拦截器
	notMatch    []HandleFunc // 匹配失败
}

// 设置全局拦截函数
func (r *PathRouter) SetInterceptor(handleFunc ...HandleFunc) {
	r.interceptor = filteHandleFunc(handleFunc...)
}

// 设置匹配失败处理函数
func (r *PathRouter) SetNotMatch(handleFunc ...HandleFunc) {
	r.notMatch = filteHandleFunc(handleFunc...)
}

// 实现http.Handler接口
func (r *PathRouter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// 上下文
	c := contextPool.Get().(*Context)
	c.Request = req
	c.Response = res
	c.Param = c.Param[:0]
	c.UserData = nil
	// 拦截
	for i := 0; i < len(r.interceptor); i++ {
		if !r.interceptor[i](c) {
			contextPool.Put(c)
			return
		}
	}
	// 匹配
	route := r.route.match(c)
	// 匹配成功
	if route != nil && len(route.Handle) > 0 {
		for _, h := range route.Handle {
			if !h(c) {
				break
			}
		}
		contextPool.Put(c)
		return
	}
	// 未匹配到
	for i := 0; i < len(r.notMatch); i++ {
		if !r.notMatch[i](c) {
			break
		}
	}
	contextPool.Put(c)
}

// 添加路由，path是路由路径，handleFunc是匹配后的回调函数
func (r *PathRouter) Add(path string, handleFunc ...HandleFunc) (*Route, error) {
	route, err := r.route.add(path)
	if err != nil {
		return nil, err
	}
	route.Handle = handleFunc
	return route, nil
}

// 获取路由，返回nil表示找不到，path是路由路径
func (r *PathRouter) Route(path string) *Route {
	return r.route.get(path)
}

// 移除路由，成功返回true，path是路由路径
func (r *PathRouter) Remove(path string) bool {
	return r.route.remove(path)
}
