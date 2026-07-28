package orm_test

import (
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &AuditTimestampTestSuite{BaseTestSuite: base}
	})
}

// AuditTimestampTestSuite pins the audit columns to every UPDATE shape the query
// builder exposes: whole-model, Set-based, and the Select(...) column whitelist.
// The whitelist path is the one that used to drop updated_at silently.
type AuditTimestampTestSuite struct {
	*BaseTestSuite
}

// staleMark is the deliberately old audit timestamp seeded on insert, so a
// refreshed updated_at is detectable without depending on clock resolution.
var staleMark = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// seedStaleUser inserts a user whose updated_at is already far in the past.
// The insert handlers only fill audit timestamps that are zero, so the stale
// mark survives.
func (suite *AuditTimestampTestSuite) seedStaleUser(email string) *User {
	user := &User{Name: "AT Probe", Email: email, Age: 40, IsActive: true}
	user.CreatedAt = timex.DateTime(staleMark)
	user.UpdatedAt = timex.DateTime(staleMark)

	_, err := suite.db.NewInsert().Model(user).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert probe user")

	suite.Require().True(
		time.Time(user.UpdatedAt).Equal(staleMark),
		"Insert must preserve an explicitly supplied updated_at",
	)

	return user
}

// reloadUser reads the persisted row, bypassing whatever the query builder left
// in the in-memory model.
func (suite *AuditTimestampTestSuite) reloadUser(id string) *User {
	var stored User

	err := suite.db.NewSelect().
		Model(&stored).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(id)
		}).
		Scan(suite.ctx)
	suite.Require().NoError(err, "Failed to reload probe user")

	return &stored
}

func (suite *AuditTimestampTestSuite) TearDownTest() {
	_, _ = suite.db.NewDelete().Model((*User)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Contains("email", "at_probe")
	}).Exec(suite.ctx)

	_, _ = suite.db.NewDelete().Model((*Tag)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Contains("name", "AT Probe Tag")
	}).Exec(suite.ctx)
}

// TestUpdatedAtRefresh asserts updated_at reaches the database on every update
// shape. All three must behave identically — the audit column is a property of
// the table, not of the way the caller happened to phrase the UPDATE.
func (suite *AuditTimestampTestSuite) TestUpdatedAtRefresh() {
	suite.Run("WholeModel", func() {
		user := suite.seedStaleUser("at_probe_whole@test.com")
		user.Name = "AT Probe Renamed"

		_, err := suite.db.NewUpdate().
			Model(user).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Whole-model update must succeed")

		stored := suite.reloadUser(user.ID)
		suite.True(
			time.Time(stored.UpdatedAt).After(staleMark),
			"Whole-model update must refresh updated_at in the database",
		)
	})

	suite.Run("SetClause", func() {
		user := suite.seedStaleUser("at_probe_set@test.com")

		_, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("name", "AT Probe Renamed").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Set-based update must succeed")

		stored := suite.reloadUser(user.ID)
		suite.True(
			time.Time(stored.UpdatedAt).After(staleMark),
			"Set-based update must refresh updated_at in the database",
		)
	})

	suite.Run("SelectedColumns", func() {
		user := suite.seedStaleUser("at_probe_select@test.com")
		user.Name = "AT Probe Renamed"

		_, err := suite.db.NewUpdate().
			Model(user).
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Column-whitelist update must succeed")

		stored := suite.reloadUser(user.ID)
		suite.Equal("AT Probe Renamed", stored.Name, "Whitelisted column must be written")
		suite.True(
			time.Time(stored.UpdatedAt).After(staleMark),
			"Column-whitelist update must refresh updated_at in the database",
		)
	})

	// A caller that already names updated_at must not get it assigned twice —
	// PostgreSQL rejects multiple assignments to the same column outright.
	suite.Run("SelectedColumnsAlreadyIncludingUpdatedAt", func() {
		user := suite.seedStaleUser("at_probe_select_dup@test.com")
		user.Name = "AT Probe Renamed"

		_, err := suite.db.NewUpdate().
			Model(user).
			Select("name", "updated_at").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Whitelisting updated_at explicitly must not assign it twice")

		stored := suite.reloadUser(user.ID)
		suite.Equal("AT Probe Renamed", stored.Name, "Whitelisted column must be written")
		suite.True(
			time.Time(stored.UpdatedAt).After(staleMark),
			"Explicitly whitelisted updated_at must still be refreshed",
		)
	})
}

