/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 14:06
     Project: dmctl
     Package: cmd
    Describe: Todo
*/
package main

import (
	"dmctl/cmd/app"
	"dmctl/internal/pkg/business/v1/common"
	"dmctl/internal/pkg/business/v1/distribute"
	"dmctl/tools"
	"flag"
	"k8s.io/klog/v2"
	"os"
)

func init() {
	_ = os.Setenv("DM_HOME", tools.GetEnv("DM_HOME", "/opt/dmdbms"))
	_ = os.Setenv("DM_INIT_ARCH_FLAG", tools.GetEnv("DM_INIT_ARCH_FLAG", "1"))
	_ = os.Setenv("DM_INIT_CASE_SENSITIVE", tools.GetEnv("DM_INIT_CASE_SENSITIVE", "1"))
	_ = os.Setenv("DM_INIT_CHARSET", tools.GetEnv("DM_INIT_CHARSET", "0"))
	_ = os.Setenv("DM_INIT_DB_NAME", tools.GetEnv("DM_INIT_DB_NAME", "DAMENG"))
	_ = os.Setenv("DM_INIT_EXTENT_SIZE", tools.GetEnv("DM_INIT_EXTENT_SIZE", "16"))
	_ = os.Setenv("DM_INIT_PAGE_SIZE", tools.GetEnv("DM_INIT_PAGE_SIZE", "8"))
	_ = os.Setenv("DM_INIT_SYSAUDITOR_PWD", tools.GetEnv("DM_INIT_SYSAUDITOR_PWD", "Dameng7777"))
	_ = os.Setenv("DM_INIT_PATH", tools.GetEnv("DM_INIT_PATH", tools.GetEnv("DM_HOME", "/opt/dmdbms")+"/data"))
	_ = os.Setenv("DM_INIT_SYSDBA_PWD", tools.GetEnv("DM_INIT_SYSDBA_PWD", "Dameng7777"))
	_ = os.Setenv("PERSISTENCE_LOGS", tools.GetEnv("PERSISTENCE_LOGS", "true"))
	_ = os.Setenv("BAK_USE_AP", tools.GetEnv("BAK_USE_AP", "2"))
}

func main() {
	//初始化k8s的日志工具
	klog.InitFlags(nil)

	flag.Set("logtostderr", "false")
	flag.Set("alsologtostderr", "true")
	flag.Set("log_file", "dmctl.log")
	flag.Set("add_dir_header", "true")
	//1. 关键字 defer 用于注册延迟调用。
	//2. 这些调用直到 return 前才被执。因此，可以用来做资源清理。
	//3. 多个defer语句，按先进后出的方式执行。
	//4. defer语句中的变量，在defer声明时就决定了。
	//用途：延迟调用刷新所有挂起的日志I/O的方法。
	defer klog.Flush()

	//把用户传递的命令行参数解析为对应变量的值
	flag.Parse()

	klog.Info(tools.BANNER)

	bootstrapModel := tools.GetEnv("BOOTSTRAP_MODEL", "single")
	inventory := `[]`

	go func() {
		svc := &distribute.Service{CommonService: &common.Service{}}
		switch bootstrapModel {
		case "single":
			klog.Info(tools.SINGLE)
			err := svc.Single(nil, inventory)
			if err != nil {
				klog.Errorf("Single Instance start error: %s......", err)
			}
		case "rww":
			klog.Info(tools.RWW)
			klog.Infof("distributing rww instance")
		case "ddw":
			klog.Info(tools.DDW)
			klog.Infof("distributing ddw instance")
		case "monitor":
			klog.Info(tools.MONITOR)
			klog.Infof("distributing monitor instance")
		}

	}()

	//应用的启动入口
	// 1.通过NewServer()方法创建一个http.Server实例
	// 2.使用ListenAndServe()方法启动并且监听服务
	// 3.如果服务启动失败则直接报出异常
	if err := app.NewServer().ListenAndServe(); err != nil {
		klog.Fatal(err)
		os.Exit(1)
	}
}
