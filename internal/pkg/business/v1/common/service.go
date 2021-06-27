/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 15:34
     Project: dmctl
     Package: common
    Describe: Todo
*/

package common

import (
	"dmctl/internal/pkg/business/v1/common/typed"
	"dmctl/pkg"
	"dmctl/tools"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/go-ping/ping"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Service struct{}

var pauseSignal = make(chan string)
var quitDmap = make(chan string)
var quitDmwatcher = make(chan string)
var exitVirtualListening = make(chan string)
var dmServer *exec.Cmd
var dmAp *exec.Cmd
var dmWatcher *exec.Cmd
var virtualListening *exec.Cmd

const HotConfig = "dynamic"   //热修改配置，可以在数据库运行时修改
const StaticConfig = "static" //冷修改，数据库停止时修改

func asyncLog(reader io.ReadCloser, logTitle string) error {
	cache := ""
	buf := make([]byte, 1024, 1024)
	for {
		num, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				err = nil
			}
			klog.Errorf(logTitle+" log read err:%s", err)
			return err
		}
		if num > 0 {
			oByte := buf[:num]
			oSlice := strings.Split(string(oByte), "\n")
			line := strings.Join(oSlice[:len(oSlice)-1], "\n")
			klog.Infof(logTitle+": %s%s\n", cache, line)
			cache = oSlice[len(oSlice)-1]
		}
	}
}

func (service Service) DmserverStart(context *gin.Context, params map[string]interface{}) error {
	startModel, hasStartModel := params["startModel"]
	var dmErr error
	go func() {
		//判断数据库是否已经启动
		dmStart := dmServer != nil
		klog.Infof("dmStart: %s......", dmStart)
		if !dmStart {
		Loop:
			for {
				select {
				case pauseSignal := <-pauseSignal:
					klog.Infof("pause dmserver: %s......", pauseSignal)
					break Loop
				default:
					virtualListeningStart := virtualListening != nil
					klog.Infof("virtualListeningStart: %s......", virtualListeningStart)
					if virtualListeningStart {
						virtualListeningProcessStart := virtualListening.Process != nil
						klog.Infof("virtualListeningProcessStart: %s......", virtualListeningProcessStart)
						if virtualListeningProcessStart {
							err := syscall.Kill(-virtualListening.Process.Pid, syscall.SIGKILL)
							if err != nil {
								if err.Error() == "no such process" {
									klog.Warningf("virtualListening close warning: %s......", err.Error())
								}
								klog.Errorf("virtualListening close error: %s......", err.Error())
								break Loop
							}
							//添加虚拟监听端口停止指令
							exitVirtualListening <- "stop"
							//清除旧的监听端口
							virtualListening = nil
						}
					}

					cmdStr := "cd ${DM_HOME}/bin && ./dmserver ${DM_INIT_PATH}/${DM_INIT_DB_NAME:-DAMENG}/dm.ini "
					if hasStartModel {
						cmdStr = cmdStr + startModel.(string)
					}
					klog.Infof("dmserver start command: %s", cmdStr)

					cmd := exec.Command("bash", "-c", cmdStr)
					//使创建的线程都在同一个线程组里面，便于停止线程及子线程
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Setpgid:                    true,
						Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
						GidMappingsEnableSetgroups: true,
					}

					outf, err := cmd.StdoutPipe()
					if err != nil {
						klog.Errorf("Error StdoutPipe: %s......", err.Error())
					}
					errf, err := cmd.StderrPipe()
					if err != nil {
						klog.Errorf("Error StderrPipe: %s......", err.Error())
					}

					go asyncLog(errf, "dmserver")
					go asyncLog(outf, "dmserver")

					dmServer = cmd
					err = cmd.Run()
					if err != nil {
						klog.Errorf("Error dmserver starting command: %s......", err)
						if ex, ok := err.(*exec.ExitError); ok {
							res := ex.Sys().(syscall.WaitStatus).ExitStatus() //获取命令执行返回状态，相当于shell: echo $?
							klog.Infof("cmd exit status: %s......", res)
							if res == 136 { //error: Database first startup failed, reinitialize database please!
								klog.Errorf("dmserver start error: Database first startup failed, reinitialize database please!")
								dmErr = errors.New("Database first startup failed, reinitialize database please!")
								break Loop
							}
						}
					}
				}

			}
		} else {
			klog.Infof("dmServer is running...")
		}
	}()

	return dmErr
}

