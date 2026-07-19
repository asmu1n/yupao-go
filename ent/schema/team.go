package schema

import (
	"time"

	"yupao-go/internal/pkg/types"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Team 队伍实体。
type Team struct {
	ent.Schema
}

func (Team) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "team"},
	}
}

func (Team) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable(),
		field.String("name").
			MaxLen(256),
		field.String("description").
			MaxLen(1024).
			Optional().
			Nillable(),
		field.Int("max_num").
			Default(1),
		field.Time("expire_time").
			Optional().
			Nillable(),
		// 外键字段：关联队长 User；由 edge.Field 绑定
		field.Int64("user_id").
			Comment("队长用户 ID"),
		field.Int("status").
			Default(int(types.TeamStatusPublic)).
			GoType(types.TeamStatus(0)).
			Validate(func(v int) error {
				return types.TeamStatus(v).Validate()
			}).
			Comment("0 公开 1 私有 2 加密"),
		field.String("password").
			MaxLen(512).
			Optional().
			Nillable().
			Sensitive(),
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

func (Team) Edges() []ent.Edge {
	return []ent.Edge{
		// 队长：N 队伍 → 1 用户
		edge.From("leader", User.Type).
			Ref("led_teams").
			Field("user_id").
			Required().
			Unique(),
		// 成员关系：1 队伍 → N user_team
		edge.To("memberships", UserTeam.Type),
	}
}

func (Team) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
	}
}
