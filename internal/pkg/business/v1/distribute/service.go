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
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"k8s.io/klog/v2"
)

type Service struct {
	CommonService *common.Service
}

func (service Service) Single(context *gin.Context, inventory string) error {
	var inventoryMaps map[string]interface{}
	if err := json.Unmarshal([]byte(inventory), &inventoryMaps); err != nil {
		klog.Errorf("Unmarshal Single configMaps error: %s", err)
		return err
	}
	klog.Infof("configMaps parse result: %s", inventoryMaps)

	//Step 1: dminit
	klog.Infof("----------Single Step 1: dminit start")
	service.CommonService.DmInit(context, nil)
	klog.Infof("----------Single Step 1: dminit end")
	//Step 2: config dm.ini & dmarch.ini
	klog.Infof("----------Single Step 2: config start")
	dmConfigs := make(map[string]*typed.ConfigValue)

	dmConfigs["MAX_SESSIONS"] = &typed.ConfigValue{Value: "1000", Type: "dm.ini"}
	dmConfigs["BAK_PATH"] = &typed.ConfigValue{Value: "/opt/dmdbms/backup", Type: "dm.ini"}

	if tools.GetEnv("DM_INIT_ARCH_FLAG", "1") == "1" {
		err := tools.CreateFile(tools.GetEnv("DM_INIT_PATH", "/opt/dmdbms/data")+"/"+tools.GetEnv("DM_INIT_DB_NAME", "DAMENG")+"/dmarch.ini", "")
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}
		dmConfigs["ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "LOCAL", Type: "dmarch.ini"}
		dmConfigs["ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("DM_INIT_PATH", "/opt/dmdbms/data") + "/arch", Type: "dmarch.ini"}
		dmConfigs["ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "128", Type: "dmarch.ini"}
		dmConfigs["ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "8192", Type: "dmarch.ini"}
	}
	err := service.CommonService.Config(context, dmConfigs)
	if err != nil {
		klog.Errorf("Config dmarch.ini err: %s", err)
	}

	var allConfigs map[string]typed.ConfigValue
	configs := inventoryMaps["configs"]
	err = mapstructure.Decode(configs, &allConfigs)
	if err != nil {
		klog.Errorf("mapstructure Decode err: %s", err)
		return err
	}

	for name, value := range allConfigs {
		dmConfigs[name] = &value
	}
	klog.Infof("dmConfigs: %s", dmConfigs)
	//create BAK_PATH
	err = tools.CreateDir(dmConfigs["BAK_PATH"].Value)
	if err != nil {
		klog.Errorf("CreateDir %s err: %s", dmConfigs["BAK_PATH"].Value, err)
	}
	klog.Infof("----------Single Step 2: config end")

	//Step 4: exec init sql script after dmserver is running
	klog.Infof("----------Single Step 4: exec init sql script after dmserver is running start")
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
		klog.Infof("----------Single Step 4: exec init sql script after dmserver is running end")

	}()

	//Step 3: start dmserver
	klog.Infof("----------Single Step 3: dmserver start")
	err = service.CommonService.DmserverStart(context, nil)
	if err != nil {
		klog.Errorf("DmserverStart err: %s", err)
	}
	return nil
}
