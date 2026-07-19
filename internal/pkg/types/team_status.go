package types

import "fmt"

// TeamStatus 队伍可见性状态。
type TeamStatus int

const (
	TeamStatusPublic  TeamStatus = 0 // 公开
	TeamStatusPrivate TeamStatus = 1 // 私有
	TeamStatusSecret  TeamStatus = 2 // 加密
)

// Validate 校验是否为合法状态值。
func (s TeamStatus) Validate() error {
	switch s {
	case TeamStatusPublic, TeamStatusPrivate, TeamStatusSecret:
		return nil
	default:
		return fmt.Errorf("invalid team status: %d", s)
	}
}

// Valid 是否为合法状态（便于 if 判断）。
func (s TeamStatus) Valid() bool {
	return s.Validate() == nil
}
