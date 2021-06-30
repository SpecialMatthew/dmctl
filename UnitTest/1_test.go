package UnitTest

import (
	"dmctl/internal/pkg/business/v1/common/typed"
	"dmctl/tools"
	"fmt"
	"github.com/go-ping/ping"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStr(t *testing.T) {
	podName := "adjkh-0"
	a := podName[strings.LastIndex(podName, "-")+1:]
	fmt.Println(a)
	currentNode, _ := strconv.Atoi(podName[strings.LastIndex(podName, "-")+1:])
	fmt.Println(currentNode)
}

func Test2(t *testing.T) {
	isMaster := false
	execSql := fmt.Sprintln("SP_SET_PARA_VALUE(1, 'ALTER_MODE_STATUS', 1);")
	execSql = execSql + fmt.Sprintln("sp_set_oguid(453331);")
	execSql = execSql + fmt.Sprintln("SP_SET_PARA_VALUE(1, 'ALTER_MODE_STATUS', 0);")
	if isMaster {
		execSql = execSql + fmt.Sprintln("alter database primary;")
	} else {
		execSql = execSql + fmt.Sprintln("alter database standby;")
	}
	fmt.Print(execSql)
}

func Test3(t *testing.T) {
	dmarchIniConfig := make(map[string]*typed.ConfigValue)
	dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_TYPE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "LOCAL"}
	dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_DEST"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "/opt/dmdbms/data/arch"}
	dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_FILE_SIZE"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "128"}
	dmarchIniConfig["ARCHIVE_LOCAL1_ARCH_SPACE_LIMIT"] = &typed.ConfigValue{Group: "ARCHIVE_LOCAL1", Value: "8192"}
	dmarchIniConfigFile := &typed.ConfigFile{Configs: dmarchIniConfig, BootStrapModel: "rww", Replicas: 3, CurrentNode: 1}
	str, _ := tools.ParseTemplate("dmarch.ini.gotmpl", dmarchIniConfigFile)
	tools.CreateFile("D:\\dmarch.ini", str, true)
	fmt.Print(str)
}

func Test4(t *testing.T) {
	replicas := 2
	objectName := "ddw"
	namespace := "dm-test"
	dmmalIniConfig := make(map[string]*typed.ConfigValue)
	dmmalIniConfig["MAL_CHECK_INTERVAL"] = &typed.ConfigValue{Value: "5", Type: "dmmal.ini"}
	dmmalIniConfig["MAL_CONN_FAIL_INTERVAL"] = &typed.ConfigValue{Value: "5", Type: "dmmal.ini"}
	for node := 0; node < replicas; node++ {
		monDwDomainName := objectName + "-" + strconv.Itoa(node) + "." + objectName + "-hl." + namespace + ".svc.cluster.local"
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_NAME"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "GRP1_RT_" + strconv.Itoa(node+1), Type: "dmmal.ini"}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_HOST"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: monDwDomainName, Type: "dmmal.ini"}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "61141", Type: "dmmal.ini"}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_HOST"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: monDwDomainName, Type: "dmmal.ini"}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "32141", Type: "dmmal.ini"}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_DW_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "52141", Type: "dmmal.ini"}
		dmmalIniConfig["MAL_INST"+strconv.Itoa(node+1)+"_MAL_INST_DW_PORT"] = &typed.ConfigValue{Group: "MAL_INST" + strconv.Itoa(node+1), Value: "33141", Type: "dmmal.ini"}
	}
	dmmalIniConfigFile := &typed.ConfigFile{Configs: dmmalIniConfig, BootStrapModel: "ddw_p", Replicas: replicas}
	str, _ := tools.ParseTemplate("dmmal.ini.gotmpl", dmmalIniConfigFile)
	fmt.Print(str)
}

