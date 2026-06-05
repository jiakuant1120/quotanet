package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// QuotaNetTask stores a request routed to a QuotaNet node.
type QuotaNetTask struct {
	ent.Schema
}

func (QuotaNetTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_tasks"},
	}
}

func (QuotaNetTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (QuotaNetTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("task_id").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.String("request_id").
			MaxLen(64).
			NotEmpty(),
		field.Int64("user_id").
			Optional().
			Nillable(),
		field.Int64("api_key_id").
			Optional().
			Nillable(),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.Int64("account_id").
			Optional().
			Nillable(),
		field.Int64("node_id").
			Optional().
			Nillable(),
		field.String("session_id").
			MaxLen(64).
			Optional().
			Nillable(),
		field.String("platform").
			MaxLen(50).
			NotEmpty(),
		field.String("endpoint").
			MaxLen(100).
			NotEmpty(),
		field.String("model").
			MaxLen(100).
			NotEmpty(),
		field.Bool("stream").
			Default(false),
		field.String("status").
			MaxLen(20).
			Default("queued"),
		field.String("error_code").
			MaxLen(64).
			Optional().
			Nillable(),
		field.String("error_message").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int("prompt_tokens").
			Default(0),
		field.Int("completion_tokens").
			Default(0),
		field.Int("total_tokens").
			Default(0),
		field.Int("first_token_ms").
			Optional().
			Nillable(),
		field.Int("duration_ms").
			Optional().
			Nillable(),
		field.Time("dispatched_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("completed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (QuotaNetTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id").Unique(),
		index.Fields("request_id"),
		index.Fields("node_id", "created_at"),
		index.Fields("account_id", "created_at"),
		index.Fields("user_id", "created_at"),
		index.Fields("status", "created_at"),
		index.Fields("session_id"),
	}
}
