package sequence

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

const DBStoreTableName = "sys_sequence_rule"

// RuleModel is the internal ORM model for the sys_sequence_rule table.
type RuleModel struct {
	orm.BaseModel `bun:"table:sys_sequence_rule,alias:ssr"`
	orm.FullAuditedModel

	Key              string           `bun:"key,notnull,unique"`
	Name             string           `bun:"name,notnull"`
	Prefix           *string          `bun:"prefix,type:varchar(32)"`
	Suffix           *string          `bun:"suffix,type:varchar(32)"`
	DateFormat       *string          `bun:"date_format,type:varchar(32)"`
	SeqLength        int16            `bun:"seq_length,notnull,default:4"`
	SeqStep          int16            `bun:"seq_step,notnull,default:1"`
	StartValue       int              `bun:"start_value,notnull,default:0"`
	MaxValue         int              `bun:"max_value,notnull,default:0"`
	OverflowStrategy OverflowStrategy `bun:"overflow_strategy,notnull,default:'error'"`
	ResetCycle       ResetCycle       `bun:"reset_cycle,notnull,default:'N'"`
	CurrentValue     int              `bun:"current_value,notnull,default:0"`
	LastResetAt      *timex.DateTime  `bun:"last_reset_at,type:timestamp"`
	IsActive         bool             `bun:"is_active,notnull,default:true"`
	Remark           *string          `bun:"remark,type:varchar(256)"`
}

// toRule converts the ORM model to the public Rule type.
func (m *RuleModel) toRule() *Rule {
	return &Rule{
		Key:              m.Key,
		Name:             m.Name,
		Prefix:           lo.FromPtr(m.Prefix),
		Suffix:           lo.FromPtr(m.Suffix),
		DateFormat:       lo.FromPtr(m.DateFormat),
		SeqLength:        int(m.SeqLength),
		SeqStep:          int(m.SeqStep),
		StartValue:       m.StartValue,
		MaxValue:         m.MaxValue,
		OverflowStrategy: m.OverflowStrategy,
		ResetCycle:       m.ResetCycle,
		CurrentValue:     m.CurrentValue,
		LastResetAt:      m.LastResetAt,
		IsActive:         m.IsActive,
	}
}

// DBStore implements Store using a relational database.
// Table name is fixed to sys_sequence_rule.
type DBStore struct {
	db orm.DB
}

// NewDBStore creates a new database-backed sequence store.
func NewDBStore(db orm.DB) Store {
	return &DBStore{db: db}
}

// Init creates the sys_sequence_rule table if it does not exist.
// Implements contract.Initializer.
func (s *DBStore) Init(ctx context.Context) error {
	if _, err := s.db.NewCreateTable().
		Model((*RuleModel)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to create sequence rule table %q: %w", DBStoreTableName, err)
	}

	logger.Infof("Sequence rule table %q ensured", DBStoreTableName)

	return nil
}

func (s *DBStore) Reserve(ctx context.Context, key string, count int, now timex.DateTime) (*Rule, int, error) {
	var (
		reservedRule *Rule
		newValue     int
	)

	if err := s.db.RunInTX(ctx, func(ctx context.Context, tx orm.DB) error {
		var model RuleModel

		if err := tx.NewSelect().
			Model(&model).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("key", key).
					IsTrue("is_active")
			}).
			ForUpdate().
			Scan(ctx); err != nil {
			if result.IsRecordNotFound(err) {
				return ErrRuleNotFound
			}

			return fmt.Errorf("failed to select sequence rule %q for reservation: %w", key, err)
		}

		rule := model.toRule()

		resetNeeded, err := evaluateReserve(rule, count, now)
		if err != nil {
			return err
		}

		if resetNeeded {
			rule.CurrentValue = rule.StartValue
			resetAt := now
			rule.LastResetAt = &resetAt
		}

		rule.CurrentValue += rule.SeqStep * count
		newValue = rule.CurrentValue

		if _, err := tx.NewUpdate().
			Model((*RuleModel)(nil)).
			Set("current_value", newValue).
			ApplyIf(resetNeeded, func(query orm.UpdateQuery) {
				query.Set("last_reset_at", rule.LastResetAt)
			}).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(model.ID)
			}).
			Exec(ctx); err != nil {
			return fmt.Errorf("failed to persist sequence reservation %q: %w", key, err)
		}

		reservedRule = rule

		return nil
	}); err != nil {
		return nil, 0, fmt.Errorf("failed to reserve sequence %q: %w", key, err)
	}

	return reservedRule, newValue, nil
}