func Test5(t *testing.T) {
	objectName := tools.GetEnv("OBJECT_NAME", "ddw")
	namespace := tools.GetEnv("NAMESPACE", "default")
	replicas := 2

	dmmonitorLog := "/opt/dmdbms/dmmonitorLog"
	dmmonitorIniConfig := make(map[string]*typed.ConfigValue)
	dmmonitorIniConfig["MON_DW_CONFIRM"] = &typed.ConfigValue{Value: "1", Type: "dmmonitor.ini"}
	dmmonitorIniConfig["MON_LOG_PATH"] = &typed.ConfigValue{Value: dmmonitorLog, Type: "dmmonitor.ini"}
	dmmonitorIniConfig["MON_LOG_INTERVAL"] = &typed.ConfigValue{Value: "60", Type: "dmmonitor.ini"}
	dmmonitorIniConfig["MON_LOG_FILE_SIZE"] = &typed.ConfigValue{Value: "32", Type: "dmmonitor.ini"}
	dmmonitorIniConfig["MON_LOG_SPACE_LIMIT"] = &typed.ConfigValue{Value: "0", Type: "dmmonitor.ini"}
	dmmonitorIniConfig["GRP1_MON_INST_OGUID"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_OGUID", "453331"), Type: "dmmonitor.ini"}
	for node := 0; node < replicas; node++ {
		monDwDomainName := objectName + "-" + strconv.Itoa(node) + "." + objectName + "-hl." + namespace + ".svc.cluster.local"
		klog.Infof("monDwDomainName: %s", monDwDomainName)
		dmmonitorIniConfig["GRP1_MON_DW_IP-"+strconv.Itoa(node)] = &typed.ConfigValue{Group: "GRP1", Value: monDwDomainName + ":52141", Type: "dmmonitor.ini", Repeatable: true}
	}
	dmmonitorIniConfigFile := &typed.ConfigFile{Configs: dmmonitorIniConfig, BootStrapModel: "ddw_p", Replicas: replicas}
	str, _ := tools.ParseTemplate("dmmonitor.ini.gotmpl", dmmonitorIniConfigFile)
	fmt.Print(str)
}

func Test6(t *testing.T) {
	dmwatcherIniConfig := make(map[string]*typed.ConfigValue)
	dmwatcherIniConfig["GRP1_DW_TYPE"] = &typed.ConfigValue{Group: "GRP1", Value: "GLOBAL", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_DW_MODE"] = &typed.ConfigValue{Group: "GRP1", Value: "AUTO", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_DW_ERROR_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "10", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_RECOVER_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "60", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_ERROR_TIME"] = &typed.ConfigValue{Group: "GRP1", Value: "10", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_OGUID"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_OGUID", "453331"), Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_INI"] = &typed.ConfigValue{Group: "GRP1", Value: "/dm.ini", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_AUTO_RESTART"] = &typed.ConfigValue{Group: "GRP1", Value: "1", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_INST_STARTUP_CMD"] = &typed.ConfigValue{Group: "GRP1", Value: tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/bin/dmserver", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_RLOG_SEND_THRESHOLD"] = &typed.ConfigValue{Group: "GRP1", Value: "0", Type: "dmwatcher.ini"}
	dmwatcherIniConfig["GRP1_RLOG_APPLY_THRESHOLD"] = &typed.ConfigValue{Group: "GRP1", Value: "0", Type: "dmwatcher.ini"}
	dmwatcherConfigFile := &typed.ConfigFile{Configs: dmwatcherIniConfig, BootStrapModel: "ddw"}
	str, _ := tools.ParseTemplate("dmwatcher.ini.gotmpl", dmwatcherConfigFile)
	fmt.Print(str)
}

func Test7(t *testing.T) {
	pinger, err := ping.NewPinger("www.baidu.com")
	pinger.Timeout = time.Second * 3
	pinger.SetPrivileged(true)
	if err != nil {
		klog.Errorf("new ping err: %v", err)
	}
	pinger.Count = 1
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		klog.Errorf("ping err: %v", err)
	}
	stats := pinger.Statistics() // get send/receive/duplicate/rtt stats
	fmt.Print(stats.IPAddr.IP)
}

func Test8(t *testing.T) {
	a := ""

	fmt.Print(len(a))

}