func (service Service) DmserverPause(context *gin.Context) error {
	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}

	//判断数据库是否已经启动
	dmStart := dmServer != nil
	klog.Infof("dmStart: %s......", dmStart)
	if dmStart {
		err := syscall.Kill(-dmServer.Process.Pid, syscall.SIGQUIT)
		if err != nil {
			if err.Error() == "no such process" {
				klog.Warningf("DmserverPause warning: %s......", err.Error())
				return nil
			}
			klog.Errorf("DmserverPause Error: %s......", err.Error())
			return err
		}
		//添加数据库停止指令
		pauseSignal <- "pause"
		//清除旧的dmServer
		dmServer = nil

		//手动停止数据库时，新起一个虚拟的5236端口监听，防止pods重启
		go func() {
		Loop:
			for {
				select {
				case stop := <-exitVirtualListening:
					klog.Infof("stop virtualListening: %s......", stop)
					break Loop
				default:
					cmd := exec.Command("nc", "-lp", *dbPort)
					//使创建的线程都在同一个线程组里面，便于停止线程及子线程
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Setpgid:                    true,
						Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
						GidMappingsEnableSetgroups: true,
					}
					virtualListening = cmd

					err := cmd.Run()
					if err != nil {
						klog.Errorf("Error virtualListening starting command: %s......", err)
					}
				}
			}
		}()
	} else {
		klog.Infof("dmserver is not running...")
	}

	return nil
}

func (service Service) DmserverRestart(context *gin.Context, params map[string]interface{}) error {
	err := service.DmserverPause(context)
	if err != nil {
		klog.Errorf("DmserverPause Error: %s......", err)
		return err
	}
	err = service.DmserverStart(context, params)
	if err != nil {
		klog.Errorf("DmserverStart Error: %s......", err)
		return err
	}
	return nil
}

func (service Service) ExecSql(context *gin.Context, internalSql string) error {
	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}

	_, err = net.Dial("tcp", "localhost:"+*dbPort)
	if err != nil {
		klog.Infof("dmserver has yet to start, can not exec sql now")
		return err
	}

	var sql string
	if internalSql != "" {
		sql = internalSql
	} else {
		sql = context.PostForm("sql")
	}
	klog.Infof("exec sql: %s", sql)

	err = tools.CreateFile("/tmp/everything.sql", sql, true)
	if err != nil {
		klog.Infof("create /tmp/everything.sql error: %s", err)
		return err
	}
	execCmdStr := "echo 'exit;' >> /tmp/everything.sql && cd ${DM_HOME}/bin && ./disql SYSDBA/'\"" + tools.GetEnv("DM_INIT_SYSDBA_PWD", "Dameng7777") + "\"'@localhost:" + *dbPort + " '`/tmp/everything.sql'"
	klog.Infof("exec sql cmd : %s", execCmdStr)
	execCmd := exec.Command("bash", "-c", execCmdStr)
	err = execCmd.Run()
	if err != nil {
		klog.Errorf("exec sql error: %s......", err)
		return err
	}
	return nil
}

func (service Service) InitSql(context *gin.Context) error {
	dbPort, err := tools.GetDbPort()
	if err != nil {
		klog.Errorf("getDbPort err: %s", err)
	}

	_, err = net.Dial("tcp", "localhost:"+*dbPort)
	if err != nil {
		klog.Infof("dmserver has yet to start, can not exec sql now")
		return err
	}

	path := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/genesis.sql"
	exist, err := tools.PathExists(path)
	if err != nil {
		klog.Errorf("get %s error: %s......", path, err)
	}

	if exist {
		execCmdStr := "cat ${DM_HOME}/script.d/genesis.sql > /tmp/genesis.sql && echo 'exit;' >> /tmp/genesis.sql && chmod 777 /tmp/genesis.sql && cd ${DM_HOME}/bin && ./disql SYSDBA/'\"" + tools.GetEnv("DM_INIT_SYSDBA_PWD", "Dameng7777") + "\"'@localhost:" + *dbPort + " '`/tmp/genesis.sql'"
		klog.Infof("exec sql cmd : %s", execCmdStr)
		execCmd := exec.Command("bash", "-c", execCmdStr)
		err = execCmd.Run()
		if err != nil {
			klog.Errorf("exec sql error: %s......", err)
			return err
		}
	} else {
		klog.Infof("/tmp/genesis.sql not exist")
	}

	return nil
}

