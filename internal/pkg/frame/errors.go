/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 陈磊
      E-mail: chenlei@dameng.com
      Create: 2020/12/21 11:14
     Project: keel
     Package: frame
    Describe: Todo
*/
package frame

import "net/http"

type Error struct {
	error
	httpCode int
}

func NewError(httpCode int, err error) *Error {
	return &Error{
		error:    err,
		httpCode: httpCode,
	}
}

func DefaultError(err error) *Error {
	return &Error{
		error:    err,
		httpCode: http.StatusInternalServerError,
	}
}

func BadRequestError(err error) *Error {
	return &Error{
		error:    err,
		httpCode: http.StatusBadRequest,
	}
}

func NotFoundError(err error) *Error {
	return &Error{
		error:    err,
		httpCode: http.StatusNotFound,
	}
}
