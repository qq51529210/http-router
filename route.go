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

// A route of a route tree.
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

func (r *Route) add(name string) *Route {
	sub := new(Route)
	sub.name = name
	sub.parent = r
	if r.name == "*" || r.name == ":" {
		sub.path = r.path + "/" + name
	} else {
		sub.path = r.path + name
	}
	return sub
}

// Try to add a param sub route to r, it returns error in these cases:
// name is no equal to r's name and r has static sub route.
func (r *Route) addSubParam(name string) (*Route, error) {
	// Case 1, r has a param sub route, name must equal to this sub route's name.
	if r.param != nil {
		if r.param.name != name {
			return nil, fmt.Errorf("%s has a param sub route %s, add sub route %s failed", r.path, r.param.name, name)
		}
		return r.param, nil
	}
	// Case 2, r has static sub route.
	for i := 0; i < len(r.static); i++ {
		if r.static[i] != nil {
			return nil, fmt.Errorf("%s has a static sub route %s, add sub route %s failed", r.path, r.static[i].name, name)
		}
	}
	// Add param sub route.
	r.param = r.add(name)
	return r.param, nil
}

// Try to add a static sub route to r, return error if r is a all match route or r has a param route.
func (r *Route) addSubStatic(name string) (*Route, error) {
	// r has a param route.
	if r.param != nil {
		return nil, fmt.Errorf("%s has a param sub route %s, add sub route %s failed", r.path, r.param.name, name)
	}
	// Let sub route to handle.
	if r.static[name[0]] != nil {
		return r.static[name[0]].addStatic(name)
	}
	r.static[name[0]] = r.add(name)
	return r.static[name[0]], nil
}

// Try to add a static path to r, it returns error if r has a param route.
func (r *Route) addStatic(name string) (*Route, error) {
	// r is a param route.
	if r.name == ":" {
		return r.addSubStatic(name)
	}
	// Add case 1, r.name="/abc", name="/abc".
	if r.name == name {
		return r, nil
	}
	diff1, diff2 := diffString(r.name, name)
	// Add case 2, r.name="/abc", name="/ab", diff1="c", diff2="".
	// New: /ab(name) -> c(r).
	if diff2 == "" {
		err := r.moveToNewSub(diff1)
		if err != nil {
			return nil, err
		}
		// Return "/ab".
		return r, nil
	}
	// Add case 3, r.name="/ab", name="/abc", diff1="", diff2="c".
	// New: /ab(r) -> c(name).
	if diff1 == "" {
		if r.static[diff2[0]] != nil {
			return r.static[diff2[0]].addStatic(diff2)
		}
		r.static[diff2[0]] = r.add(diff2)
		return r.static[diff2[0]], nil
	}
	// Add case 4, r.name="/abc", name="/abd", diff1="c", diff2="d".
	//  		-> c(r).
	// New: /ab
	// 			-> d(name).
	err := r.moveToNewSub(diff1)
	if err != nil {
		return nil, err
	}
	// Return "d".
	return r.addSubStatic(diff2)
}

// Add a new sub route, copy r's data to new route.
func (r *Route) moveToNewSub(name string) error {
	// Save r's data.
	handle := r.Handle
	staic := r.static
	param := r.param
	// Modify r's data.
	r.path = r.path[:len(r.path)-len(name)]
	r.name = r.name[:len(r.name)-len(name)]
	r.Handle = nil
	r.removeAllStatic()
	r.param = nil
	// Add a new static route.
	sub, err := r.addSubStatic(name)
	if err != nil {
		return err
	}
	sub.Handle = handle
	sub.static = staic
	sub.param = param
	return nil
}

func (r *Route) removeAllStatic() {
	for i := 0; i < len(r.static); i++ {
		r.static[i] = nil
	}
}

// Root route of a route tree.
type rootRoute struct {
	route Route
}

// Try to add a route by path.
func (r *rootRoute) Add(path string) (*Route, error) {
	// Split path into static and param routes.
	routePath, err := splitRoute(path)
	if err != nil {
		return nil, err
	}
	route := &r.route
	// Initialize root route.
	if route.name == "" {
		route.name = routePath[0]
		route.path = route.name
		routePath = routePath[1:]
	}
	// Add sub route loop.
	for _, name := range routePath {
		// Route is a all match route, can not add sub route.
		if route.name == "*" {
			return nil, fmt.Errorf("%s is a all match route, add sub route %s failed", route.path, name)
		}
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

// Try to find route by path.
func (r *rootRoute) Find(path string) *Route {
	// Split path into static and param routes.
	routePath, err := splitRoute(path)
	if err != nil {
		return nil
	}
	// Root
	route := &r.route
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

// Try to remove route by path.
// If success, it will go on remove the route's parent if its parent has no sub route and no handlers.
func (r *rootRoute) Remove(path string) bool {
	// Find the route.
	route := r.Find(path)
	if route == nil {
		return false
	}
	for {
		// Reset root route.
		if route == &r.route {
			route.path = ""
			route.Handle = nil
			route.name = ""
			route.removeAllStatic()
			route.param = nil
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

// Try to match path, return the final route and value of param route.
// Value of param route will append to param and return.
func (r *rootRoute) Match(c *Context) *Route {
	path := c.Req.URL.Path
	route := &r.route
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
					if route.param.name == ":" {
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
						// Can not find '/', it's the end.
						c.Param = append(c.Param, path)
						route = route.param
						return route
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