func (service Service) DmInit(context *gin.Context, params map[string]interface{}) error {
	cmdStr := "cd ${DM_HOME}/bin && ./dminit "
	if len(os.Environ()) > 0 {
		for _, v := range os.Environ() {
			//输出系统所有环境变量的值
			//fmt.Println("#########",v)
			env := strings.Split(v, "=")
			if strings.HasPrefix(env[0], "DM_INIT_") {
				fmt.Println("#########", env)
				envName := strings.TrimPrefix(env[0], "DM_INIT_")
				cmdStr = cmdStr + envName + "=" + env[1] + " "
			}
		}
		klog.Infof("dminit command: %s", cmdStr)
	}

	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:                    true,
		Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
		GidMappingsEnableSetgroups: true,
	}
	klog.Errorf("dminit exec  command: %s......", cmd.String())

	outf, err := cmd.StdoutPipe()
	if err != nil {
		klog.Errorf("dminit Error StdoutPipe: %s......", err.Error())
		return err
	}
	errf, err := cmd.StderrPipe()
	if err != nil {
		klog.Errorf("dminit Error StderrPipe: %s......", err.Error())
		return err
	}
	go asyncLog(errf, "dminit")
	go asyncLog(outf, "dminit")

	err = cmd.Run()
	if err != nil {
		klog.Errorf("dminit Error starting command: %s......", err)
		return err
	}

	return nil
}

