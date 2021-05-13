/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 15:04
     Project: dmctl
     Package: controller
    Describe: Todo
*/
package frame

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"net/http"
)

type HandlerFunc func(*gin.Context) (*Response, *Error)

func handlerWrapper(wrappers []HandlerFunc) (handlers []gin.HandlerFunc) {
	for _, wrapper := range wrappers {
		handlers = append(handlers, func(context *gin.Context) {
			response, err := wrapper(context)
			if err != nil {
				klog.Errorf("frame default error handling: %v", err)
				context.JSON(err.httpCode, gin.H{"message": fmt.Sprintf("%v", err)})
			} else {
				if response != nil {
					context.JSON(response.httpCode, response.payload)
				} else {
					context.JSON(http.StatusOK, gin.H{})
				}
			}
		})
	}
	return handlers
}

type DmEngine struct {
	*gin.Engine
}

func NewDmEngine() *DmEngine {
	return &DmEngine{gin.Default()}
}

func (engine *DmEngine) GET(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.GET(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) POST(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.POST(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) DELETE(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.DELETE(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) PATCH(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.PATCH(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) PUT(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.PUT(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) OPTIONS(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.OPTIONS(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) HEAD(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.HEAD(relativePath, handlerWrapper(handlers)...)
}

func (engine *DmEngine) Any(relativePath string, handlers ...HandlerFunc) {
	engine.Engine.Any(relativePath, handlerWrapper(handlers)...)
}
