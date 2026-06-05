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

// QuotaNetContributionLedger records node earnings produced by completed tasks.
type QuotaNetContributionLedger struct {
	ent.Schema
}

func (QuotaNetContributionLedger) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "quotanet_contribution_ledger"},
	}
}

func (QuotaNetContributionLedger) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (QuotaNetContributionLedger) Fields() []ent.Field {
	return []ent.Field{
		field.String("task_id").
			MaxLen(64).
			NotEmpty().
			Unique(),
		field.Int64("usage_log_id").
			Optional().
			Nillable(),
		field.Int64("node_id"),
		field.String("wallet_address").
			MaxLen(128).
			NotEmpty(),
		field.Int64("account_id").
			Optional().
			Nillable(),
		field.String("platform").
			MaxLen(50).
			NotEmpty(),
		field.String("model").
			MaxLen(100).
			NotEmpty(),
		field.Int64("token_flow").
			Default(0),
		field.Float("amount_cxs").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,12)"}),
		field.Float("rate").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.Int64("payout_batch_id").
			Optional().
			Nillable(),
		field.Time("settled_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (QuotaNetContributionLedger) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id").Unique(),
		index.Fields("node_id", "created_at"),
		index.Fields("wallet_address", "status"),
		index.Fields("payout_batch_id"),
		index.Fields("status", "created_at"),
	}
}
