/*
     Company: 达梦数据库有限公司
  Department: 达梦技术公司/产品研发中心
      Author: 毕艺翔
      E-mail: byx@dameng.com
      Create: 2021/5/10 13:34
     Project: dmctl
     Package: typed
    Describe: Todo
*/

package typed

type ConfigValue struct {
	Group string `json:"group"`
	Value string `json:"value"`
	Type  string `json:"type"`
}