func (service Service) Config(context *gin.Context, params map[string]*typed.ConfigValue, configModel string) error {
	isServerRestart := 0
	var editSql string
	for name, v := range params {
		//获取待修改的文件路径
		configPath := tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG") + "/" + v.Type
		klog.Infof("configPath: %s", configPath)

		//去掉文件中每行开头tab
		formatConfigCmdStr := "sed -i 's/^\t*//g' " + configPath
		klog.Infof("format configFile command: %s", formatConfigCmdStr)
		cmd := exec.Command("bash", "-c", formatConfigCmdStr)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid:                    true,
			Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
			GidMappingsEnableSetgroups: true,
		}
		err := cmd.Run()
		if err != nil {
			klog.Errorf("format configFile %s command error: %s......", v.Type, err)
			return err
		}

		if v.Group != "" {
			klog.Infof("ConfigParam: %s:%s:%s:%s", v.Type, v.Group, name, v.Value)
			//首先查看文件中有没有该分组,有就跳过，没有就创建
			checkGroupCmdStr := "res=$(sed -r -n '/^\\[" + v.Group + "\\]/'p " + configPath + ");[[ -z $res ]] && echo [" + v.Group + "] >> " + configPath + " || echo group exist"
			klog.Infof("check %s group exist command: %s", configPath, checkGroupCmdStr)
			checkGroupCmd := exec.Command("bash", "-c", checkGroupCmdStr)
			checkGroupCmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid:                    true,
				Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
				GidMappingsEnableSetgroups: true,
			}
			err := checkGroupCmd.Run()
			if err != nil {
				klog.Errorf("check %s group error: %s......", configPath, err)
				return err
			}
			//在属组下修改or新增参数
			editConfigCmdStr := "res=$(sed -r -n '/^" + strings.ReplaceAll(name, v.Group+"_", "") + "/'p " + configPath + " |grep '#" + v.Group + "');[[ -n $res ]] && sed -i -r -e 's$^" + strings.ReplaceAll(name, v.Group+"_", "") + "(.*)$" + strings.ReplaceAll(name, v.Group+"_", "") + "=" + v.Value + " #" + v.Group + " #edit$' " + configPath + " || sed -i '/\\[" + v.Group + "\\]/a" + strings.ReplaceAll(name, v.Group+"_", "") + "=" + v.Value + " #" + v.Group + " #edit' " + configPath
			if v.Repeatable {
				editConfigCmdStr = "sed -i '/\\[" + v.Group + "\\]/a" + strings.ReplaceAll(name[:strings.LastIndex(name, "-")], v.Group+"_", "") + "=" + v.Value + " #" + v.Group + " #edit' " + configPath
			}
			if v.Type == "dmarch.ini" {
				editConfigCmdStr = "res=$(sed -r -n '/^" + strings.ReplaceAll(name, v.Group+"_", "") + "/'p " + configPath + " |grep '#" + v.Group + "');[[ -n $res ]] && line=$(grep -n \"${res}\" " + configPath + " | sed 's/:.*//g' ) && sed -i ''${line}'c" + strings.ReplaceAll(name, v.Group+"_", "") + "=" + v.Value + " #" + v.Group + " #edit$' " + configPath + " || sed -i '/\\[" + v.Group + "\\]/a" + strings.ReplaceAll(name, v.Group+"_", "") + "=" + v.Value + " #" + v.Group + " #edit' " + configPath
			}
			klog.Infof("edit %s command: %s", configPath, editConfigCmdStr)
			cmd := exec.Command("bash", "-c", editConfigCmdStr)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid:                    true,
				Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
				GidMappingsEnableSetgroups: true,
			}
			err = cmd.Run()
			if err != nil {
				klog.Errorf("edit %s error: %s......", configPath, err)
				return err
			}
			isServerRestart++
		} else {
			klog.Infof("ConfigParam: %s:%s:%s", v.Type, name, v.Value)
			if v.Type == "dm.ini" && configModel == HotConfig {
				_, ok := pkg.DmIni[name]
				if ok {
					if pkg.DmIni[name].Attribute == 0 || pkg.DmIni[name].Attribute == 1 {
						//修改or新增参数
						editConfigCmdStr := "res=$(sed -r -n '/^" + name + "/'p " + configPath + ");[[ -n $res ]] && sed -i -r -e 's$^" + name + "(.*)$" + name + "=" + v.Value + " #edit$' " + configPath + " || echo " + name + "=" + v.Value + " #edit >>" + configPath
						klog.Infof("edit %s command: %s", configPath, editConfigCmdStr)
						cmd := exec.Command("bash", "-c", editConfigCmdStr)
						cmd.SysProcAttr = &syscall.SysProcAttr{
							Setpgid:                    true,
							Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
							GidMappingsEnableSetgroups: true,
						}
						err := cmd.Run()
						if err != nil {
							klog.Errorf("edit %s error: %s......", configPath, err)
							return err
						}
						isServerRestart++
					} else {
						if pkg.DmIni[name].Attribute == 2 {
							editSql = editSql + fmt.Sprintln(`SP_SET_PARA_VALUE(1,'`+name+`','`+v.Value+`');`)
						}
						if pkg.DmIni[name].Attribute == 3 {
							if pkg.DmIni[name].ValueType == "varchar" {
								editSql = editSql + fmt.Sprintln(`SF_SET_SYSTEM_PARA_VALUE('`+name+`','`+v.Value+`',0,1);`)
							} else {
								editSql = editSql + fmt.Sprintln(`SF_SET_SYSTEM_PARA_VALUE('`+name+`',`+v.Value+`,0,1);`)
							}
						}
					}
				} else {
					klog.Infof("%s is not exist in DmIni......", name)
					continue
				}
			} else {
				//修改or新增参数
				editConfigCmdStr := "res=$(sed -r -n '/^" + name + "/'p " + configPath + ");[[ -n $res ]] && sed -i -r -e 's$^" + name + "(.*)$" + name + "=" + v.Value + " #edit$' " + configPath + " || sed -i '1i" + name + "=" + v.Value + " #edit' " + configPath
				klog.Infof("edit %s command: %s", configPath, editConfigCmdStr)
				cmd := exec.Command("bash", "-c", editConfigCmdStr)
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Setpgid:                    true,
					Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
					GidMappingsEnableSetgroups: true,
				}
				err := cmd.Run()
				if err != nil {
					klog.Errorf("edit %s error: %s......", configPath, err)
					return err
				}
				isServerRestart++
			}
		}
	}

	if editSql != "" {
		klog.Infof("editSql content: %s......", editSql)
		err := service.ExecSql(context, editSql)
		if err != nil {
			klog.Errorf("ExecSql %s error: %s......", editSql, err)
			return err
		}
	}

	if isServerRestart > 0 && configModel == HotConfig {
		klog.Infof("config params contains manual or static params, need to restart dmserver to make it useful")
		service.DmserverRestart(context, nil)
	}

	return nil
}

