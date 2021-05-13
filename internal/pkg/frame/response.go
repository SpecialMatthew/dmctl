// Package frame
/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 陈磊
      E-mail: chenlei@dameng.com
      Create: 2020/12/22 11:17
     Project: keel
     Package: frame
    Describe: Todo
*/
package frame

import (
	"net/http"
)

type Response struct {
	httpCode int
	payload  interface{}
}

func OK(response interface{}) *Response {
	return &Response{
		httpCode: http.StatusOK,
		payload:  response,
	}
}
