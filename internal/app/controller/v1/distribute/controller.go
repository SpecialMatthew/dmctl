/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/10 15:36
     Project: dmctl
     Package: distribute
    Describe: Todo
*/

package distribute

import (
	"dmctl/internal/pkg/business/v1/common/typed"
	"dmctl/internal/pkg/business/v1/distribute"
	"dmctl/internal/pkg/frame"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	service distribute.Interface
}

func NewController(service distribute.Interface) *Controller {
	return &Controller{service: service}
}

func (controller *Controller) Register(engine *frame.DmEngine) {
	engine.POST("/dmctl/v1/single", controller.single)
	engine.POST("/dmctl/v1/rww", controller.rww)
	engine.POST("/dmctl/v1/ddw", controller.ddw)
	engine.POST("/dmctl/v1/monitor", controller.monitor)
	engine.GET("/dmctl/v1/nodeWakeup", controller.nodeWakeUp)
	engine.GET("/dmctl/v1/monitorWakeUp", controller.monitorWakeUp)
	engine.GET("/dmctl/v1/primaryDbWakeUp", controller.primaryDbWakeUp)
	engine.GET("/dmctl/health", controller.health)
	engine.POST("/dmctl/v1/bakJsonRestore", controller.BakJsonRestore)
	engine.GET("/dmctl/v1/monitorCheck", controller.monitorCheck)
}

func (controller Controller) single(context *gin.Context) (*frame.Response, *frame.Error) {
	var configs string
	err := controller.service.Single(context, configs)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("单实例初始化成功，正在启动数据库服务..."), nil
}

func (controller Controller) rww(context *gin.Context) (*frame.Response, *frame.Error) {
	return frame.OK("开始部署rww..."), nil
}

func (controller Controller) ddw(context *gin.Context) (*frame.Response, *frame.Error) {
	return frame.OK("开始部署ddw..."), nil
}

func (controller Controller) monitor(context *gin.Context) (*frame.Response, *frame.Error) {
	return frame.OK("开始部署monitor..."), nil
}

func (controller Controller) nodeWakeUp(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.NodeWakeUp(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("Wakeup success..."), nil
}

func (controller Controller) monitorWakeUp(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.MonitorWakeUp(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("monitorWakeUp success..."), nil
}

func (controller Controller) primaryDbWakeUp(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.PrimaryDbWakeUp(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("monitorWakeUp success..."), nil
}

func (controller Controller) health(context *gin.Context) (*frame.Response, *frame.Error) {
	return frame.OK("healthy dmctl..."), nil
}

func (controller Controller) BakJsonRestore(context *gin.Context) (*frame.Response, *frame.Error) {
	var ddwBak *typed.DdwBak
	if err := context.Bind(&ddwBak); err != nil {
		return nil, frame.BadRequestError(err)
	}
	err := controller.service.BakJsonRestore(context, ddwBak)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("ddwBakJsonFile restore success..."), nil
}

func (controller Controller) monitorCheck(context *gin.Context) (*frame.Response, *frame.Error) {
	err := controller.service.MonitorCheck(context)
	if err != nil {
		return nil, frame.DefaultError(err)
	}
	return frame.OK("MonitorCheck success..."), nil
}
