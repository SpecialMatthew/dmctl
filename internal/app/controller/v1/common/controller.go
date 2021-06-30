/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 15:29
     Project: dmctl
     Package: start
    Describe: Todo
*/

package common

import (
	"dmctl/internal/pkg/business/v1/common"
	"dmctl/internal/pkg/business/v1/common/typed"
	"dmctl/internal/pkg/frame"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	service common.Interface
}

func NewController(service common.Interface) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(engine *frame.DmEngine) {
	engine.POST("/dmctl/v1/dmserver/start", controller.dmserverStart)
	engine.POST("/dmctl/v1/init", controller.init)
	engine.POST("/dmctl/v1/config/:configModel", controller.config)
	engine.POST("/dmctl/v1/execSql", controller.execSql)
	engine.GET("/dmctl/v1/dmserver/pause", controller.dmserverPause)
	engine.GET("/dmctl/v1/dmserver/restart", controller.dmserverRestart)
	engine.GET("/dmctl/v1/listenPort/:type/:port", controller.listenPort)
	engine.GET("/dmctl/v1/dmap/start", controller.dmapStart)
	engine.GET("/dmctl/v1/dmap/quit", controller.dmapQuit)
	engine.GET("/dmctl/v1/dmwatcher/start", controller.dmwatcherStart)
	engine.GET("/dmctl/v1/dmwatcher/quit", controller.dmwatcherQuit)
	engine.GET("/dmctl/ping/:addr", controller.ping)
	engine.GET("/dmctl/v1/dmserver/status", controller.status)
}

func (controller Controller) dmserverStart(context *gin.Context) (*frame.Response, *frame.Error) {
	var params map[string]interface{}
	if err := context.Bind(&params); err != nil {
		return nil, frame.BadRequestError(err)
	}
	controller.service.DmserverStart(context, params)
	return frame.OK("开始启动数据库..."), nil
}

func (controller Controller) dmserverPause(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.DmserverPause(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("关闭dmwatcher&dmserver..."), nil
}

func (controller Controller) dmserverRestart(context *gin.Context) (*frame.Response, *frame.Error) {
	var params map[string]interface{}
	if err := context.Bind(&params); err != nil {
		return nil, frame.BadRequestError(err)
	}
	err := controller.service.DmserverRestart(context, params)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("重启dmserver..."), nil
}

func (controller Controller) init(context *gin.Context) (*frame.Response, *frame.Error) {
	var params map[string]interface{}
	if err := context.Bind(&params); err != nil {
		return nil, frame.BadRequestError(err)
	}
	err := controller.service.DmInit(context, params)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("初始化成功"), nil
}

func (controller Controller) config(context *gin.Context) (*frame.Response, *frame.Error) {
	var params map[string]*typed.ConfigValue
	if err := context.Bind(&params); err != nil {
		return nil, frame.BadRequestError(err)
	}
	err := controller.service.Config(context, params, context.Query("configModel"))
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("修改dm.ini成功"), nil
}

func (controller Controller) execSql(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.ExecSql(context, "")
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("执行sql脚本成功"), nil
}

func (controller Controller) listenPort(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.ListenPort(context, context.Param("type"), "localhost", context.Param("port"))
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("开始监听端口:" + context.Param("port")), nil
}

func (controller Controller) dmapStart(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.DmapStart(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("开始启动dmap..."), nil
}

func (controller Controller) dmapQuit(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.DmapQuit(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("关闭dmap..."), nil
}

func (controller Controller) dmwatcherStart(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.DmwatcherStart(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("开始启动dmwatcher..."), nil
}

func (controller Controller) dmwatcherQuit(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.DmwatcherQuit(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("关闭dmwatcher..."), nil
}

func (controller Controller) ping(context *gin.Context) (*frame.Response, *frame.Error) {
	ip, err := controller.service.Ping(context.Param("addr"))
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK(ip), nil
}

func (controller Controller) status(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.DmserverStatus(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("dm status ok"), nil
}
