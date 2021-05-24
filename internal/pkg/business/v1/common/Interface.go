/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 15:34
     Project: dmctl
     Package: start
    Describe: Todo
*/

package common

import (
	"dmctl/internal/pkg/business/v1/common/typed"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
)

type Interface interface {
	DmserverStart(context *gin.Context, params map[string]interface{}) error
	DmserverPause(context *gin.Context) error
	DmserverRestart(context *gin.Context, params map[string]interface{}) error
	DmInit(context *gin.Context, params map[string]interface{}) error
	Config(context *gin.Context, params map[string]*typed.ConfigValue, configModel string) error
	ExecSql(context *gin.Context, internalSql string) error
	InitSql(context *gin.Context) error
	ListenPort(context *gin.Context, serverType string, port string) error
	ConfigsWatchDog(context *gin.Context, file string, watcher *fsnotify.Watcher) error
}
