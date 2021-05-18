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
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"os"
)

func GetEnv(name, def string) string {
	env, found := os.LookupEnv(name)
	if found {
		return env
	}
	return def
}

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
		err := os.Mkdir(filePath, os.ModePerm)
		if err != nil {
			klog.Infof("mkdir failed![%v]\n", err)
			return err
		} else {
			klog.Infof("mkdir success!\n")
			return err
		}
	}
	return nil
}

// 判断文件夹是否存在
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

//创建文件，写入内容（可选）,是否覆盖原文件
func CreateFile(fileName string, fileContent string, override bool) error {
	exist, err := PathExists(fileName)
	if err != nil {
		klog.Errorf("get dir error![%v]\n", err)
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
		klog.Infof("create file %s success", fileName)
	}
	return nil
}

//将内容以覆盖的形式写入文件
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

func ConfigArr2Map(arrs []*typed.ConfigValue, maps map[string]*typed.ConfigValue) map[string]*typed.ConfigValue {
	for _, v := range arrs {
		maps[v.Name] = v
	}
	return maps
}
