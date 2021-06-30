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
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Service struct {
	CommonService *common.Service
}

var nodeWait sync.WaitGroup
var monitorWait sync.WaitGroup
var primaryDbWait sync.WaitGroup

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
			klog.Errorf("persistence logs error: %s......", err)
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
		/*file := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG") + "/dmarch.ini"
		err := tools.CreateFile(file, "", false)
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}*/
		dmarchIniConfig := make(map[string]*typed.ConfigValue)
		dmarchIniConfigPath := instancePath + "/dmarch.ini"
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_TYPE", "LOCAL")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/arch"}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_FILE_SIZE", "128")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT", "8192")}
		dmarchIniConfigFile := &typed.ConfigFile{Configs: dmarchIniConfig, BootStrapModel: "single", Replicas: 1}
		err := service.CommonService.CreateConfigFile(dmarchIniConfigFile, dmarchIniConfigPath, "dmarch.ini.gotmpl")
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}
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
	err = service.CommonService.Config(context, dmConfigs, common.StaticConfig)
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
	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}
	klog.Infof("----------Single Step 3: exec init sql script after dmserver is running start")
	go func() {
		err = service.CommonService.ListenPort(context, "tcp", "localhost", *dbPort)
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

func (service Service) DDW(context *gin.Context, inventory string) error {
	objectName := tools.GetEnv("OBJECT_NAME", "")
	namespace := tools.GetEnv("NAMESPACE", "default")
	instancePath := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG")
	podName := tools.GetEnv("HOSTNAME", "dm")
	replicas, _ := strconv.Atoi(tools.GetEnv("REPLICAS", "1"))
	isMaster := strings.HasSuffix(podName, "-0")

	//开启协程同步/etc/hosts
	err := service.CommonService.SyncHosts(objectName, namespace, replicas)
	if err != nil {
		klog.Errorf("----------ddw SyncHosts error: %s ", err)
	}

	if isMaster {
		klog.Infof(tools.DDW_PRIMARY)
		err := service.Share(context, inventory)
		if err != nil {
			klog.Errorf("----------primary db share backupsets error ", err)
		}
	} else {
		klog.Infof(tools.DDW_STANDBY)
	}

	var inventoryArrs []*typed.ConfigValue
	//Step 1: dminit & persistence logs
	// check db instance exist
	instanceExist, err := tools.PathExists(instancePath)
	if !instanceExist {
		klog.Infof("----------DDW Step 1: dminit start")
		err = service.CommonService.DmInit(context, nil)
		if err != nil {
			klog.Errorf("DmInit err: %s", err)
		}
		klog.Infof("----------DDW Step 1: dminit end")
	} else {
		klog.Infof("----------DDW Step 1: instance exist & dminit skip")
	}

	//是否持久化数据库日志
	if tools.GetEnv("PERSISTENCE_LOGS", "true") == "true" {
		cmdStr := "cd ${DM_HOME} && mkdir -p ${DM_INIT_PATH}/log && rm -rf log && ln -s ${DM_INIT_PATH}/log log && touch ${DM_INIT_PATH}/container.ctl"
		klog.Infof("persistence logs cmd: %s", cmdStr)
		execCmd := exec.Command("bash", "-c", cmdStr)
		err = execCmd.Run()
		if err != nil {
			klog.Errorf("persistence logs error: %s......", err)
		}
	}

	//创建dmctl.ini配置文件
	dmctlIni := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/dmctl.ini"
	dmctlIniExist, err := tools.PathExists(dmctlIni)
	if err != nil {
		klog.Errorf("get %s error: %s......", dmctlIni, err)
	}

	if dmctlIniExist {
		bytes, err := ioutil.ReadFile(dmctlIni)
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
		err = tools.CreateFile(dmctlIni, "[]", false)
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
	klog.Infof("----------DDW Step 2: config start")
	dmConfigs := make(map[string]*typed.ConfigValue)

	index, _ := strconv.Atoi(podName[strings.LastIndex(podName, "-")+1:])

	dmConfigs["MAX_SESSIONS"] = &typed.ConfigValue{Value: "1000", Type: "dm.ini"}
	dmConfigs["INSTANCE_NAME"] = &typed.ConfigValue{Value: "GRP1_RT_" + strconv.Itoa(index+1), Type: "dm.ini"}
	dmConfigs["DW_INACTIVE_INTERVAL"] = &typed.ConfigValue{Value: "60", Type: "dm.ini"}
	dmConfigs["ALTER_MODE_STATUS"] = &typed.ConfigValue{Value: "0", Type: "dm.ini"}
	dmConfigs["ENABLE_OFFLINE_TS"] = &typed.ConfigValue{Value: "2", Type: "dm.ini"}
	dmConfigs["MAL_INI"] = &typed.ConfigValue{Value: "1", Type: "dm.ini"}
	dmConfigs["RLOG_SEND_APPLY_MON"] = &typed.ConfigValue{Value: "64", Type: "dm.ini"}
	dmConfigs["BAK_PATH"] = &typed.ConfigValue{Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/backup", Type: "dm.ini"}
	dmConfigs["BAK_USE_AP"] = &typed.ConfigValue{Value: tools.GetEnv("BAK_USE_AP", "2"), Type: "dm.ini"}

	if tools.GetEnv("DM_INIT_ARCH_FLAG", "1") == "1" {
		var dmarchIniConfigFile *typed.ConfigFile
		dmarchIniConfig := make(map[string]*typed.ConfigValue)
		dmarchIniConfigPath := instancePath + "/dmarch.ini"
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_TYPE", "LOCAL")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/arch"}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_FILE_SIZE", "128")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT", "8192")}
		if isMaster {
			dmarchIniConfigFile = &typed.ConfigFile{Configs: dmarchIniConfig, BootStrapModel: "ddw_p", Replicas: replicas}
		} else {
			dmarchIniConfigFile = &typed.ConfigFile{Configs: dmarchIniConfig, BootStrapModel: "ddw_s"}
		}
		err := service.CommonService.CreateConfigFile(dmarchIniConfigFile, dmarchIniConfigPath, "dmarch.ini.gotmpl")
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}
	}

	var allConfigMaps = make(map[string]*typed.ConfigValue)
	//前台使用arr便于操作渲染，后台使用map便于赋值操作，所以此此处将前台的数组转为map进行后续操作
	tools.ConfigArr2Map(inventoryArrs, allConfigMaps)

	for name, value := range allConfigMaps {
		dmConfigs[name] = value
	}

	var portNum string
	portNumConfig, hasPortNum := dmConfigs["PORT_NUM"]
	if hasPortNum {
		portNum = portNumConfig.Value
	} else {
		portNum = "32141"
		dmConfigs["PORT_NUM"] = &typed.ConfigValue{Value: "32141", Type: "dm.ini"}
	}

	//配置dmmal.ini
	dmmalIniConfigPath := instancePath + "/dmmal.ini"
	dmmalIniConfig := make(map[string]*typed.ConfigValue)
	dmmalIniConfig["MAL_CHECK_INTERVAL"] = &typed.ConfigValue{Value: "5", Type: "dmmal.ini"}
	dmmalIniConfig["MAL_CONN_FAIL_INTERVAL"] = &typed.ConfigValue{Value: "5", Type: "dmmal.ini"}
	for node := 0; node < replicas; node++ {
		//monDwDomainName := objectName+"-"+strconv.Itoa(node)+"."+objectName+"-hl."+namespace+".svc.cluster.local"
		monDwDomainName := objectName + "-" + strconv.Itoa(node)
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_NAME"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "GRP1_RT_" + strconv.Itoa(node+1)}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_HOST"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: monDwDomainName}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: tools.GetEnv("MAL_INST"+strconv.Itoa(node+1)+"_MAL_PORT", "61141")}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_HOST"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: monDwDomainName}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: portNum}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_DW_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: tools.GetEnv("MAL_INST"+strconv.Itoa(node+1)+"_MAL_DW_PORT", "52141")}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_DW_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: tools.GetEnv("MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_DW_PORT", "33141")}
	}
	dmmalIniConfigFile := &typed.ConfigFile{Configs: dmmalIniConfig, BootStrapModel: "ddw_p", Replicas: replicas}
	err = service.CommonService.CreateConfigFile(dmmalIniConfigFile, dmmalIniConfigPath, "dmmal.ini.gotmpl")
	if err != nil {
		klog.Errorf("Create dmmal.ini err: %s", err)
	}

	//配置 dmwatcher.ini
	dmwatcherIniConfigPath := instancePath + "/dmwatcher.ini"
	dmwatcherIniConfig := make(map[string]*typed.ConfigValue)
	dmwatcherIniConfig["GRP1_DW_TYPE"] = &typed.ConfigValue{Group: "GRP1", Value: "GLOBAL", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_DW_MODE"] = &typed.ConfigValue{Group: "GRP1", Value: "AUTO", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_DW_ERROR_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "10", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_RECOVER_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "60", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_ERROR_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "10", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_OGUID"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_OGUID", "453331"), Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_INI"] = &typed.ConfigValue{Group: "GRP1", Value: instancePath + "/dm.ini", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_AUTO_RESTART"] = &typed.ConfigValue{Group: "GRP1", Value: "1", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_STARTUP_CMD"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/bin/dmserver", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_RLOG_SEND_THRESHOLD"] = &typed.ConfigValue{Group: "GRP1", Value: "0", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_RLOG_APPLY_THRESHOLD"] = &typed.ConfigValue{Group: "GRP1", Value: "0", Type: "dmwatcher.ini"}
	dmwatcherConfigFile := &typed.ConfigFile{Configs: dmwatcherIniConfig, BootStrapModel: "ddw", Replicas: replicas}
	err = service.CommonService.CreateConfigFile(dmwatcherConfigFile, dmwatcherIniConfigPath, "dmwatcher.ini.gotmpl")
	if err != nil {
		klog.Errorf("Create dmwatcher.ini err: %s", err)
	}

	klog.Infof("dmConfigs: %s", dmConfigs)
	//start config
	err = service.CommonService.Config(context, dmConfigs, common.StaticConfig)
	if err != nil {
		klog.Errorf("modify Configs err: %s", err)
	}

	//create BAK_PATH
	err = tools.CreateDir(dmConfigs["BAK_PATH"].Value)
	if err != nil {
		klog.Errorf("CreateDir %s err: %s", dmConfigs["BAK_PATH"].Value, err)
	}

	if !isMaster {
		ddwBakPath := dmConfigs["BAK_PATH"].Value + "/" + objectName + "-primary-bak"
		ddwBakPathExist, _ := tools.PathExists(ddwBakPath)
		if ddwBakPathExist {
			klog.Infof("----------ddw bak had restored! skip restored...")
		} else {
			klog.Infof("----------DDW I'm standby db, waiting orders")
			nodeWait.Add(1)
			nodeWait.Wait()
			klog.Infof("----------DDW I'm standby db, launch")
		}
	}

	klog.Infof("----------DDW Step 2: config end")

	//start dmap
	klog.Infof("----------start dmap")
	err = service.CommonService.DmapStart(context)
	if err != nil {
		klog.Errorf("DmapStart err: %s", err)
	}

	//check dmap start
	err = service.CommonService.CheckProcessRunning(context, "dmap")
	if err != nil {
		klog.Errorf("CheckProcessRunning err: %s", err)
	}

	if !isMaster {
		//配置s3.ini文件
		path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/s3.ini"
		s3IniExist, err := tools.PathExists(path)
		if err != nil {
			klog.Errorf("get %s error: %s......", path, err)
		}
		if s3IniExist {
			bytes, err := ioutil.ReadFile(path)
			if err != nil {
				klog.Errorf("get s3.ini error: %s", err)
			}
			inventory = fmt.Sprint(string(bytes))
			klog.Infof("s3.ini content: %s", inventory)
			/*
				/   此处的/opt/dmdbms/script.d/dmctl.ini为k8s挂载进来的文件，只有读权限，因为dmctl.ini需要同步修改，所以copy一份放到挂载的pvc目录data下上进行持久化
				/  /opt/dmdbms/data/dmctl.ini作为参数修改历史记录的副本，在修改参数时进行比较
			*/
			s3Path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/data"
			s3PathExist, err := tools.PathExists(s3Path)
			if err != nil {
				klog.Errorf("get s3PathExist error: %s......", s3Path, err)
			}
			if !s3PathExist {
				err := tools.CreateDir(s3Path)
				if err != nil {
					klog.Errorf("create dir error: %s......", s3Path, err)
				}
			}
			cmdStr := "cat ${DM_HOME}/script.d/s3.ini > ${DM_HOME}/data/s3.ini"
			klog.Infof("save s3.ini in data : %s", cmdStr)
			execCmd := exec.Command("bash", "-c", cmdStr)
			err = execCmd.Run()
			if err != nil {
				klog.Errorf("save s3.ini error: %s......", err)
			}

			//执行流式备份初始化数据集到s3上
			err = service.CommonService.DmrmanExecCmd(context, "RESTORE DATABASE '"+instancePath+"/dm.ini' FROM BACKUPSET '"+dmConfigs["BAK_PATH"].Value+"/"+podName[:strings.LastIndex(podName, "-")]+"-primary-bak' device type tape")
			if err != nil {
				klog.Errorf("DmrmanExecCmd error: %s......", err)
			}
			err = service.CommonService.DmrmanExecCmd(context, "RECOVER DATABASE '"+instancePath+"/dm.ini' FROM BACKUPSET '"+dmConfigs["BAK_PATH"].Value+"/"+podName[:strings.LastIndex(podName, "-")]+"-primary-bak' device type tape")
			if err != nil {
				klog.Errorf("DmrmanExecCmd error: %s......", err)
			}
			err = service.CommonService.DmrmanExecCmd(context, "RECOVER DATABASE '"+instancePath+"/dm.ini' UPDATE DB_MAGIC")
			if err != nil {
				klog.Errorf("DmrmanExecCmd error: %s......", err)
			}
			klog.Infof("----------init backupSets to s3 end")
		} else {
			klog.Infof("----------s3.ini not exist! Game Over!")
			return errors.New("s3.ini not exist! Game Over!")
		}
	}

	//Step 4: exec init sql script after dmserver is running
	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}
	klog.Infof("----------DDW Step 3: exec init sql script after dmserver is running start")
	go func() {
		err = service.CommonService.ListenPort(context, "tcp", "localhost", *dbPort)
		if err != nil {
			klog.Errorf("ListenPort err: %s", err)
		}

		//设置 OGUID
		execSql := ""
		execSql = execSql + fmt.Sprintln("SP_SET_PARA_VALUE(1, 'ALTER_MODE_STATUS', 1);")
		execSql = execSql + fmt.Sprintln("sp_set_oguid("+tools.GetEnv("DM_OGUID", "453331")+");")
		execSql = execSql + fmt.Sprintln("SP_SET_PARA_VALUE(1, 'ALTER_MODE_STATUS', 0);")
		if isMaster {
			execSql = execSql + fmt.Sprintln("alter database primary;")
		} else {
			execSql = execSql + fmt.Sprintln("alter database standby;")
		}
		err = service.CommonService.ExecSql(context, execSql)
		if err != nil {
			klog.Errorf("ExecSql err: %s", err)
		}

		if isMaster {
			go func() {
				monitorServiceName := objectName + "-monitor-service." + namespace + ".svc"
				for {
					resp, err := tools.Get("http://" + monitorServiceName + "/dmctl/health")
					if err != nil || resp.StatusCode != 200 {
						klog.Errorf("dmctl health check err: %s", err)
					} else if resp.StatusCode == 200 {
						break
					}
					time.Sleep(time.Second * 5)
				}

				//执行初始化sql脚本
				err = service.CommonService.InitSql(context)
				if err != nil {
					klog.Errorf("InitSql err: %s", err)
				}
				klog.Infof("----------DDW Step 3: exec init sql script after dmserver is running end")
			}()
		}

		//启动dmwatcher
		err = service.CommonService.DmwatcherStart(context)
		if err != nil {
			klog.Errorf("DmwatcherStart err: %s", err)
		}
	}()

	//Step 4: start watch dmctl.ini to modify dm config file
	klog.Infof("----------DDW Step 4: dmctl.ini watching start")
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
	klog.Infof("----------DDW Step 5: dmserver start")
	params := map[string]interface{}{"startModel": "mount"}
	err = service.CommonService.DmserverStart(context, params)
	if err != nil {
		klog.Errorf("DmserverStart err: %s", err)
	}
	return nil
}

