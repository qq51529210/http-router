package router

import (
	"net/http"
	"sync"
)

var (
	contextPool sync.Pool // context池
)

// 初始化Context缓存
func init() {
	contextPool.New = func() interface{} {
		c := new(Context)
		return c
	}
}

// 路由匹配的上下文数据，HandleFunc的参数
type Context struct {
	Request  *http.Request       // http.ServeHTTP接口参数
	Response http.ResponseWriter // http.ServeHTTP接口参数
	Param    []string            // 参数路由的值
	UserData interface{}         // 应用有时候需要在调用链中传递一些数据
}
