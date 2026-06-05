package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// QuotaNetTaskEvent stores append-only task protocol events.
type QuotaNetTaskEvent struct {
	ent.Schema
}

func (QuotaNetTaskEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_task_events"},
	}
}

func (QuotaNetTaskEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("task_id").
			MaxLen(64).
			NotEmpty(),
		field.String("event_type").
			MaxLen(50).
			NotEmpty(),
		field.Int64("sequence"),
		field.JSON("payload", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (QuotaNetTaskEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id", "sequence").
			Unique(),
		index.Fields("task_id", "created_at"),
		index.Fields("event_type"),
	}
}
