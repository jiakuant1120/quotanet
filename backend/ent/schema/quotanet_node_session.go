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

// QuotaNetNodeSession stores one WebSocket connection from a QuotaNet node.
type QuotaNetNodeSession struct {
	ent.Schema
}

func (QuotaNetNodeSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_node_sessions"},
	}
}

func (QuotaNetNodeSession) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (QuotaNetNodeSession) Fields() []ent.Field {
	return []ent.Field{
		field.String("session_id").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.Int64("node_id"),
		field.String("instance_id").
			MaxLen(64).
			NotEmpty(),
		field.String("status").
			MaxLen(20).
			Default("connected"),
		field.String("remote_addr").
			MaxLen(128).
			Optional().
			Nillable(),
		field.Int("max_concurrency").
			Default(1),
		field.Int("current_concurrency").
			Default(0),
		field.Int("queue_size").
			Default(0),
		field.Int("max_queue_size").
			Default(0),
		field.JSON("capabilities", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("connected_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("disconnected_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_heartbeat_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("close_reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (QuotaNetNodeSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id").Unique(),
		index.Fields("node_id", "connected_at"),
		index.Fields("status"),
		index.Fields("instance_id"),
		index.Fields("last_heartbeat_at"),
	}
}