func (service Service) NodeWakeUp(context *gin.Context) error {
	nodeWait.Done()
	klog.Infof("WakeUp success")
	return nil
}

func (service Service) PrimaryDbWakeUp(context *gin.Context) error {
	primaryDbWait.Done()
	klog.Infof("primaryDbWakeUp success")
	return nil
}

func (service Service) RWW(context *gin.Context, inventory string) error {
	objectName := tools.GetEnv("OBJECT_NAME", "")
	namespace := tools.GetEnv("NAMESPACE", "default")
	instancePath := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG")
	podName := tools.GetEnv("HOSTNAME", "dm")
	replicas, _ := strconv.Atoi(tools.GetEnv("REPLICAS", "1"))
	isMaster := strings.HasSuffix(podName, "-0")
	currentNode, _ := strconv.Atoi(podName[strings.LastIndex(podName, "-")+1:])

	//开启协程同步/etc/hosts
	err := service.CommonService.SyncHosts(objectName, namespace, replicas)
	if err != nil {
		klog.Errorf("----------rww SyncHosts error: %s ", err)
	}

	if isMaster {
		klog.Infof(tools.RWW_PRIMARY)
		err := service.Share(context, inventory)
		if err != nil {
			klog.Errorf("----------primary db share backupsets error ", err)
		}
	} else {
		klog.Infof(tools.RWW_STANDBY)
	}

	var inventoryArrs []*typed.ConfigValue
	//Step 1: dminit & persistence logs
	// check db instance exist
	instanceExist, err := tools.PathExists(instancePath)
	if !instanceExist {
		klog.Infof("----------RWW Step 1: dminit start")
		err = service.CommonService.DmInit(context, nil)
		if err != nil {
			klog.Errorf("DmInit err: %s", err)
		}
		klog.Infof("----------RWW Step 1: dminit end")
	} else {
		klog.Infof("----------RWW Step 1: instance exist & dminit skip")
	}

	//是否持久化数据库日志
	if tools.GetEnv("PERSISTENCE_LOGS", "true") == "true" {
		cmdStr := "cd ${DM_HOME} && mkdir -p ${DM_INIT_PATH}/log && rm -rf log && ln -s ${DM_INIT_PATH}/log log && touch ${DM_INIT_PATH}/container.ctl"
		klog.Infof("persistence logs cmd: %s", cmdStr)
		execCmd := exec.Command("bash", "-c", cmdStr)
		err = execCmd.Run()
		if err != nil {
			klog.Errorf("persistence logs error: %s......", err)
		}
	}

	//创建dmctl.ini配置文件
	dmctlIni := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/dmctl.ini"
	dmctlIniExist, err := tools.PathExists(dmctlIni)
	if err != nil {
		klog.Errorf("get %s error: %s......", dmctlIni, err)
	}

	if dmctlIniExist {
		bytes, err := ioutil.ReadFile(dmctlIni)
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
		err = tools.CreateFile(dmctlIni, "[]", false)
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
	klog.Infof("----------RWW Step 2: config start")
	dmConfigs := make(map[string]*typed.ConfigValue)

	index, _ := strconv.Atoi(podName[strings.LastIndex(podName, "-")+1:])

	dmConfigs["MAX_SESSIONS"] = &typed.ConfigValue{Value: "1000", Type: "dm.ini"}
	dmConfigs["INSTANCE_NAME"] = &typed.ConfigValue{Value: "GRP1_RWW_" + strconv.Itoa(index+1), Type: "dm.ini"}
	dmConfigs["DW_INACTIVE_INTERVAL"] = &typed.ConfigValue{Value: "60", Type: "dm.ini"}
	dmConfigs["ALTER_MODE_STATUS"] = &typed.ConfigValue{Value: "0", Type: "dm.ini"}
	dmConfigs["ENABLE_OFFLINE_TS"] = &typed.ConfigValue{Value: "2", Type: "dm.ini"}
	dmConfigs["MAL_INI"] = &typed.ConfigValue{Value: "1", Type: "dm.ini"}
	dmConfigs["RLOG_SEND_APPLY_MON"] = &typed.ConfigValue{Value: "64", Type: "dm.ini"}
	dmConfigs["BAK_PATH"] = &typed.ConfigValue{Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/backup", Type: "dm.ini"}
	dmConfigs["BAK_USE_AP"] = &typed.ConfigValue{Value: tools.GetEnv("BAK_USE_AP", "2"), Type: "dm.ini"}

	if tools.GetEnv("DM_INIT_ARCH_FLAG", "1") == "1" {
		var dmarchIniConfigFile *typed.ConfigFile
		dmarchIniConfig := make(map[string]*typed.ConfigValue)
		dmarchIniConfigPath := instancePath + "/dmarch.ini"
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_TYPE", "LOCAL")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/arch"}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_FILE_SIZE", "128")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT", "8192")}
		dmarchIniConfigFile = &typed.ConfigFile{Configs: dmarchIniConfig, BootStrapModel: "rww", Replicas: replicas, CurrentNode: currentNode}
		err := service.CommonService.CreateConfigFile(dmarchIniConfigFile, dmarchIniConfigPath, "dmarch.ini.gotmpl")
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}
	}

	var allConfigMaps = make(map[string]*typed.ConfigValue)
	//前台使用arr便于操作渲染，后台使用map便于赋值操作，所以此此处将前台的数组转为map进行后续操作
	tools.ConfigArr2Map(inventoryArrs, allConfigMaps)

	for name, value := range allConfigMaps {
		dmConfigs[name] = value
	}

	var portNum string
	portNumConfig, hasPortNum := dmConfigs["PORT_NUM"]
	if hasPortNum {
		portNum = portNumConfig.Value
	} else {
		portNum = "32141"
		dmConfigs["PORT_NUM"] = &typed.ConfigValue{Value: "32141", Type: "dm.ini"}
	}

	//配置dmmal.ini
	dmmalIniConfigPath := instancePath + "/dmmal.ini"
	dmmalIniConfig := make(map[string]*typed.ConfigValue)
	dmmalIniConfig["MAL_CHECK_INTERVAL"] = &typed.ConfigValue{Value: "5", Type: "dmmal.ini"}
	dmmalIniConfig["MAL_CONN_FAIL_INTERVAL"] = &typed.ConfigValue{Value: "5", Type: "dmmal.ini"}
	for node := 0; node < replicas; node++ {
		//monDwDomainName := objectName+"-"+strconv.Itoa(node)+"."+objectName+"-hl."+namespace+".svc.cluster.local"
		monDwDomainName := objectName + "-" + strconv.Itoa(node)
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_NAME"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "GRP1_RWW_" + strconv.Itoa(node+1)}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_HOST"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: monDwDomainName}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: tools.GetEnv("MAL_INST"+strconv.Itoa(node+1)+"_MAL_PORT", "61141")}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_HOST"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: monDwDomainName}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: portNum}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_DW_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: tools.GetEnv("MAL_INST"+strconv.Itoa(node+1)+"_MAL_DW_PORT", "52141")}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_DW_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: tools.GetEnv("MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_DW_PORT", "33141")}
	}
	dmmalIniConfigFile := &typed.ConfigFile{Configs: dmmalIniConfig, BootStrapModel: "ddw_p", Replicas: replicas}
	err = service.CommonService.CreateConfigFile(dmmalIniConfigFile, dmmalIniConfigPath, "dmmal.ini.gotmpl")
	if err != nil {
		klog.Errorf("Create dmmal.ini err: %s", err)
	}

	//配置 dmwatcher.ini
	dmwatcherIniConfigPath := instancePath + "/dmwatcher.ini"
	dmwatcherIniConfig := make(map[string]*typed.ConfigValue)
	dmwatcherIniConfig["GRP1_DW_TYPE"] = &typed.ConfigValue{Group: "GRP1", Value: "GLOBAL", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_DW_MODE"] = &typed.ConfigValue{Group: "GRP1", Value: "AUTO", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_DW_ERROR_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "10", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_RECOVER_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "60", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_ERROR_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "10", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_OGUID"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_OGUID", "453332"), Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_INI"] = &typed.ConfigValue{Group: "GRP1", Value: instancePath + "/dm.ini", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_AUTO_RESTART"] = &typed.ConfigValue{Group: "GRP1", Value: "1", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_STARTUP_CMD"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/bin/dmserver", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_RLOG_SEND_THRESHOLD"] = &typed.ConfigValue{Group: "GRP1", Value: "0", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_RLOG_APPLY_THRESHOLD"] = &typed.ConfigValue{Group: "GRP1", Value: "0", Type: "dmwatcher.ini"}
	dmwatcherConfigFile := &typed.ConfigFile{Configs: dmwatcherIniConfig, BootStrapModel: "ddw", Replicas: replicas}
	err = service.CommonService.CreateConfigFile(dmwatcherConfigFile, dmwatcherIniConfigPath, "dmwatcher.ini.gotmpl")
	if err != nil {
		klog.Errorf("Create dmwatcher.ini err: %s", err)
	}

	klog.Infof("dmConfigs: %s", dmConfigs)
	//start config
	err = service.CommonService.Config(context, dmConfigs, common.StaticConfig)
	if err != nil {
		klog.Errorf("modify Configs err: %s", err)
	}

	//create BAK_PATH
	err = tools.CreateDir(dmConfigs["BAK_PATH"].Value)
	if err != nil {
		klog.Errorf("CreateDir %s err: %s", dmConfigs["BAK_PATH"].Value, err)
	}

	if !isMaster {
		ddwBakPath := dmConfigs["BAK_PATH"].Value + "/" + objectName + "-primary-bak"
		ddwBakPathExist, _ := tools.PathExists(ddwBakPath)
		if ddwBakPathExist {
			klog.Infof("----------RWW bak had restored! skip restored...")
		} else {
			klog.Infof("----------RWW I'm standby db, waiting orders")
			nodeWait.Add(1)
			nodeWait.Wait()
			klog.Infof("----------RWW I'm standby db, launch")
		}
	}

	klog.Infof("----------RWW Step 2: config end")

	//start dmap
	klog.Infof("----------start dmap")
	err = service.CommonService.DmapStart(context)
	if err != nil {
		klog.Errorf("DmapStart err: %s", err)
	}

	//check dmap start
	err = service.CommonService.CheckProcessRunning(context, "dmap")
	if err != nil {
		klog.Errorf("CheckProcessRunning err: %s", err)
	}

	if !isMaster {
		//配置s3.ini文件
		path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/s3.ini"
		s3IniExist, err := tools.PathExists(path)
		if err != nil {
			klog.Errorf("get %s error: %s......", path, err)
		}
		if s3IniExist {
			bytes, err := ioutil.ReadFile(path)
			if err != nil {
				klog.Errorf("get s3.ini error: %s", err)
			}
			inventory = fmt.Sprint(string(bytes))
			klog.Infof("s3.ini content: %s", inventory)
			/*
				/   此处的/opt/dmdbms/script.d/dmctl.ini为k8s挂载进来的文件，只有读权限，因为dmctl.ini需要同步修改，所以copy一份放到挂载的pvc目录data下上进行持久化
				/  /opt/dmdbms/data/dmctl.ini作为参数修改历史记录的副本，在修改参数时进行比较
			*/
			s3Path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/data"
			s3PathExist, err := tools.PathExists(s3Path)
			if err != nil {
				klog.Errorf("get s3PathExist error: %s......", s3Path, err)
			}
			if !s3PathExist {
				err := tools.CreateDir(s3Path)
				if err != nil {
					klog.Errorf("create dir error: %s......", s3Path, err)
				}
			}
			cmdStr := "cat ${DM_HOME}/script.d/s3.ini > ${DM_HOME}/data/s3.ini"
			klog.Infof("save s3.ini in data : %s", cmdStr)
			execCmd := exec.Command("bash", "-c", cmdStr)
			err = execCmd.Run()
			if err != nil {
				klog.Errorf("save s3.ini error: %s......", err)
			}

			//执行流式备份初始化数据集到s3上
			err = service.CommonService.DmrmanExecCmd(context, "RESTORE DATABASE '"+instancePath+"/dm.ini' FROM BACKUPSET '"+dmConfigs["BAK_PATH"].Value+"/"+podName[:strings.LastIndex(podName, "-")]+"-primary-bak' device type tape")
			if err != nil {
				klog.Errorf("DmrmanExecCmd error: %s......", err)
			}
			err = service.CommonService.DmrmanExecCmd(context, "RECOVER DATABASE '"+instancePath+"/dm.ini' FROM BACKUPSET '"+dmConfigs["BAK_PATH"].Value+"/"+podName[:strings.LastIndex(podName, "-")]+"-primary-bak' device type tape")
			if err != nil {
				klog.Errorf("DmrmanExecCmd error: %s......", err)
			}
			err = service.CommonService.DmrmanExecCmd(context, "RECOVER DATABASE '"+instancePath+"/dm.ini' UPDATE DB_MAGIC")
			if err != nil {
				klog.Errorf("DmrmanExecCmd error: %s......", err)
			}
			klog.Infof("----------init backupSets to s3 end")
		} else {
			klog.Infof("----------s3.ini not exist! Game Over!")
			return errors.New("s3.ini not exist! Game Over!")
		}
	}

	//Step 4: exec init sql script after dmserver is running
	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}
	klog.Infof("----------RWW Step 3: exec init sql script after dmserver is running start")
	go func() {
		err = service.CommonService.ListenPort(context, "tcp", "localhost", *dbPort)
		if err != nil {
			klog.Errorf("ListenPort err: %s", err)
		}

		//设置 OGUID
		execSql := ""
		execSql = execSql + fmt.Sprintln("SP_SET_PARA_VALUE(1, 'ALTER_MODE_STATUS', 1);")
		execSql = execSql + fmt.Sprintln("sp_set_oguid("+tools.GetEnv("DM_OGUID", "453331")+");")
		execSql = execSql + fmt.Sprintln("SP_SET_PARA_VALUE(1, 'ALTER_MODE_STATUS', 0);")
		if isMaster {
			execSql = execSql + fmt.Sprintln("alter database primary;")
		} else {
			execSql = execSql + fmt.Sprintln("alter database standby;")
		}
		err = service.CommonService.ExecSql(context, execSql)
		if err != nil {
			klog.Errorf("ExecSql err: %s", err)
		}

		if isMaster {
			go func() {
				monitorServiceName := objectName + "-monitor-service." + namespace + ".svc"
				for {
					resp, err := tools.Get("http://" + monitorServiceName + "/dmctl/health")
					if err != nil || resp.StatusCode != 200 {
						klog.Errorf("dmctl health check err: %s", err)
					} else if resp.StatusCode == 200 {
						break
					}
					time.Sleep(time.Second * 5)
				}

				//执行初始化sql脚本
				err = service.CommonService.InitSql(context)
				if err != nil {
					klog.Errorf("InitSql err: %s", err)
				}
				klog.Infof("----------RWW Step 3: exec init sql script after dmserver is running end")
			}()
		}

		//启动dmwatcher
		err = service.CommonService.DmwatcherStart(context)
		if err != nil {
			klog.Errorf("DmwatcherStart err: %s", err)
		}
	}()

	//Step 4: start watch dmctl.ini to modify dm config file
	klog.Infof("----------RWW Step 4: dmctl.ini watching start")
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
	klog.Infof("----------RWW Step 5: dmserver start")
	params := map[string]interface{}{"startModel": "mount"}
	err = service.CommonService.DmserverStart(context, params)
	if err != nil {
		klog.Errorf("DmserverStart err: %s", err)
	}
	return nil
}

