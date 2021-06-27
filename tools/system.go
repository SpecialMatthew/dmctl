/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 14:59
     Project: dmctl
     Package: tools
    Describe: Todo
*/

package tools

import (
	"dmctl/internal/pkg/business/v1/common/typed"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func GetEnv(name, def string) string {
	env, found := os.LookupEnv(name)
	if found {
		return env
	}
	return def
}

// CreateDir 创建文件夹(支持创建嵌套文件夹)
func CreateDir(filePath string) error {
	exist, err := PathExists(filePath)
	if err != nil {
		klog.Errorf("get dir error![%v]\n", err)
		return err
	}

	if exist {
		klog.Infof("has dir![%v]\n", filePath)
	} else {
		// 创建文件夹
		err := os.MkdirAll(filePath, os.ModePerm)
		if err != nil {
			klog.Errorf("mkdir %s failed![%v]\n", filePath, err)
			return err
		}
		klog.Infof("mkdir [%v] success!\n", filePath)
	}
	return nil
}

// PathExists 判断文件夹是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CreateFile 创建文件，写入内容（可选）,是否覆盖原文件
func CreateFile(fileName string, fileContent string, override bool) error {
	exist, err := PathExists(fileName)
	if err != nil {
		klog.Errorf("get file error![%v]\n", err)
		return err
	}
	if exist && !override {
		klog.Infof("has file![%v]\n", fileName)
	} else {
		file, err := os.Create(fileName)
		if err != nil {
			klog.Infof("create file %s error: %s", fileName, err)
			return err
		}
		defer file.Close()
		if fileContent != "" {
			_, err := file.WriteString(fileContent)
			if err != nil {
				klog.Infof("write content to file %s error: %s", fileName, err)
				return err
			}
		}
		err = os.Chmod(fileName, os.ModePerm)
		if err != nil {
			klog.Errorf("chmod file %s error![%v]\n", fileName, err)
			return err
		}
		klog.Infof("create file %s success", fileName)
	}
	return nil
}

// WriteToFile 将内容以覆盖的形式写入文件
func WriteToFile(fileName string, content string) error {
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println("file open failed. err: " + err.Error())
	} else {
		// offset
		//os.Truncate(filename, 0) //clear
		n, _ := f.Seek(0, io.SeekEnd)
		_, err = f.WriteAt([]byte(content), n)
		fmt.Println("%s override write succeed!", fileName)
		defer f.Close()
	}
	return err
}

// ReadFile 一次性读取文件内容(文件不能太大)
func ReadFile(fileName string) (string, error) {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		klog.Errorf("ReadFile file[%v] error: %v", fileName, err)
		return "", err
	}
	return string(bytes), err
}

func ConfigArr2Map(arrs []*typed.ConfigValue, maps map[string]*typed.ConfigValue) map[string]*typed.ConfigValue {
	for _, v := range arrs {
		maps[v.Name] = v
	}
	return maps
}

func Files(path, pattern string) (files []string) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		klog.Errorf("compile regex error: %v", err)
		return nil
	}
	if err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err == nil && regex.MatchString(info.Name()) {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		klog.Errorf("recursive file error: %v", err)
		return nil
	}
	return files
}

func GetDbPort() (*string, error) {
	instancePath := GetEnv("DM_INIT_PATH", GetEnv("DM_HOME", "/opt/dmdbms")+"/data") + "/" + GetEnv("DM_INIT_DB_NAME", "DAMENG")
	dmIniExist, err := PathExists(instancePath + "/dm.ini")
	if err != nil {
		klog.Errorf("get dm.ini error: %s......", err)
	}

	if dmIniExist {
		//去掉文件中每行开头tab
		formatConfigCmdStr := "sed -i 's/^\t*//g' " + instancePath + "/dm.ini"
		klog.Infof("format configFile command: %s", formatConfigCmdStr)
		cmd := exec.Command("bash", "-c", formatConfigCmdStr)
		err = cmd.Run()
		if err != nil {
			klog.Errorf("format configFile dm.ini command error: %s......", err)
			return nil, err
		}

		//获取db_port
		getPortNumCmdStr := `res=$(sed -r -n '/^PORT_NUM/'p ` + instancePath + `/dm.ini);res=${res#*=};res=${res%%#*};echo $res`
		klog.Infof("getPortNumCmd : %s", getPortNumCmdStr)
		getPortNumCmd := exec.Command("bash", "-c", getPortNumCmdStr)
		portNumBytes, err := getPortNumCmd.CombinedOutput()
		if err != nil {
			klog.Errorf("getPortNum error: %s......", err)
			return nil, err
		}
		dbPort := string(portNumBytes)
		dbPort = strings.Trim(dbPort, "\n")
		klog.Infof("DB_PORT is [%s]", dbPort)
		return &dbPort, nil
	} else {
		klog.Errorf("dm.ini has yet created!")
		return nil, errors.New("dm.ini has yet created!")
	}

}
