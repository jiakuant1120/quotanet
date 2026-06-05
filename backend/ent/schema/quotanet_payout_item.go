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

// QuotaNetPayoutItem stores a wallet-level settlement item in a payout batch.
type QuotaNetPayoutItem struct {
	ent.Schema
}

func (QuotaNetPayoutItem) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_payout_items"},
	}
}

func (QuotaNetPayoutItem) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (QuotaNetPayoutItem) Fields() []ent.Field {
	return []ent.Field{
		field.String("item_key").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.Int64("batch_id"),
		field.Int64("node_id").
			Optional().
			Nillable(),
		field.String("wallet_address").
			MaxLen(128).
			NotEmpty(),
		field.Int64("token_flow").
			Default(0),
		field.Float("amount_cxs").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,12)"}),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.String("tx_hash").
			MaxLen(128).
			Optional().
			Nillable(),
		field.String("error_message").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("finalized_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (QuotaNetPayoutItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("item_key").Unique(),
		index.Fields("batch_id"),
		index.Fields("wallet_address", "status"),
		index.Fields("tx_hash"),
	}
}
