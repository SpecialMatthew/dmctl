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
