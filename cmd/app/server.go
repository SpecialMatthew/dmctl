/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/6 14:57
     Project: dmctl
     Package: app
    Describe: Todo
*/

package app

import (
	"dmctl/internal/app/controller"
	"dmctl/tools"
	"k8s.io/klog/v2"
	"net/http"
	"time"
)

func NewServer() *http.Server {
	timeout, err := time.ParseDuration(tools.GetEnv("REQUEST_TIMEOUT_SECONDS", "30s"))
	if err != nil {
		klog.Errorf("timeout parse error: %v", err)
		return nil
	}

	return &http.Server{
		Addr:              ":80",                         //Addr可选地指定服务器要监听的TCP地址，形式为“host:port”。如果为空，则使用“:http”(端口80)
		Handler:           controller.NewServerHandler(), //**调用的处理程序**
		TLSConfig:         nil,                           //可选地为ServeTLS和ListenAndServeTLS提供使用的TLS配置
		ReadTimeout:       timeout,                       //读取整个请求(包括请求体)的最大持续时间。
		ReadHeaderTimeout: 0,                             //ReadHeaderTimeout是允许读取请求头的时间。如果ReadHeaderTimeout为0，则使用ReadTimeout的值。如果两者都为零，则不存在超时。
		WriteTimeout:      timeout,                       //响应写超时之前的最大持续时间
		IdleTimeout:       0,                             //启用keep-alive时等待下一个请求的最大时间。如果IdleTimeout为0，则使用ReadTimeout的值。如果两者都为零，则不存在超时。
		MaxHeaderBytes:    1 << 20,                       //控制服务器解析请求头的键和值(包括请求行)时读取的最大字节数,1 << 20 十进制的值为1048576
		TLSNextProto:      nil,                           //
		ConnState:         nil,                           //一个可选的回调函数，当客户端连接改变状态时被调用
		ErrorLog:          nil,                           //指定一个可选的日志记录器，用于接收连接错误、处理程序的意外行为和底层文件系统错误。如果为nil，日志记录是通过日志包的标准日志记录程序来完成的
		BaseContext:       nil,                           //可选地指定一个函数，该函数返回此服务器上传入请求的基本上下文,如果BaseContext为nil，默认值为context.Background()
		ConnContext:       nil,
	}
}
