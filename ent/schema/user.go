package schema

import (
	"time"

	"yupao-go/internal/shared/usertype"

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
			Optional().
			Nillable(),
		field.String("user_account").
			MaxLen(256).
			Unique(),
		field.String("avatar_url").
			MaxLen(1024).
			Optional().
			Nillable(),
		field.Int8("gender").
			Optional().
			Nillable().
			GoType(usertype.Gender(0)).
			Validate(func(v int8) error {
				return usertype.Gender(v).Validate()
			}),
		field.String("user_password").
			MaxLen(512).
			Sensitive(),
		field.String("phone").
			MaxLen(128).
			Optional().
			Nillable(),
		field.String("email").
			MaxLen(512).
			Optional().
			Nillable(),
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
			MaxLen(512),
		field.String("tags").
			MaxLen(1024).
			Optional(),
	}
}

func (User) Edges() []ent.Edge {
	return nil
}