func (service Service) CreateConfigFile(configFile *typed.ConfigFile, filePath string, templateName string) error {
	//klog.Infof("%s Configs: %s", filePath, configFile.Configs)
	configFileContent, err := tools.ParseTemplate(templateName, configFile)
	if err != nil {
		klog.Errorf("%s parse error: %v", templateName, err)
		return err
	}

	klog.Infof("%s content: %s", filePath, configFileContent)

	err = tools.CreateFile(filePath, configFileContent, true)
	if err != nil {
		klog.Errorf("create [%s] error: %v", filePath, err)
		return err
	}
	return nil
}

func (service Service) ListenPort(context *gin.Context, serviceType string, ip string, port string) error {
	for {
		_, err := net.Dial(serviceType, ip+":"+port)
		if err != nil {
			klog.Infof("serviceType: %s ,ip %s, port: %s has yet to start", serviceType, ip, port)
			time.Sleep(time.Millisecond * 1500)
		} else {
			break
		}
	}
	klog.Infof("serviceType: %s ,port: %s has started", serviceType, port)
	return nil
}

func (service Service) ConfigsWatchDog(context *gin.Context, file string, watcher *fsnotify.Watcher) error {
	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				//klog.Infof("dmctl.ini watch event: %s", event)
				klog.V(8).Infof("dmctl.ini watch event: %s", event)
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					klog.Infof("dmctl.ini has been wrote")

					path1 := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/script.d/dmctl.ini"
					exist1, err := tools.PathExists(path1)
					if err != nil {
						klog.Errorf("get %s error: %s......", path1, err)
					}

					path2 := tools.GetEnv("DM_HOME", "/opt/dmdbms") + "/data/dmctl.ini"
					exist2, err := tools.PathExists(path2)
					if err != nil {
						klog.Errorf("get %s error: %s......", path2, err)
					}

					if exist1 && exist2 {
						bytes1, err := ioutil.ReadFile(path1)
						if err != nil {
							klog.Errorf("get dmctl.ini error: %s", err)
						}
						bytes2, err := ioutil.ReadFile(path2)
						if err != nil {
							klog.Errorf("get history dmctl.ini error: %s", err)
						}

						inventory := fmt.Sprint(string(bytes1))
						inventoryHistory := fmt.Sprint(string(bytes2))

						var inventoryMaps = make(map[string]*typed.ConfigValue)
						var inventoryHistoryMaps = make(map[string]*typed.ConfigValue)
						var inventoryArrs []*typed.ConfigValue
						var inventoryHistoryArrs []*typed.ConfigValue
						if err := json.Unmarshal([]byte(inventory), &inventoryArrs); err != nil {
							klog.Errorf("Unmarshal inventoryArrs error: %s", err)
						}
						klog.Infof("inventoryArrs parse result: %s", inventoryArrs)

						if err := json.Unmarshal([]byte(inventoryHistory), &inventoryHistoryArrs); err != nil {
							klog.Errorf("Unmarshal history inventoryArrs error: %s", err)
						}
						klog.Infof("history inventoryArrs parse result: %s", inventoryHistoryArrs)

						tools.ConfigArr2Map(inventoryArrs, inventoryMaps)
						tools.ConfigArr2Map(inventoryHistoryArrs, inventoryHistoryMaps)

						if !reflect.DeepEqual(inventoryMaps, inventoryHistoryMaps) {
							dmConfigs := make(map[string]*typed.ConfigValue)

							//与上一次历史修改记录对比出此次更新的配置参数
							for name, value := range inventoryMaps {
								_, ok := inventoryHistoryMaps[name]
								if ok {
									if value.Value != inventoryHistoryMaps[name].Value {
										dmConfigs[name] = value
									}
								} else {
									dmConfigs[name] = value
								}
							}
							klog.Infof("dmConfigs: %s", dmConfigs)

							err = service.Config(context, dmConfigs, HotConfig)
							if err != nil {
								klog.Errorf("modify Configs err: %s", err)
							} else {
								//TODO: 更新上一次历史记录
								err := tools.WriteToFile(path2, inventory)
								if err != nil {
									klog.Errorf("update history dmctl.ini err: %s", err)
								}
							}

							//更新完文件被删除之后，需要重新监听
							err := watcher.Add(file)
							if err != nil {
								klog.Errorf("add watcher dmctl.ini error: %s", err)
							}
							klog.Infof("reload watcher!!!")
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				klog.Errorf("dmctl.ini fsnotify watcher error: %s", err)
			}
		}

	}()

	err := watcher.Add(file)
	if err != nil {
		klog.Errorf("add watcher dmctl.ini error: %s", err)
		return err
	}

	return nil
}

