/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/10 15:33
     Project: dmctl
     Package: distribute
    Describe: Todo
*/

package distribute

import (
	"dmctl/internal/pkg/business/v1/common"
	"dmctl/internal/pkg/business/v1/common/typed"
	"dmctl/tools"
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

type Service struct {
	CommonService *common.Service
}

func (service Service) Single(context *gin.Context, inventory string) error {
	var inventoryArrs []*typed.ConfigValue
	if err := json.Unmarshal([]byte(inventory), &inventoryArrs); err != nil {
		klog.Errorf("Unmarshal Single inventoryArrs error: %s", err)
		return err
	}
	klog.Infof("inventoryArrs parse result: %s", inventoryArrs)

	//Step 1: dminit
	// check db instance exist
	exist, err := tools.PathExists(tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG"))
	if !exist {
		klog.Infof("----------Single Step 1: dminit start")
		err = service.CommonService.DmInit(context, nil)
		if err != nil {
			klog.Errorf("DmInit err: %s", err)
		}
		klog.Infof("----------Single Step 1: dminit end")
	} else {
		klog.Infof("----------Single Step 1: instance exist & dminit skip")
	}
	//Step 2: config dm.ini & dmarch.ini
	klog.Infof("----------Single Step 2: config start")
	dmConfigs := make(map[string]*typed.ConfigValue)

	dmConfigs["MAX_SESSIONS"] = &typed.ConfigValue{Value: "1000", Type: "dm.ini"}
	dmConfigs["BAK_PATH"] = &typed.ConfigValue{Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/backup", Type: "dm.ini"}
	dmConfigs["BAK_USE_AP"] = &typed.ConfigValue{Value: tools.GetEnv("BAK_USE_AP", "2"), Type: "dm.ini"}

	if tools.GetEnv("DM_INIT_ARCH_FLAG", "1") == "1" {
		file := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG") + "/dmarch.ini"
		err := tools.CreateFile(file, "", false)
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}
		dmConfigs["ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "LOCAL", Type: "dmarch.ini"}
		dmConfigs["ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/arch", Type: "dmarch.ini"}
		dmConfigs["ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "128", Type: "dmarch.ini"}
		dmConfigs["ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "8192", Type: "dmarch.ini"}
	}

	var allConfigMaps = make(map[string]*typed.ConfigValue)
	/*var allConfigArrs []*typed.ConfigValue
	configs := inventoryMaps["configs"]
	err = mapstructure.Decode(configs, &allConfigArrs)
	if err != nil {
		klog.Errorf("mapstructure Decode err: %s", err)
		return err
	}*/
	//前台使用arr便于操作渲染，后台使用map便于赋值操作，所以此此处将前台的数组转为map进行后续操作
	tools.ConfigArr2Map(inventoryArrs, allConfigMaps)

	for name, value := range allConfigMaps {
		dmConfigs[name] = value
	}
	klog.Infof("dmConfigs: %s", dmConfigs)
	//start config
	err = service.CommonService.Config(context, dmConfigs)
	if err != nil {
		klog.Errorf("modify Configs err: %s", err)
	}

	//create BAK_PATH
	err = tools.CreateDir(dmConfigs["BAK_PATH"].Value)
	if err != nil {
		klog.Errorf("CreateDir %s err: %s", dmConfigs["BAK_PATH"].Value, err)
	}
	klog.Infof("----------Single Step 2: config end")

	//Step 3: exec init sql script after dmserver is running
	klog.Infof("----------Single Step 3: exec init sql script after dmserver is running start")
	go func() {
		err = service.CommonService.ListenPort(context, "tcp", tools.GetEnv("DM_INI_PORT_NUM", "5236"))
		if err != nil {
			klog.Errorf("ListenPort err: %s", err)
		}

		//执行初始化sql脚本
		err = service.CommonService.InitSql(context)
		if err != nil {
			klog.Errorf("InitSql err: %s", err)
		}
		klog.Infof("----------Single Step 3: exec init sql script after dmserver is running end")

	}()

	//Step 4: start watch dmctl.ini to modify dm config file
	klog.Infof("----------Single Step 4: dmctl.ini watching start")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Errorf("fsnotify NewWatcher  error: %s", err)
	}

	err = service.CommonService.ConfigsWatchDog(context, tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/script.d/dmctl.ini", watcher)
	if err != nil {
		klog.Errorf("Wake up the configsWatchDog error: %s", err)
		return err
	}

	//Step 5: start dmserver
	klog.Infof("----------Single Step 5: dmserver start")
	err = service.CommonService.DmserverStart(context, nil)
	if err != nil {
		klog.Errorf("DmserverStart err: %s", err)
	}
	return nil
}
