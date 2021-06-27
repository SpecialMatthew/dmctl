package tools

import (
	"bytes"
	"dmctl/internal/pkg/business/v1/common/typed"
	"github.com/Masterminds/sprig/v3"
	"k8s.io/klog/v2"
	"strconv"
	"text/template"
)

func ParseTemplate(name string, parameters interface{}) (string, error) {
	templates, err := template.New("default").Funcs(sprig.TxtFuncMap()).Funcs(buildFunctionMap()).ParseFiles(Files(GetEnv("TEMPLATES_PATH", "D:\\go_path\\src\\dmctl\\templates"), "\\.gotmpl$")...)
	if err != nil {
		klog.Errorf("parse template error: %v", err)
		return "", err
	}
	buffer := new(bytes.Buffer)
	if err := templates.ExecuteTemplate(buffer, name, parameters); err != nil {
		klog.Errorf("template execute error: %v", err)
		return "", err
	}
	return buffer.String(), nil
}

func buildFunctionMap() template.FuncMap {
	return template.FuncMap{
		"get": func(object map[string]*typed.ConfigValue, index int, groupPrefix string, paramName string) string {
			indexStr := strconv.Itoa(index + 1)
			name := groupPrefix + indexStr + "_" + paramName
			return object[name].Value
		},
		"repeatGet": func(object map[string]*typed.ConfigValue, index int, groupParam string) string {
			indexStr := strconv.Itoa(index)
			name := groupParam + "-" + indexStr
			return object[name].Value
		},
	}
}
