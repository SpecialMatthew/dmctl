/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 15:16
     Project: dmctl
     Package: business
    Describe: Todo
*/

package business

import (
	"dmctl/internal/pkg/business/v1/common"
	"dmctl/internal/pkg/business/v1/distribute"
)

type Interface interface {
	// common
	Common() common.Interface
	// common
	Distribute() distribute.Interface
}

type Services struct {
	common     *common.Service
	distribute *distribute.Service
}

func (receiver Services) Common() common.Interface {
	return receiver.common
}

func (receiver Services) Distribute() distribute.Interface {
	return receiver.distribute
}

//创建一个服务实例
func NewServices(handles ...Handle) *Services {
	// handlers builder
	//将工具类客户端等实例信息注册到Handles里面
	receiver := &Handles{}
	for _, handle := range handles {
		handle(receiver)
	}

	// services builder
	// services实例的创建
	var services Services

	//通用接口
	services.common = &common.Service{}
	//实例发放接口,将通用接口注册到发放接口中
	services.distribute = &distribute.Service{CommonService: services.common}

	return &services
}

type Handles struct {
}

type Handle func(*Handles)
