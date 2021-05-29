package router

import (
	"fmt"
	"path"
	"strings"
)

// Return different sub string of s1 and s2.
// Example: s1="abc4", s2="abc123" -> "4", "123"
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

// Split static route and param route.
// Example: "/users/:/status" -> ["/users/",":","/status"]
func splitRoute(_path string) ([]string, error) {
	_path = path.Clean(_path)
	// Empty path
	if _path == "" || _path == "/" {
		return []string{"/"}, nil
	}
	// Ignore first '/'
	if _path[0] == '/' {
		_path = _path[1:]
	}
	// Split path.
	part := strings.Split(_path, "/")
	var routePath []string
	var static strings.Builder
	isStatic := true
	for i := 0; i < len(part); i++ {
		switch part[i][0] {
		case ':':
			part[i] = ":"
		case '*':
			part[i] = "*"
		default:
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

// A node of a tree.
type Route struct {
	Handle []HandleFunc
	// Use for remove sub route.
	parent *Route
	// Full path from root.Used for return a error.
	path string
	// Current route path.
	// Example: "/user/","/:int","*"
	name string
	// Static sub routes. 256 spaces for fast indexing.
	static [256]*Route
	// Param sub route. A route can only has one param sub route.
	param *Route
}

// Try to add path to route, return final route or error.
func (r *Route) add(path string) (*Route, error) {
	routePath, err := splitRoute(path)
	if err != nil {
		return nil, err
	}
	// Initial current route.
	if r.name == "" {
		r.name = routePath[0]
		r.path = r.name
		routePath = routePath[1:]
	}
	route := r
	// Add sub route loop.
	for _, name := range routePath {
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

// Try to add a param sub route, return error if r is a all match route,
// or name is no equal to r, or r has static sub route.
func (r *Route) addSubParam(name string) (*Route, error) {
	// r is a all match route.
	if r.name == "*" {
		return nil, fmt.Errorf("can't add '%s' to '%s'", name, r.path)
	}
	// r has a param sub route.
	if r.param != nil {
		if r.param.name != name {
			return nil, fmt.Errorf("can't add '%s' to '%s' has sub param '%s'", name, r.path, r.param.name)
		}
		return r.param, nil
	}
	// r has static sub route.
	for i := 0; i < len(r.static); i++ {
		if r.static[i] != nil {
			return nil, fmt.Errorf("can't add '%s' to '%s' has sub static '%s'", name, r.path, r.static[i].name)
		}
	}
	route := new(Route)
	route.name = name
	route.parent = r
	if r.name[0] == '*' || r.name[0] == ':' {
		route.path = r.path + "/" + name
	} else {
		route.path = r.path + name
	}
	r.param = route
	return route, nil
}

// Try to add static sub route, return error if r is a all match route or r has a param route.
func (r *Route) addSubStatic(name string) (*Route, error) {
	// r is a all match route.
	if r.name == "*" {
		return nil, fmt.Errorf("can't add '%s' to '%s'", name, r.path)
	}
	// r has a param sub route.
	if r.param != nil {
		return nil, fmt.Errorf("can't add '%s' to '%s' has sub param '%s'", name, r.path, r.param.name)
	}
	// Let sub route to handle.
	// Example: r=“abc123" add "abc456", let sub route 'a' to handle "abc456".
	if r.static[name[0]] != nil {
		return r.static[name[0]].addStatic(name)
	}
	// Add
	sub := new(Route)
	sub.name = name
	sub.parent = r
	if r.name == ":" {
		sub.path = r.path + "/" + sub.name
	} else {
		sub.path = r.path + sub.name
	}
	r.static[name[0]] = sub
	return sub, nil
}

// Try to add static path route.
func (r *Route) addStatic(name string) (*Route, error) {
	if r.name == "*" {
		return nil, fmt.Errorf("can't add '%s' to '%s'", name, r.path)
	}
	// case 1, r="/abc", name="/abc"
	if r.name == name {
		return r, nil
	}
	// r is a param route.
	if r.name[0] == ':' {
		return r.addSubStatic(name)
	}
	diff1, diff2 := diffString(r.name, name)
	// case 2, r="/abc123", name="/abc", diff1="123", diff2="", same="/abc".
	// "/abc123"(r) -> "/abc"(new route)
	// 				      |
	//				    "123"(r)
	if diff2 == "" {
		r.path = r.path[:len(r.path)-len(diff1)]
		r.name = r.name[:len(r.name)-len(diff1)]
		// r route become "/abc", copy r's data to “123”.
		handle := r.Handle
		staic := r.static
		param := r.param
		r.Handle = nil
		r.removeAllStatic()
		r.param = nil
		// Add sub "123", copy r's data.
		sub, err := r.addSubStatic(diff1)
		if err != nil {
			return nil, err
		}
		sub.Handle = handle
		sub.static = staic
		sub.param = param
		// Return "/abc"
		return r, nil
	}
	// case 3, r="/abc", name="/abc123", diff1="", diff2="123", same="/abc"
	// "/abc"(r) -> "/abc"(r)
	// 				   |
	//				 "123"(new route)
	if diff1 == "" {
		return r.addSubStatic(diff2)
	}
	// case 4, r="/abc456", name="/abc123", diff1="456", diff2="123", same="/abc"
	// "/abc456"(r) -> "/abc"
	// 				    /   \
	//			    "123"   "456"
	// 			    (new)	 (r)
	r.path = r.path[:len(r.path)-len(diff1)]
	r.name = r.name[:len(r.name)-len(diff1)]
	// r route become "/abc", copy r's data to “456”.
	handle := r.Handle
	staic := r.static
	param := r.param
	r.Handle = nil
	r.removeAllStatic()
	r.param = nil
	// Add sub "456", copy r's data.
	sub, err := r.addSubStatic(diff1)
	if err != nil {
		return nil, err
	}
	sub.Handle = handle
	sub.static = staic
	sub.param = param
	// Return "123"
	return r.addSubStatic(diff2)
}

// Try to remove route path.
func (r *Route) remove(path string) bool {
	route := r.get(path)
	if route == nil {
		return false
	}
	for {
		// Is r
		if route == r {
			r.path = ""
			r.Handle = nil
			r.name = ""
			r.removeAllStatic()
			r.param = nil
			return true
		}
		// Remove from it's parent route.
		parent := route.parent
		if route.name[0] == ':' || route.name[0] == '*' {
			parent.param = nil
		} else {
			parent.static[route.name[0]] = nil
		}
		if len(parent.Handle) > 0 {
			return true
		}
		// Parent has no handlers, if it has only one static sub, join them.
		if parent.name != ":" && parent.name != "*" {
			var static []int
			for i := 0; i < len(parent.static); i++ {
				if parent.static[i] != nil {
					static = append(static, i)
					if len(static) > 1 {
						break
					}
				}
			}
			if len(static) > 1 {
				return true
			}
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
		}
		// If parent has no sub route and no handlers, go on remove parent.
		route = parent
	}
}

// Try to find sub route by path.
func (r *Route) get(path string) *Route {
	routePath, err := splitRoute(path)
	if err != nil {
		return nil
	}
	// Root
	route := r
	name := routePath[0]
	// r must be static route.
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
	// Check sub.
	for _, name := range routePath {
		if name[0] == '*' || name[0] == ':' {
			route = route.param
			if route == nil || route.name != name {
				return nil
			}
			continue
		}
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

// Try to match url path, return the final route or nil.
// param route name will be appended to c.Param.
func (r *Route) match(c *Context) *Route {
	path := c.Req.URL.Path
	route := r
	i := 0
Loop:
	for {
		// Whether current route match the prefix of path.
		if len(route.name) < len(path) {
			// Current route match the prefix of path.
			if path[:len(route.name)] == route.name {
				// Path removes prefix.
				path = path[len(route.name):]
				// Try to match sub routes.
			ParamLoop:
				// If sub route is a param route.
				for route.param != nil {
					// Is a param route.
					if route.param.name[0] == ':' {
						i = 1
						// Find next '/'
						for ; i < len(path); i++ {
							if path[i] == '/' {
								c.Param = append(c.Param, path[:i])
								// Ignore '/'
								path = path[i+1:]
								route = route.param
								continue ParamLoop
							}
						}
					}
					// Is a all match route.
					c.Param = append(c.Param, path)
					return route.param
				}
				// If sub route is a static route.
				if route.static[path[0]] != nil {
					route = route.static[path[0]]
					continue Loop
				}
			}
			// Current route dose not match the prefix of path.
			return nil
		}
		// Current route match the rest of path.
		if path == route.name {
			return route
		}
		// Current route dose not match the rest of path.
		return nil
	}
}

// Remove all static sub routes.
func (r *Route) removeAllStatic() {
	for i := 0; i < len(r.static); i++ {
		r.static[i] = nil
	}
}
