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
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os/exec"
	"strings"
)

type Service struct {
	CommonService *common.Service
}

func (service Service) Single(context *gin.Context, inventory string) error {
	var inventoryArrs []*typed.ConfigValue
	//Step 1: dminit & persistence logs
	// check db instance exist
	instancePath := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG")
	exist, err := tools.PathExists(instancePath)
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

	//是否持久化数据库日志
	if tools.GetEnv("PERSISTENCE_LOGS", "true") == "true" {
		cmdStr := "cd ${DM_HOME} && mkdir -p ${DM_INIT_PATH}/log && rm -rf log && ln -s ${DM_INIT_PATH}/log log && touch ${DM_INIT_PATH}/container.ctl"
		klog.Infof("persistence logs cmd: %s", cmdStr)
		execCmd := exec.Command("bash", "-c", cmdStr)
		err = execCmd.Run()
		if err != nil {
			klog.Errorf("save dmctl.ini error: %s......", err)
		}
	}

	//创建dmctl.ini配置文件
	path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/dmctl.ini"
	dmctlIniExist, err := tools.PathExists(path)
	if err != nil {
		klog.Errorf("get %s error: %s......", path, err)
	}

	if dmctlIniExist {
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			klog.Errorf("get dmctl.ini error: %s", err)
		}
		inventory = fmt.Sprint(string(bytes))
		klog.Infof("dmctl.ini content: %s", inventory)
		/*
			/   此处的/opt/dmdbms/script.d/dmctl.ini为k8s挂载进来的文件，只有读权限，因为dmctl.ini需要同步修改，所以copy一份放到挂载的pvc目录data下上进行持久化
			/  /opt/dmdbms/data/dmctl.ini作为参数修改历史记录的副本，在修改参数时进行比较
		*/
		cmdStr := "cat ${DM_HOME}/script.d/dmctl.ini > ${DM_INIT_PATH}/dmctl.ini"
		klog.Infof("save dmctl.ini in dm_init_path : %s", cmdStr)
		execCmd := exec.Command("bash", "-c", cmdStr)
		err = execCmd.Run()
		if err != nil {
			klog.Errorf("save dmctl.ini error: %s......", err)
		}
	} else {
		err := tools.CreateDir(tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d")
		if err != nil {
			return err
		}
		err = tools.CreateFile(path, "[]", false)
		if err != nil {
			return err
		}
	}

	//解析dm.ini配置参数
	if err := json.Unmarshal([]byte(inventory), &inventoryArrs); err != nil {
		klog.Errorf("Unmarshal Single inventoryArrs error: %s", err)
		return err
	}
	klog.Infof("inventoryArrs parse result: %s", inventoryArrs)

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
	err = service.CommonService.Config(context, dmConfigs, common.STATIC_CONFIG)
	if err != nil {
		klog.Errorf("modify Configs err: %s", err)
	}

	//create BAK_PATH
	err = tools.CreateDir(dmConfigs["BAK_PATH"].Value)
	if err != nil {
		klog.Errorf("CreateDir %s err: %s", dmConfigs["BAK_PATH"].Value, err)
	}
	klog.Infof("----------Single Step 2: config end")

	dmIniExist, err := tools.PathExists(instancePath + "/dm.ini")
	if err != nil {
		klog.Errorf("get dm.ini error: %s......", err)
	}
	if dmIniExist {
		//获取db_port
		getPortNumCmdStr := `res=$(sed -r -n '/^PORT_NUM/'p ` + instancePath + `/dm.ini);res=${res#*=};res=${res%%#*};echo $res`
		klog.Infof("getPortNumCmd : %s", getPortNumCmdStr)
		getPortNumCmd := exec.Command("bash", "-c", getPortNumCmdStr)
		portNum_bytes, err := getPortNumCmd.CombinedOutput()
		if err != nil {
			klog.Errorf("getPortNum error: %s......", err)
			return err
		}
		dbPort := string(portNum_bytes)
		dbPort = strings.Trim(dbPort, "\n")
		typed.DbPort = dbPort
		klog.Infof("DB_PORT is [%s]", typed.DbPort)
	} else {
		klog.Infof("dm.ini has yet created!")
	}

	//Step 3: exec init sql script after dmserver is running
	klog.Infof("----------Single Step 3: exec init sql script after dmserver is running start")
	go func() {
		err = service.CommonService.ListenPort(context, "tcp", typed.DbPort)
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
