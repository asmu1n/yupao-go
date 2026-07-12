package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type User struct {
	ent.Schema
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user"},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Positive().
			Immutable(),
		field.String("username").
			MaxLen(256).
			Optional(),
		field.String("user_account").
			MaxLen(256).
			Optional().
			Unique(),
		field.String("avatar_url").
			MaxLen(1024).
			Optional(),
		field.Int8("gender").
			Optional(),
		field.String("user_password").
			MaxLen(512).
			Sensitive(),
		field.String("phone").
			MaxLen(128).
			Optional(),
		field.String("email").
			MaxLen(512).
			Optional(),
		field.Int("user_status").
			Default(0),
		field.Time("create_time").
			Default(time.Now).
			Immutable(),
		field.Time("update_time").
			Default(time.Now).
			UpdateDefault(time.Now),
		field.Int8("is_delete").
			Default(0),
		field.Int("user_role").
			Default(0),
		field.String("planet_code").
			MaxLen(512).
			Optional(),
		field.String("tags").
			MaxLen(1024).
			Optional(),
	}
}

func (User) Edges() []ent.Edge {
	return nil
}
