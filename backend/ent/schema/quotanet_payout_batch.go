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

// QuotaNetPayoutBatch groups contribution ledger rows for settlement.
type QuotaNetPayoutBatch struct {
	ent.Schema
}

func (QuotaNetPayoutBatch) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_payout_batches"},
	}
}

func (QuotaNetPayoutBatch) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (QuotaNetPayoutBatch) Fields() []ent.Field {
	return []ent.Field{
		field.String("batch_key").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.Time("window_start").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("window_end").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("status").
			MaxLen(20).
			Default("draft"),
		field.String("network").
			MaxLen(40).
			Default("solana-devnet"),
		field.Int64("total_token_flow").
			Default(0),
		field.Float("total_contribution_usd").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Comment("Total QuotaNet contribution amount in USD"),
		field.Float("total_amount_cxs").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,12)"}),
		field.Int("item_count").
			Default(0),
		field.Int64("created_by").
			Optional().
			Nillable(),
		field.Int64("approved_by").
			Optional().
			Nillable(),
	}
}

func (QuotaNetPayoutBatch) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("batch_key").Unique(),
		index.Fields("status"),
		index.Fields("window_start", "window_end"),
	}
}