// TestExplicitAuditValueWins asserts the auto-column handlers defer to a caller
// that assigns an audit column itself — stating a value is stating intent.
func (suite *AuditTimestampTestSuite) TestExplicitAuditValueWins() {
	pinned := time.Date(2011, 2, 3, 4, 5, 6, 0, time.UTC)

	suite.Run("UpdatedAt", func() {
		user := suite.seedStaleUser("at_probe_explicit_at@test.com")

		_, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("name", "AT Probe Renamed").
			Set("updated_at", pinned).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Explicit updated_at update must succeed")

		suite.True(
			time.Time(suite.reloadUser(user.ID).UpdatedAt).Equal(pinned),
			"An explicitly set updated_at must not be overwritten by the handler",
		)
	})

	suite.Run("UpdatedBy", func() {
		user := suite.seedStaleUser("at_probe_explicit_by@test.com")

		_, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "explicit-operator").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Explicit updated_by update must succeed")

		suite.Equal("explicit-operator", suite.reloadUser(user.ID).UpdatedBy,
			"An explicitly set updated_by must not be overwritten by the handler")
	})
}

// TestUpdatedByRefresh mirrors TestUpdatedAtRefresh for updated_by, so the two
// audit columns cannot drift apart across update shapes.
func (suite *AuditTimestampTestSuite) TestUpdatedByRefresh() {
	const operator = "at-probe-operator"

	scopedDB := suite.db.WithNamedArg(orm.PlaceholderKeyOperator, operator)

	suite.Run("SetClause", func() {
		user := suite.seedStaleUser("at_probe_by_set@test.com")

		_, err := scopedDB.NewUpdate().
			Model((*User)(nil)).
			Set("name", "AT Probe Renamed").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Set-based update must succeed")

		suite.Equal(operator, suite.reloadUser(user.ID).UpdatedBy,
			"Set-based update must stamp updated_by")
	})

	suite.Run("SelectedColumns", func() {
		user := suite.seedStaleUser("at_probe_by_select@test.com")
		user.Name = "AT Probe Renamed"

		_, err := scopedDB.NewUpdate().
			Model(user).
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Column-whitelist update must succeed")

		suite.Equal(operator, suite.reloadUser(user.ID).UpdatedBy,
			"Column-whitelist update must stamp updated_by")
	})
}

// TestAuditlessModelUnaffected guards the fix against over-reach: a model with
// no audit columns must not gain them on any update shape.
func (suite *AuditTimestampTestSuite) TestAuditlessModelUnaffected() {
	tag := &Tag{Name: "AT Probe Tag"}
	tag.ID = "at-probe-tag-1"

	_, err := suite.db.NewInsert().Model(tag).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert probe tag")

	suite.Run("SelectedColumns", func() {
		tag.Name = "AT Probe Tag Renamed"

		_, err := suite.db.NewUpdate().
			Model(tag).
			Select("name").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(tag.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Column-whitelist update on an audit-less model must succeed")

		var stored Tag

		err = suite.db.NewSelect().
			Model(&stored).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(tag.ID)
			}).
			Scan(suite.ctx)
		suite.Require().NoError(err, "Failed to reload probe tag")
		suite.Equal("AT Probe Tag Renamed", stored.Name, "Whitelisted column must be written")
	})

	suite.Run("WholeModel", func() {
		tag.Name = "AT Probe Tag Again"

		_, err := suite.db.NewUpdate().
			Model(tag).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(tag.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Whole-model update on an audit-less model must succeed")
	})
}
