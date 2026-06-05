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

// QuotaNetNode stores a registered compute client node.
type QuotaNetNode struct {
	ent.Schema
}

func (QuotaNetNode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_nodes"},
	}
}

func (QuotaNetNode) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (QuotaNetNode) Fields() []ent.Field {
	return []ent.Field{
		field.String("node_key").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.String("name").
			MaxLen(100).
			Default(""),
		field.Int64("owner_user_id").
			Optional().
			Nillable(),
		field.String("wallet_address").
			MaxLen(128).
			NotEmpty(),
		field.String("token_hash").
			MaxLen(128).
			NotEmpty(),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.String("protocol_version").
			MaxLen(40).
			Optional().
			Nillable(),
		field.String("client_version").
			MaxLen(40).
			Optional().
			Nillable(),
		field.Time("last_seen_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (QuotaNetNode) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("node_key").Unique(),
		index.Fields("wallet_address"),
		index.Fields("status"),
		index.Fields("last_seen_at"),
		index.Fields("owner_user_id"),
	}
}