func (service Service) Share(context *gin.Context, inventory string) error {
	objectName := tools.GetEnv("OBJECT_NAME", "")
	namespace := tools.GetEnv("NAMESPACE", "default")
	replicas, _ := strconv.Atoi(tools.GetEnv("REPLICAS", "1"))
	podName := tools.GetEnv("HOSTNAME", "dm")
	var inventoryArrs []*typed.ConfigValue
	//Step 1: dminit & persistence logs
	// check db instance exist
	instancePath := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG")
	exist, err := tools.PathExists(instancePath)
	if !exist {
		klog.Infof("----------Share Step 1: dminit start")
		err = service.CommonService.DmInit(context, nil)
		if err != nil {
			klog.Errorf("DmInit err: %s", err)
		}
		klog.Infof("----------Share Step 1: dminit end")
	} else {
		klog.Infof("----------Share Step 1: instance exist & dminit skip")
	}

	//创建dmctl.ini配置文件
	dmctlIni := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/dmctl.ini"
	dmctlIniExist, err := tools.PathExists(dmctlIni)
	if err != nil {
		klog.Errorf("get %s error: %s......", dmctlIni, err)
	}

	if dmctlIniExist {
		bytes, err := ioutil.ReadFile(dmctlIni)
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
		err = tools.CreateFile(dmctlIni, "[]", false)
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
	klog.Infof("----------Share Step 2: config start")
	dmConfigs := make(map[string]*typed.ConfigValue)

	dmConfigs["MAX_SESSIONS"] = &typed.ConfigValue{Value: "1000", Type: "dm.ini"}
	dmConfigs["BAK_PATH"] = &typed.ConfigValue{Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/backup", Type: "dm.ini"}
	dmConfigs["BAK_USE_AP"] = &typed.ConfigValue{Value: tools.GetEnv("BAK_USE_AP", "2"), Type: "dm.ini"}

	if tools.GetEnv("DM_INIT_ARCH_FLAG", "1") == "1" {
		dmarchIniConfig := make(map[string]*typed.ConfigValue)
		dmarchIniConfigPath := instancePath + "/dmarch.ini"
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_TYPE", "LOCAL")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/arch"}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: tools.GetEnv("ARCHIVE_LOCAL1_ARCH_FILE_SIZE", "128")}
		dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "0"}
		dmarchIniConfigFile := &typed.ConfigFile{Configs: dmarchIniConfig, BootStrapModel: "share", Replicas: 1}
		err := service.CommonService.CreateConfigFile(dmarchIniConfigFile, dmarchIniConfigPath, "dmarch.ini.gotmpl")
		if err != nil {
			klog.Errorf("Create dmarch.ini err: %s", err)
		}
	}

	var allConfigMaps = make(map[string]*typed.ConfigValue)
	//前台使用arr便于操作渲染，后台使用map便于赋值操作，所以此此处将前台的数组转为map进行后续操作
	tools.ConfigArr2Map(inventoryArrs, allConfigMaps)

	for name, value := range allConfigMaps {
		dmConfigs[name] = value
	}
	klog.Infof("dmConfigs: %s", dmConfigs)

	ddwBakPath := dmConfigs["BAK_PATH"].Value + "/" + objectName + "-primary-bak"
	ddwBakPathExist, _ := tools.PathExists(ddwBakPath)
	if ddwBakPathExist {
		klog.Infof("----------ddw bak had shared! skip share...")
		return nil
	}

	//start config
	err = service.CommonService.Config(context, dmConfigs, common.StaticConfig)
	if err != nil {
		klog.Errorf("modify Configs err: %s", err)
	}
	klog.Infof("----------Share Step 2: config end")

	//create BAK_PATH
	err = tools.CreateDir(dmConfigs["BAK_PATH"].Value)
	if err != nil {
		klog.Errorf("CreateDir %s err: %s", dmConfigs["BAK_PATH"].Value, err)
	}

	klog.Infof("----------Share Step 3: start dmap")
	err = service.CommonService.DmapStart(context)
	if err != nil {
		klog.Errorf("DmapStart err: %s", err)
	}

	//Step 4: start dmserver
	klog.Infof("----------Share Step 4: dmserver start")
	err = service.CommonService.DmserverStart(context, nil)
	if err != nil {
		klog.Errorf("DmserverStart err: %s", err)
	}

	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}
	err = service.CommonService.ListenPort(context, "tcp", "localhost", *dbPort)
	if err != nil {
		klog.Errorf("ListenPort err: %s", err)
	}
	err = service.CommonService.DmserverPause(context)
	if err != nil {
		klog.Errorf("DmserverPause err: %s", err)
	}
	//check dmap start
	err = service.CommonService.CheckProcessRunning(context, "dmap")
	if err != nil {
		klog.Errorf("CheckProcessRunning err: %s", err)
	}

	klog.Infof("----------Share Step 5: init backupSets to s3 start")
	//首先配置s3.ini文件
	path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/s3.ini"
	s3IniExist, err := tools.PathExists(path)
	if err != nil {
		klog.Errorf("get %s error: %s......", path, err)
	}
	if s3IniExist {
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			klog.Errorf("get s3.ini error: %s", err)
		}
		inventory = fmt.Sprint(string(bytes))
		klog.Infof("s3.ini content: %s", inventory)
		/*
			/   此处的/opt/dmdbms/script.d/dmctl.ini为k8s挂载进来的文件，只有读权限，因为dmctl.ini需要同步修改，所以copy一份放到挂载的pvc目录data下上进行持久化
			/  /opt/dmdbms/data/dmctl.ini作为参数修改历史记录的副本，在修改参数时进行比较
		*/
		s3Path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/data"
		s3PathExist, err := tools.PathExists(s3Path)
		if err != nil {
			klog.Errorf("get s3PathExist error: %s......", s3Path, err)
		}
		if !s3PathExist {
			err := tools.CreateDir(s3Path)
			if err != nil {
				klog.Errorf("create dir error: %s......", s3Path, err)
			}
		}
		cmdStr := "cat ${DM_HOME}/script.d/s3.ini > ${DM_HOME}/data/s3.ini"
		klog.Infof("save s3.ini in data : %s", cmdStr)
		execCmd := exec.Command("bash", "-c", cmdStr)
		err = execCmd.Run()
		if err != nil {
			klog.Errorf("save s3.ini error: %s......", err)
		}

		//执行流式备份初始化数据集到s3上
		err = service.CommonService.DmrmanExecCmd(context, "BACKUP DATABASE '"+instancePath+"/dm.ini' FULL BACKUPSET '"+dmConfigs["BAK_PATH"].Value+"/"+podName[:strings.LastIndex(podName, "-")]+"-primary-bak'  device type tape")
		if err != nil {
			klog.Errorf("DmrmanExecCmd error: %s......", err)
		}
		klog.Infof("----------Share Step 5: init backupSets to s3 end")
	} else {
		klog.Infof("----------Share Step 5: s3.ini not exist! Game Over!")
	}

	ddwBakJson, err := tools.ReadFile(ddwBakPath + "/" + objectName + "-primary-bak.bak.json")
	if err != nil {
		klog.Errorf("ReadFile error: %s......", err)
	}
	ddwMetaJson, err := tools.ReadFile(ddwBakPath + "/" + objectName + "-primary-bak.meta.json")
	if err != nil {
		klog.Errorf("ReadFile error: %s......", err)
	}

	postData := &typed.DdwBak{
		DdwBakPath: ddwBakPath,
		BakJson:    ddwBakJson,
		MetaJson:   ddwMetaJson,
	}

	//通知备库开始还原和安装
	for node := 1; node < replicas; node++ {
		monDwDomainName := objectName + "-" + strconv.Itoa(node) + "." + objectName + "-hl." + namespace + ".svc.cluster.local"
		go func(node int) {
			for {
				resp, err := tools.Get("http://" + monDwDomainName + "/dmctl/health")
				if err != nil {
					klog.Errorf("get health err: %s", err)
				} else {
					if resp.StatusCode == 200 {
						klog.Errorf("standby-%s dmctl healthy", node)
						break
					}
				}
			}

			resp, err := tools.Post("http://"+monDwDomainName+"/dmctl/v1/bakJsonRestore", postData, "application/json")
			if err != nil {
				klog.Errorf("get health err: %s", err)
			} else {
				if resp.StatusCode == 200 {
					resp, err := tools.Get("http://" + monDwDomainName + "/dmctl/v1/nodeWakeup")
					if err != nil {
						klog.Errorf("get Wakeup err: %s", err)
					} else {
						if resp.StatusCode == 200 {
							klog.Infof("%s Wakeup success...", objectName+"-"+strconv.Itoa(node))
						} else {
							klog.Infof("%s Wakeup failed...", objectName+"-"+strconv.Itoa(node))
						}
					}
				}
			}

		}(node)
	}

	//执行完备份正常退出
	//os.Exit(0)
	return nil
}

