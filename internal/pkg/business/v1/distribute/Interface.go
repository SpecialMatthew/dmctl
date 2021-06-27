/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/10 15:32
     Project: dmctl
     Package: distribute
    Describe: Todo
*/

package distribute

import (
	"dmctl/internal/pkg/business/v1/common/typed"
	"github.com/gin-gonic/gin"
)

type Interface interface {
	Single(context *gin.Context, configs string) error
	Share(context *gin.Context, configs string) error
	DDW(context *gin.Context, configs string) error
	Monitor(context *gin.Context, configs string) error
	MonitorCheck(context *gin.Context) error
	RWW(context *gin.Context, configs string) error
	DDWWakeUp(context *gin.Context) error
	MonitorWakeUp(context *gin.Context) error
	PrimaryDbWakeUp(context *gin.Context) error
	BakJsonRestore(context *gin.Context, ddwBak *typed.DdwBak) error
	Test(context *gin.Context) error
}
