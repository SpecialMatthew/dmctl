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
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

//静态， 可以被动态修改， 修改后重启服务器才能生效。
//动态， 可以被动态修改， 修改后即时生效。 动态参数又分为会话级和系统级两种。会话
//级参数被修改后， 新参数值只会影响新创建的会话， 之前创建的会话不受影响；系统级参数
//的修改则会影响所有的会话。
//手动， 不能被动态修改， 必须手动修改 dm.ini 参数文件，然后重启才能生效。
//动态修改是指 DBA 用户可以在数据库服务器运行期间，通过调用系统过程
//SP_SET_PARA_VALUE()、 SP_SET_PARA_DOUBLE_VALUE()和
//SP_SET_PARA_STRING_VALUE()对参数值进行修改。

type DmIni struct {
	Name         string `json:"name"`
	Attribute    int    `json:"attribute"` //静态：0 ，手动：1 ，动态-会话级：2，动态-系统级：3
	DefaultValue string `json:"defaultValue"`
	ValueType    string `json:"valueType"`
	Describe     string `json:"describe"`
	Group        string `json:"group"`
}