func (service Service) DmapStart(context *gin.Context) error {
	var dmapErr error
	go func() {
		//判断dmmap服务是否已经启动
		dmapStart := dmAp != nil
		klog.Infof("dmapStart: %s......", dmapStart)
		if !dmapStart {
		Loop:
			for {
				select {
				case quitDmap := <-quitDmap:
					klog.Infof("quit dmap: %s......", quitDmap)
					break Loop
				default:
					cmdStr := "cd ${DM_HOME}/bin && ./dmap"
					klog.Infof("dmap start command: %s", cmdStr)

					cmd := exec.Command("bash", "-c", cmdStr)
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Setpgid:                    true,
						Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
						GidMappingsEnableSetgroups: true,
					}
					//使创建的线程都在同一个线程组里面，便于停止线程及子线程
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Setpgid:                    true,
						Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
						GidMappingsEnableSetgroups: true,
					}

					outf, err := cmd.StdoutPipe()
					if err != nil {
						klog.Errorf("Error StdoutPipe: %s......", err.Error())
					}
					errf, err := cmd.StderrPipe()
					if err != nil {
						klog.Errorf("Error StderrPipe: %s......", err.Error())
					}

					go asyncLog(errf, "dmap")
					go asyncLog(outf, "dmap")

					dmAp = cmd
					err = cmd.Run()
					if err != nil {
						klog.Errorf("Error dmap starting command: %s......", err)
					}
				}

			}
		} else {
			klog.Infof("dmap is running...")
		}
	}()

	return dmapErr
}

func (service Service) DmmonitorStart(context *gin.Context) error {
	go func() {
		for {
			cmdStr := "cd ${DM_HOME}/bin && ./dmmonitor ${DM_INIT_PATH}/${DM_INIT_DB_NAME:-DAMENG}/dmmonitor.ini"
			klog.Infof("dmmonitor start command: %s", cmdStr)

			cmd := exec.Command("bash", "-c", cmdStr)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid:                    true,
				Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
				GidMappingsEnableSetgroups: true,
			}
			outf, err := cmd.StdoutPipe()
			if err != nil {
				klog.Errorf("Error StdoutPipe: %s......", err.Error())
			}
			errf, err := cmd.StderrPipe()
			if err != nil {
				klog.Errorf("Error StderrPipe: %s......", err.Error())
			}

			go asyncLog(errf, "dmmonitor")
			go asyncLog(outf, "dmmonitor")

			err = cmd.Run()
			if err != nil {
				klog.Errorf("Error dmmonitor starting command: %s......", err)
			}
		}
	}()
	return nil
}

func (service Service) DmapQuit(context *gin.Context) error {
	//判断数据库是否已经启动
	dmapStart := dmAp != nil
	klog.Infof("dmapStart: %s......", dmapStart)
	if dmapStart {
		err := syscall.Kill(-dmAp.Process.Pid, syscall.SIGQUIT)
		if err != nil {
			if err.Error() == "no such process" {
				klog.Warningf("DmapQuit warning: %s......", err.Error())
				return nil
			}
			klog.Errorf("DmapQuit Error: %s......", err.Error())
			return err
		}
		//添加数据库停止指令
		quitDmap <- "quit"
		//清除旧的dmAp
		dmAp = nil
	} else {
		klog.Infof("dmap is not running...")
	}

	return nil
}

