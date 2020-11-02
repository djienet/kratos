package cmdb

// 实体类型
type Entity struct {
	Name   string
	Type   string
	Labels map[string]string
}

// 实体事件类型
type EntityEventType int32

const (
	ENTITY_ADD    EntityEventType = 1
	ENTITY_UPDATE EntityEventType = 2
	ENTITY_REMOVE EntityEventType = 3
)

// 实体事件
type EntityEvent struct {
	Type       EntityEventType
	EntityName string
	EntityType string
}

// 标签
type Label struct {
	Name        string
	Values      []string
	Description string
}
