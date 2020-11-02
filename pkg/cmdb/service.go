package cmdb

/**
	CMDB 核心抽象概念，借鉴阿里开源nacos

	Entity (实体), 承载数据的最小单元，类似 IP 、 APP、服务

	Entity Type (实体类型)

	Label (标签)，承载实体属性的K/V对。

	Entity Event (实体事件)

**/

type CMDBService interface {
	// 获取标签列表
	GetLabelNames() []string

	// 获取实体类型集合 (IP/APP/Service...)
	GetEntityTypes() []string

	// 获取标签详情
	GetLabel(labelName string) Label

	// 获取实体标签值
	GetLabelValue(entityName string, entityType string, labelName string) string

	// 获取实体所有标签值
	GetLabelValues(entityName string, entityType string) map[string]string

	// 获取所有实体
	GetAllEntities() map[string]map[string]Entity

	// 获取指定时间戳开始的事件
	GetEntityEvents(timestamp int64) []EntityEvent

	// 获取指定实体
	GetEntity(entityName string, entityType string) Entity
}