func (service Service) DmrmanExecCmd(context *gin.Context, internalCmd string) error {
	var cmd string
	if internalCmd != "" {
		cmd = internalCmd
	} else {
		cmd = context.PostForm("cmd")
	}
	klog.Infof("dmrman exec cmd: %s", cmd)
	execCmdStr := `cd ${DM_HOME}/bin && ./dmrman CTLSTMT="` + cmd + `"`
	klog.Infof("exec dmrman cmd : %s", execCmdStr)
	execCmd := exec.Command("bash", "-c", execCmdStr)
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:                    true,
		Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
		GidMappingsEnableSetgroups: true,
	}

	outf, err := execCmd.StdoutPipe()
	if err != nil {
		klog.Errorf("Error StdoutPipe: %s......", err.Error())
	}
	errf, err := execCmd.StderrPipe()
	if err != nil {
		klog.Errorf("Error StderrPipe: %s......", err.Error())
	}

	go asyncLog(errf, "dmrman")
	go asyncLog(outf, "dmrman")

	err = execCmd.Run()
	if err != nil {
		klog.Errorf("exec dmrman cmd error: %s......", err)
		return err
	}
	return nil
}

func (service Service) CheckProcessRunning(context *gin.Context, serverName string) error {
	cmd := `ps aux | grep  ` + serverName + ` | grep -v grep`
	klog.Infof("CheckProcessRunning cmd : %s......", cmd)
	for {
		result, err := exec.Command("/bin/sh", "-c", cmd).Output()
		out := strings.TrimSpace(string(result))
		if err != nil {
			klog.Errorf("CheckProcessRunning err: %s... ", err)
			time.Sleep(time.Millisecond * 1500)
		}
		if out == "" {
			klog.Infof("Process %s has yet to start... ", serverName)
			time.Sleep(time.Millisecond * 1500)
		} else {
			klog.Infof("Process %s has started...", serverName)
			break
		}
	}

	return nil
}

func (service Service) Ping(addr string) (*string, error) {
	pinger, err := ping.NewPinger(addr)
	if err != nil {
		klog.Errorf("new ping err: %v", err)
		return nil, err
	}
	pinger.SetPrivileged(true)
	pinger.Count = 1
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		klog.Errorf("ping err: %v", err)
		return nil, err
	}
	stats := pinger.Statistics() // get send/receive/duplicate/rtt stats
	ip := fmt.Sprint(stats.IPAddr.IP)
	klog.V(8).Infof("addr[%v]", ip)
	return &ip, nil
}

func (service Service) SyncHosts(objectName, namespace string, replicas int) error {
	klog.Infof("/etc/hosts start synchronizing......")
	for node := 0; node < replicas; node++ {
		monDwDomainName := objectName + "-" + strconv.Itoa(node) + "." + objectName + "-hl." + namespace + ".svc.cluster.local"
		go func(node int) {
			for {
				ipNew, err := service.Ping(monDwDomainName)
				if err != nil {
					klog.V(8).Infof("[%v] connect err: %s", monDwDomainName, err)
					time.Sleep(time.Second * 3)
					continue
				}
				checkHostsCmdStr := "cat /etc/hosts |grep '" + monDwDomainName + "' | awk '{print $1}'"
				checkHostsCmd := exec.Command("bash", "-c", checkHostsCmdStr)
				res, err := checkHostsCmd.CombinedOutput()
				if err != nil {
					klog.Errorf("check /etc/hosts err: %v", err)
					continue
				}
				ip := string(res)
				hosts := *ipNew + "\t" + monDwDomainName + "\t" + objectName + "-" + strconv.Itoa(node)

				if len(res) == 0 {
					cmdStr := "echo '" + hosts + "' >> /etc/hosts"
					klog.Infof("add hosts command [%v]", cmdStr)
					cmd := exec.Command("bash", "-c", cmdStr)
					err := cmd.Run()
					if err != nil {
						klog.Errorf("add hosts err: %v", err)
					}
				} else if strings.Compare(ip, *ipNew) == -1 {
					cmdStr := "line_num=$(sed -n -e '/" + monDwDomainName + "/=' " + "/etc/hosts);[[ -n $line_num ]] && echo \"$(sed \"${line_num}c " + hosts + "\" /etc/hosts)\" > /etc/hosts"
					klog.Infof("sync hosts command [%v]", cmdStr)
					cmd := exec.Command("bash", "-c", cmdStr)
					err := cmd.Run()
					if err != nil {
						klog.Errorf("sync hosts err: %v", err)
					}
				}

				time.Sleep(time.Second * 5)
			}
		}(node)
	}
	return nil
}

