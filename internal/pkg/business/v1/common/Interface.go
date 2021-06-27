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
	DmserverStatus(context *gin.Context) error
	DmapStart(context *gin.Context) error
	DmapQuit(context *gin.Context) error
	DmwatcherStart(context *gin.Context) error
	DmwatcherQuit(context *gin.Context) error
	DmmonitorStart(context *gin.Context) error
	DmserverRestart(context *gin.Context, params map[string]interface{}) error
	DmInit(context *gin.Context, params map[string]interface{}) error
	Config(context *gin.Context, params map[string]*typed.ConfigValue, configModel string) error
	ExecSql(context *gin.Context, internalSql string) error
	DmrmanExecCmd(context *gin.Context, cmd string) error
	InitSql(context *gin.Context) error
	ListenPort(context *gin.Context, serverType, ip, port string) error
	ConfigsWatchDog(context *gin.Context, file string, watcher *fsnotify.Watcher) error
	CheckProcessRunning(context *gin.Context, serverName string) error
	CreateConfigFile(configFile *typed.ConfigFile, filePath string, templateName string) error
	Ping(addr string) (ip *string, err error)
	SyncHosts(objectName, namespace string, replicas int) error
}
