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

import "github.com/gin-gonic/gin"

type Interface interface {
	Single(context *gin.Context, configs string) error
}
