package router

import (
	"fmt"
	"path"
	"strings"
)

// 返回s1和s2不同的字符串，区分大小写，比如，s1="abc4"，s2="abc123"，返回diff1"4"，diff2"123"，same"abc"。
func diffString(s1, s2 string) (string, string) {
	if s1 == "" || s2 == "" {
		return s1, s2
	}
	n := len(s1)
	if n > len(s2) {
		n = len(s2)
	}
	i := 0
	for ; i < n; i++ {
		if s1[i] != s2[i] {
			break
		}
	}
	return s1[i:], s2[i:]
}

// 区分静态路由和参数路由，比如，"/users/:/status"， 返回["/users/",":","/status"]
func splitRoute(_path string) ([]string, error) {
	_path = path.Clean(_path)
	if _path == "" || _path == "/" {
		return []string{"/"}, nil
	}
	if _path[0] == '/' {
		_path = _path[1:]
	}
	// 先分开每个目录
	part := strings.Split(_path, "/")
	var routePath []string
	var static strings.Builder
	isStatic := true
	// 检查每一层目录
	for i := 0; i < len(part); i++ {
		switch part[i][0] {
		case ':': // 参数
			part[i] = ":"
		case '*': // 全匹配
			part[i] = "*"
		default: // 静态
			if isStatic {
				static.WriteByte('/')
			} else {
				isStatic = true
			}
			static.WriteString(part[i])
			continue
		}
		if isStatic {
			static.WriteByte('/')
			isStatic = false
		}
		if static.Len() > 0 {
			routePath = append(routePath, static.String())
			static.Reset()
		}
		routePath = append(routePath, part[i])
	}
	if static.Len() > 0 {
		routePath = append(routePath, static.String())
	}
	return routePath, nil
}

// 表示一层路由
type Route struct {
	Handle []HandleFunc // 处理函数
	parent *Route       // 父路由，移除子路由时用到
	path   string       // 从根到当前路由的全路径，返回错误时用到
	name   string       // 当前路由的路径，比如，"/user/"或者"/:int"
	static [256]*Route  // 静态子路由节点，匹配的时候，无需遍历，快速索引到相应的子路由，空间换时间（可能只用到一个，剩余255是多余的）。
	param  *Route       // 动态子路由节点，由name[0]决定是":"或"*"
}

// 添加路由，path是路径。
func (r *Route) add(path string) (*Route, error) {
	// 区分静态和动态路由
	routePath, err := splitRoute(path)
	if err != nil {
		return nil, err
	}
	// 初始化根路由
	if r.name == "" {
		r.name = routePath[0]
		r.path = r.name
		routePath = routePath[1:]
	}
	route := r
	// 添加路由
	for _, name := range routePath {
		// 当前路由是全匹配，不能添加
		if name == ":" || name == "*" {
			route, err = route.addSubParam(name)
		} else {
			route, err = route.addStatic(name)
		}
		if err != nil {
			return nil, err
		}
	}
	return route, nil
}

// 添加参数子路由
func (r *Route) addSubParam(name string) (*Route, error) {
	if r.name == "*" {
		return nil, fmt.Errorf("can't add '%s' to '%s'", name, r.path)
	}
	// 当前路由有参数子路由
	if r.param != nil {
		if r.param.name != name {
			return nil, fmt.Errorf("can't add '%s' to '%s' has sub param '%s'", name, r.path, r.param.name)
		}
		return r.param, nil
	}
	// 当前路由有静态子路由，不能添加
	for i := 0; i < len(r.static); i++ {
		if r.static[i] != nil {
			return nil, fmt.Errorf("can't add '%s' to '%s' has sub static '%s'", name, r.path, r.static[i].name)
		}
	}
	// 添加
	route := new(Route)
	route.name = name
	route.parent = r
	if r.name[0] == '*' || r.name[0] == ':' {
		// 当前路由是参数，那么添加一个'/'
		route.path = r.path + "/" + name
	} else {
		// 当前路由是静态，那么就是'/'结尾的
		route.path = r.path + name
	}
	r.param = route
	return route, nil
}

// 添加静态子路由
func (r *Route) addSubStatic(name string) (*Route, error) {
	// 当前路由有参数子路由，不能添加
	if r.param != nil {
		return nil, fmt.Errorf("can't add '%s' to '%s' has sub param '%s'", name, r.path, r.param.name)
	}
	// 当前路由有相同的静态子路由，比如，“abc123"添加"abc456"，则让子路由去处理。
	if r.static[name[0]] != nil {
		return r.static[name[0]].addStatic(name)
	}
	// 添加
	sub := new(Route)
	sub.name = name
	sub.parent = r
	if r.name == ":" {
		// 当前路由是参数，那么添加一个'/'
		sub.path = r.path + "/" + sub.name
	} else {
		// 当前路由是静态
		sub.path = r.path + sub.name
	}
	r.static[name[0]] = sub
	return sub, nil
}

