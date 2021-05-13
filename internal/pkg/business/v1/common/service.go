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
	"dmctl/tools"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"k8s.io/klog/v2"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Service struct{}

var pauseSignal = make(chan string)
var exitVirtualListening = make(chan string)
var dmServer *exec.Cmd
var virtualListening *exec.Cmd

func asyncLog(reader io.ReadCloser, logTitle string) error {
	cache := ""
	buf := make([]byte, 1024, 1024)
	for {
		num, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				err = nil
			}
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
	go func() {
		//判断数据库是否已经启动
		dmStart := (dmServer != nil)
		klog.Infof("dmStart: %s......", dmStart)
		if !dmStart {
		Loop:
			for {
				select {
				case pauseSignal := <-pauseSignal:
					klog.Infof("pause dmserver: %s......", pauseSignal)
					break Loop
				default:
					virtualListeningStart := (virtualListening != nil)
					klog.Infof("virtualListeningStart: %s......", virtualListeningStart)
					if virtualListeningStart {
						virtualListeningProcessStart := (virtualListening.Process != nil)
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

					cmdStr := "cd /opt/dmdbms/bin && ./dmserver " + tools.GetEnv("DM_INIT_PATH", "/opt/dmdbms/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG") + "/dm.ini"
					klog.Infof("dmserver start command: %s", cmdStr)

					cmd := exec.Command("bash", "-c", cmdStr)
					//使创建的线程都在同一个线程组里面，便于停止线程及子线程
					cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
					}
				}

			}
		}
	}()

	return nil
}

func (service Service) DmserverPause(context *gin.Context) error {
	//判断数据库是否已经启动
	dmStart := (dmServer != nil)
	klog.Infof("dmStart: %s......", dmStart)
	if dmStart {
		err := syscall.Kill(-dmServer.Process.Pid, syscall.SIGKILL)
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
					cmd := exec.Command("nc", "-lp", tools.GetEnv("DM_INI_PORT_NUM", "5236"))
					//使创建的线程都在同一个线程组里面，便于停止线程及子线程
					cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	service.DmserverPause(context)
	service.DmserverStart(context, params)
	return nil
}

func (service Service) ExecSql(context *gin.Context) error {
	port := tools.GetEnv("DM_INI_PORT_NUM", "5236")
	_, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		klog.Infof("dmserver has yet to start, can not exec sql now")
		return err
	}

	sql := context.PostForm("sql")
	klog.Infof("exec sql: %s", sql)

	err = tools.CreateFile("/tmp/everything.sql", sql)
	if err != nil {
		klog.Infof("create /tmp/everything.sql error: %s", err)
		return err
	}
	execCmdStr := "echo exit; >> /tmp/everything.sql && cd /opt/dmdbms/bin && ./disql SYSDBA/'\"" + tools.GetEnv("DM_INIT_SYSDBA_PWD", "Dameng7777") + "\"'@localhost:" + port + " '`/tmp/everything.sql'"
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
	port := tools.GetEnv("DM_INI_PORT_NUM", "5236")
	_, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		klog.Infof("dmserver has yet to start, can not exec sql now")
		return err
	}

	exist, err := tools.PathExists("/opt/dmdbms/script.d/genesis.sql")
	if err != nil {
		klog.Errorf("get /opt/dmdbms/script.d/genesis.sql error: %s......", err)
	}

	if exist {
		execCmdStr := "cat /opt/dmdbms/script.d/genesis.sql > /tmp/genesis.sql && echo exit; >> /tmp/genesis.sql && cd /opt/dmdbms/bin && ./disql SYSDBA/'\"" + tools.GetEnv("DM_INIT_SYSDBA_PWD", "Dameng7777") + "\"'@localhost:" + port + " '`/tmp/genesis.sql'"
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
	cmdStr := "cd /opt/dmdbms/bin && ./dminit "
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

func (service Service) Config(context *gin.Context, params map[string]*typed.ConfigValue) error {
	for name, v := range params {
		//获取待修改的文件路径
		configPath := tools.GetEnv("DM_INIT_PATH", "/opt/dmdbms/data") + "/" + tools.GetEnv("DM_INIT_DB_NAME", "DAMENG") + "/" + v.Type
		klog.Infof("configPath: %s", configPath)

		//去掉文件中每行开头tab
		formatConfigCmdStr := "sed -i 's/^\t*//g' " + configPath
		klog.Infof("format configFile command: %s", formatConfigCmdStr)
		cmd := exec.Command("bash", "-c", formatConfigCmdStr)
		err := cmd.Run()
		if err != nil {
			klog.Errorf("format configFile %s command error: %s......", v.Type, err)
			return err
		}

		if v.Group != "" {
			klog.Infof("ConfigParam: %s:%s:%s:%s", v.Type, v.Group, name, v.Value)
			//首先查看文件中有没有该分组,有就跳过，没有就创建
			checkGroupCmdStr := "res=$(sed -n '/^\\[" + v.Group + "\\]/'p " + configPath + ");[[ -z $res ]] && echo [" + v.Group + "] >> " + configPath + " || echo group exist"
			klog.Infof("check %s group exist command: %s", configPath, checkGroupCmdStr)
			checkGroupCmd := exec.Command("bash", "-c", checkGroupCmdStr)
			err := checkGroupCmd.Run()
			if err != nil {
				klog.Errorf("check %s group error: %s......", configPath, err)
				return err
			}
			//在属组下修改or新增参数
			editConfigCmdStr := "res=$(sed -n '/^" + name + "/'p " + configPath + ");[[ -n $res ]] && sed -i -r -e 's$^" + name + "(.*)$" + name + "=" + v.Value + " #" + v.Group + " #edit$' " + configPath + " || sed -i '/\\[" + v.Group + "\\]/a" + name + "=" + v.Value + " #" + v.Group + " #edit' " + configPath
			klog.Infof("edit %s command: %s", configPath, editConfigCmdStr)
			cmd := exec.Command("bash", "-c", editConfigCmdStr)
			err = cmd.Run()
			if err != nil {
				klog.Errorf("edit %s error: %s......", configPath, err)
				return err
			}
		} else {
			klog.Infof("ConfigParam: %s:%s:%s", v.Type, name, v.Value)
			//修改or新增参数
			editConfigCmdStr := "res=$(sed -n '/^" + name + "/'p " + configPath + ");[[ -n $res ]] && sed -i -r -e 's$^" + name + "(.*)$" + name + "=" + v.Value + " #edit$' " + configPath + " || echo " + name + "=" + v.Value + " #edit >>" + configPath
			klog.Infof("edit %s command: %s", configPath, editConfigCmdStr)
			cmd := exec.Command("bash", "-c", editConfigCmdStr)
			err := cmd.Run()
			if err != nil {
				klog.Errorf("edit %s error: %s......", configPath, err)
				return err
			}
		}
	}

	return nil
}

func (service Service) ListenPort(context *gin.Context, serviceType string, port string) error {
	for {
		_, err := net.Dial(serviceType, "localhost:"+port)
		if err != nil {
			klog.Infof("serviceType: %s ,port: %s has yet to start", serviceType, port)
			time.Sleep(time.Millisecond * 1500)
		} else {
			break
		}
	}
	klog.Infof("serviceType: %s ,port: %s has started", serviceType, port)
	return nil
}