func (service Service) Monitor(context *gin.Context, configs string) error {
	/*klog.Infof("----------Monitor I'm Monitor, waiting orders")
	monitorWait.Add(1)
	monitorWait.Wait()*/
	objectName := tools.GetEnv("OBJECT_NAME", "")
	namespace := tools.GetEnv("NAMESPACE", "default")
	replicas, _ := strconv.Atoi(tools.GetEnv("REPLICAS", "1"))

	//开启协程同步/etc/hosts
	err := service.CommonService.SyncHosts(objectName, namespace, replicas)
	if err != nil {
		klog.Errorf("----------monitor SyncHosts error: %s ", err)
	}

	monitorPath := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/data/DAMENG"
	err = tools.CreateDir(monitorPath)
	if err != nil {
		klog.Errorf("Create monitorPath[%v] err: %s", monitorPath, err)
	}

	klog.Infof("----------Monitor Step 1: config dmmonitor.ini start")
	//配置dmmonitor.ini
	dmmonitorLog := monitorPath + "/dmmonitorLog"
	err = tools.CreateDir(dmmonitorLog)
	if err != nil {
		klog.Errorf("Create dmmonitorLog path err: %s", err)
	}

	dmmonitorIniConfigPath := monitorPath + "/dmmonitor.ini"
	dmmonitorTestIniConfigPath := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/dmmonitor.ini"
	dmmonitorIniConfig := make(map[string]*typed.ConfigValue)
	dmmonitorIniConfig["MON_DW_CONFIRM"] = &typed.ConfigValue{Value: tools.GetEnv("MON_DW_CONFIRM", "1")}
	dmmonitorIniConfig["MON_LOG_PATH"] = &typed.ConfigValue{Value: dmmonitorLog}
	dmmonitorIniConfig["MON_LOG_INTERVAL"] = &typed.ConfigValue{Value: tools.GetEnv("MON_DW_CONFIRM", "60")}
	dmmonitorIniConfig["MON_LOG_FILE_SIZE"] = &typed.ConfigValue{Value: tools.GetEnv("MON_DW_CONFIRM", "32")}
	dmmonitorIniConfig["MON_LOG_SPACE_LIMIT"] = &typed.ConfigValue{Value: tools.GetEnv("MON_DW_CONFIRM", "0")}
	dmmonitorIniConfig["GRP1_MON_INST_OGUID"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_OGUID", "453331")}
	for node := 0; node < replicas; node++ {
		//monDwDomainName := objectName+"-"+strconv.Itoa(node)+"."+objectName+"-hl."+namespace+".svc.cluster.local"
		monDwDomainName := objectName + "-" + strconv.Itoa(node)
		klog.Infof("monDwDomainName: %s", monDwDomainName)
		dmmonitorIniConfig["GRP1_MON_DW_IP-"+strconv.Itoa(node)] = &typed.ConfigValue{Group: "GRP1", Value: monDwDomainName + ":52141", Repeatable: true}
	}
	dmmonitorConfigFile := &typed.ConfigFile{Configs: dmmonitorIniConfig, BootStrapModel: "monitor", Replicas: replicas}
	err = service.CommonService.CreateConfigFile(dmmonitorConfigFile, dmmonitorIniConfigPath, "dmmonitor.ini.gotmpl")
	if err != nil {
		klog.Errorf("Create dmmonitor.ini err: %s", err)
	}

	dmmonitorIniConfig["MON_DW_CONFIRM"] = &typed.ConfigValue{Value: "0"}
	dmmonitorTestConfigFile := &typed.ConfigFile{Configs: dmmonitorIniConfig, BootStrapModel: "monitor", Replicas: replicas}
	err = service.CommonService.CreateConfigFile(dmmonitorTestConfigFile, dmmonitorTestIniConfigPath, "dmmonitor.ini.gotmpl")
	if err != nil {
		klog.Errorf("Create dmmonitorTest.ini err: %s", err)
	}

	chownCmdStr := "chown -R 1001:1001 " + tools.GetEnv("DM_HOME", "/opt/dmdbms")
	chownCmd := exec.Command("bash", "-c", chownCmdStr)
	err = chownCmd.Run()
	if err != nil {
		klog.Errorf("Chown DM_HOME path err: %s", err)
	}

	klog.Infof("----------Monitor Step 1: config dmmonitor.ini end")

	klog.Infof("----------Monitor Step 2: dmmonitor start")
	service.CommonService.DmmonitorStart(context)

	return nil
}

func (service Service) MonitorCheck(context *gin.Context) error {
	cmdStr := "[ $(status | grep OPEN[[:blank:]]*OK.*OPEN | wc -l) -eq ${REPLICAS} ]"
	klog.Infof("dmmonitorCheck : %s", cmdStr)
	execCmd := exec.Command("bash", "-c", cmdStr)
	err := execCmd.Run()
	if err != nil {
		klog.Errorf("dmmonitorCheck error: %s......", err)
		return err
	}
	return nil
}

func (service Service) MonitorWakeUp(context *gin.Context) error {
	monitorWait.Done()
	klog.Infof("MonitorWakeUp success")
	return nil
}

func (service Service) BakJsonRestore(context *gin.Context, ddwBak *typed.DdwBak) error {
	objectName := tools.GetEnv("OBJECT_NAME", "")
	err := tools.CreateDir(ddwBak.DdwBakPath)
	if err != nil {
		klog.Errorf("CreateDirerr: %s", err)
	}
	err = os.Chown(ddwBak.DdwBakPath, 1001, 1001)
	if err != nil {
		klog.Errorf("Chown [%v] err: %s", ddwBak.DdwBakPath, err)
	}

	ddwBakJsonFile := ddwBak.DdwBakPath + "/" + objectName + "-primary-bak.bak.json"
	ddwMetaJsonFile := ddwBak.DdwBakPath + "/" + objectName + "-primary-bak.meta.json"

	err = tools.CreateFile(ddwBakJsonFile, ddwBak.BakJson, true)
	if err != nil {
		klog.Errorf("CreateFile err: %s", err)
	}
	err = tools.CreateFile(ddwMetaJsonFile, ddwBak.MetaJson, true)
	if err != nil {
		klog.Errorf("CreateFile err: %s", err)
	}

	return nil
}

func (service Service) Test(context *gin.Context) error {
	return nil
}
