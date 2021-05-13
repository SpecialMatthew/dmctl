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

package controller

import (
	"dmctl/internal/app/controller/v1/common"
	"dmctl/internal/app/controller/v1/distribute"
	"dmctl/internal/pkg/business"
	"dmctl/internal/pkg/frame"
	"dmctl/internal/pkg/frame/middlewares"
	"net/http"
)

func NewServerHandler() http.Handler {

	// web engine
	handler := frame.NewDmEngine()

	// middleware: cors
	handler.Use(middlewares.CORS())

	// business services
	services := business.NewServices()

	// registration: common interface
	common.NewController(services.Common()).Register(handler)
	// registration: distribute interface
	distribute.NewController(services.Distribute()).Register(handler)

	return handler
}