func (service Service) DmwatcherStart(context *gin.Context) error {
	var dmwatcherErr error
	go func() {
		//判断dmwatcher服务是否已经启动
		dmwatcherStart := dmWatcher != nil
		klog.Infof("dmwatcherStart: %s......", dmwatcherStart)
		if !dmwatcherStart {
		Loop:
			for {
				select {
				case quitDmwatcher := <-quitDmwatcher:
					klog.Infof("quit dmwatcher: %s......", quitDmwatcher)
					break Loop
				default:
					cmdStr := "cd ${DM_HOME}/bin && ./dmwatcher ${DM_INIT_PATH}/${DM_INIT_DB_NAME:-DAMENG}/dmwatcher.ini"
					klog.Infof("dmwatcher start command: %s", cmdStr)

					cmd := exec.Command("bash", "-c", cmdStr)
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Setpgid:                    true,
						Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
						GidMappingsEnableSetgroups: true,
					}
					//使创建的线程都在同一个线程组里面，便于停止线程及子线程
					cmd.SysProcAttr = &syscall.SysProcAttr{
						Setpgid:                    true,
						Credential:                 &syscall.Credential{Uid: uint32(1001), Gid: uint32(1001)},
						GidMappingsEnableSetgroups: true,
					}

					outf, err := cmd.StdoutPipe()
					if err != nil {
						klog.Errorf("Error StdoutPipe: %s......", err.Error())
					}
					errf, err := cmd.StderrPipe()
					if err != nil {
						klog.Errorf("Error StderrPipe: %s......", err.Error())
					}

					go asyncLog(errf, "dmwatcher")
					go asyncLog(outf, "dmwatcher")

					dmWatcher = cmd
					err = cmd.Run()
					if err != nil {
						klog.Errorf("Error dmwatcher starting command: %s......", err)
					}
				}

			}
		} else {
			klog.Infof("dmwatcher is running...")
		}
	}()

	return dmwatcherErr
}

func (service Service) DmwatcherQuit(context *gin.Context) error {
	//判断数据库是否已经启动
	dmwatcherStart := dmWatcher != nil
	klog.Infof("dmwatcherStart: %s......", dmwatcherStart)
	if dmwatcherStart {
		err := syscall.Kill(-dmWatcher.Process.Pid, syscall.SIGQUIT)
		if err != nil {
			if err.Error() == "no such process" {
				klog.Warningf("DmwatcherQuit warning: %s......", err.Error())
				return nil
			}
			klog.Errorf("DmwatcherQuit Error: %s......", err.Error())
			return err
		}
		//添加数据库停止指令
		quitDmwatcher <- "quit"
		//清除旧的dmWatcher
		dmWatcher = nil
	} else {
		klog.Infof("DmwatcherQuit is not running...")
	}

	return nil
}

func (service Service) DmserverStatus(context *gin.Context) error {
	cmd := `ps aux | grep  "dmserver" | grep -v grep`
	result, err := exec.Command("/bin/sh", "-c", cmd).Output()
	out := strings.TrimSpace(string(result))
	if err != nil {
		klog.Errorf("Check dmserver status err: %s... ", err)
		return err
	}
	klog.Infof("DmserverStatusOut: %s", out)

	if out == "" {
		klog.Infof("dmserver is not running... ")
		return errors.New("dmserver is not running...")
	} else {
		klog.Infof("dmserver is running...")
		return nil
	}
}

func (service Service) Backup(context *gin.Context) error {

	return nil
}
