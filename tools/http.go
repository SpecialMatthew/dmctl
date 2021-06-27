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
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// Get
// 发送GET请求
// url：         请求地址
// response：    请求返回的内容
func Get(url string) (*http.Response, error) {
	// 超时时间：5秒
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	return resp, err
}

// Post
// 发送POST请求
// url：         请求地址
// data：        POST请求提交的数据
// contentType： 请求体格式，如：application/json
// content：     请求放回的内容
func Post(url string, data interface{}, contentType string) (*http.Response, error) {
	// 超时时间：5秒
	client := &http.Client{Timeout: 5 * time.Second}
	jsonStr, _ := json.Marshal(data)
	resp, err := client.Post(url, contentType, bytes.NewBuffer(jsonStr))
	return resp, err
}
