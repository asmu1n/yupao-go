package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserTeam 用户-队伍关系（中间表）。
type UserTeam struct {
	ent.Schema
}

func (UserTeam) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_team"},
	}
}

func (UserTeam) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable(),
		// 外键字段，分别绑定 user / team edge
		field.Int64("user_id"),
		field.Int64("team_id"),
		field.Time("join_time").
			Optional().
			Nillable(),
		field.Time("create_time").
			Default(time.Now).
			Immutable(),
		field.Time("update_time").
			Default(time.Now).
			UpdateDefault(time.Now),
		field.Int8("is_delete").
			Default(0),
	}
}

func (UserTeam) Edges() []ent.Edge {
	return []ent.Edge{
		// 成员用户：N 关系 → 1 用户
		edge.From("user", User.Type).
			Ref("team_memberships").
			Field("user_id").
			Required().
			Unique(),
		// 所属队伍：N 关系 → 1 队伍
		edge.From("team", Team.Type).
			Ref("memberships").
			Field("team_id").
			Required().
			Unique(),
	}
}

func (UserTeam) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("team_id"),
		// 同一用户同一队伍的有效关系由业务层 is_delete 控制；此处保留联合索引加速查询
		index.Fields("user_id", "team_id"),
	}
}