// 添加静态路由，匹配和分裂
func (r *Route) addStatic(name string) (*Route, error) {
	if r.name == "*" {
		return nil, fmt.Errorf("can't add '%s' to '%s'", name, r.path)
	}
	// case 1，r.name="/abc"，name="/abc"
	if r.name == name {
		return r, nil
	}
	// 当前路径是参数
	if r.name[0] == ':' {
		return r.addSubStatic(name)
	}
	// 区别路径
	diff1, diff2 := diffString(r.name, name)
	// case 2，r.name="/abc123"，name="/abc"，diff1="123"，diff2=""，same="/abc"
	if diff2 == "" {
		r.path = r.path[:len(r.path)-len(diff1)]
		r.name = r.name[:len(r.name)-len(diff1)]
		// 保存r，转移到子路由“123”
		handle := r.Handle
		staic := r.static
		param := r.param
		// 重置r，r变成公共父路由"/abc"
		r.Handle = nil
		r.removeAllStatic()
		r.param = nil
		// 添加子路由"123"，即原来的r
		sub, err := r.addSubStatic(diff1)
		if err != nil {
			return nil, err
		}
		sub.Handle = handle
		sub.static = staic
		sub.param = param
		// 添加的是"/abc"，返回r
		return r, nil
	}
	// case 3，r.name="/abc"，name="/abc123"，diff1=""，diff2="123"，same="/abc"
	if diff1 == "" {
		// r添加子路由"123"
		return r.addSubStatic(diff2)
	}
	// case 4，r.name="/abc456"，name="/abc123"，diff1="456"，diff2="123"，same="/abc"
	r.path = r.path[:len(r.path)-len(diff1)]
	r.name = r.name[:len(r.name)-len(diff1)]
	// 保存r，转移到子路由"456"
	handle := r.Handle
	staic := r.static
	param := r.param
	// 重置r，r变成公共父路由"/abc"
	r.Handle = nil
	r.removeAllStatic()
	r.param = nil
	// 子路由"456"，即原来的r
	sub, err := r.addSubStatic(diff1)
	if err != nil {
		return nil, err
	}
	sub.Handle = handle
	sub.static = staic
	sub.param = param
	// 子路由"123"，新添加的
	return r.addSubStatic(diff2)
}

// 移除子路由，path是路径
func (r *Route) remove(path string) bool {
	// 获取Route
	route := r.get(path)
	// 没有返回false
	if route == nil {
		return false
	}
	for {
		// 是本路由
		if route == r {
			r.path = ""
			r.Handle = nil
			r.name = ""
			r.removeAllStatic()
			r.param = nil
			return true
		}
		// 从父路由中删除
		parent := route.parent
		if route.name[0] == ':' || route.name[0] == '*' {
			// 要删除的是参数路由
			parent.param = nil
		} else {
			// 要删除的是静态路由
			parent.static[route.name[0]] = nil
		}
		// 如果parent移除route后，没有handle，并且没有子路由，一并删除
		if len(parent.Handle) > 0 || parent.param != nil {
			return true
		}
		// 静态子路由，只有一个静态子路由，合并到当前路由
		var static []int
		for i := 0; i < len(parent.static); i++ {
			if parent.static[i] != nil {
				static = append(static, i)
				if len(static) > 1 {
					break
				}
			}
		}
		// 多个静态
		if len(static) > 1 {
			return true
		}
		// 只有一个静态子路由，合并到当前路由
		if len(static) == 1 {
			sub := route.static[static[0]]
			route.path = sub.path
			route.Handle = sub.Handle
			route.name += sub.name
			if sub.param != nil {
				sub.param.parent = route
			} else {
				for i := 0; i < len(sub.static); i++ {
					route.static[i] = sub.static[i]
					if route.static[i] != nil {
						route.static[i].parent = route
					}
				}
			}
		}
		// 继续向上移除
		route = parent
	}
}

// 获取子路由，path是路径
func (r *Route) get(path string) *Route {
	// 区分静态和动态路由
	routePath, err := splitRoute(path)
	if err != nil {
		return nil
	}
	// 根路由
	route := r
	name := routePath[0]
	// 静态路由
	for {
		if route == nil || len(route.name) > len(name) || route.name != name[:len(route.name)] {
			return nil
		}
		name = name[len(route.name):]
		if name == "" {
			break
		}
		route = route.static[name[0]]
	}
	routePath = routePath[1:]
	// 子路由
	for _, name := range routePath {
		// 参数路由
		if name[0] == '*' || name[0] == ':' {
			route = route.param
			if route == nil || route.name != name {
				return nil
			}
			continue
		}
		// 静态路由
		for {
			route = route.static[name[0]]
			if route == nil || len(route.name) > len(name) || route.name != name[:len(route.name)] {
				return nil
			}
			name = name[len(route.name):]
			if name == "" {
				break
			}
		}
	}
	return route
}

// 匹配c.Request.Url.Path，同时将参数路由的值添加到c.Param，返回最终的Route或者nil
func (r *Route) match(c *Context) *Route {
	// 当前匹配的路径
	path := c.Request.URL.Path
	// 当前匹配的路由
	route := r
	i := 0
Loop:
	for {
		// 当前路由是匹配路径短
		if len(route.name) < len(path) {
			// 匹配当前路由
			if path[:len(route.name)] == route.name {
				// 剩下的path
				path = path[len(route.name):]
				// 匹配子路由
			ParamLoop:
				for route.param != nil {
					// 子路由是参数路由
					if route.param.name[0] == ':' {
						i = 1
						// 找到下一个'/'
						for ; i < len(path); i++ {
							if path[i] == '/' {
								c.Param = append(c.Param, path[:i])
								// 略过'/'，因为没有意义
								path = path[i+1:]
								route = route.param
								continue ParamLoop
							}
						}
					}
					// 参数路由最后一层目录，或者子路由是全匹配路由
					c.Param = append(c.Param, path)
					return route.param
				}
				// 子路由是静态
				if route.static[path[0]] != nil {
					route = route.static[path[0]]
					continue Loop
				}
			}
			// 匹配失败
			return nil
		}
		// 当前路由等于匹配路径
		if path == route.name {
			return route
		}
		// 当前路由比匹配路径长，不匹配
		return nil
	}
}

// 移除所有的静态子路由
func (r *Route) removeAllStatic() {
	for i := 0; i < len(r.static); i++ {
		r.static[i] = nil
	}
}
