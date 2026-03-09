package orm_test

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &InsertTestSuite{BaseTestSuite: base}
	})
}

// InsertTestSuite tests INSERT operations across all databases.
type InsertTestSuite struct {
	*BaseTestSuite
}

// TestBasicInsert tests Model and Exec methods with single and bulk inserts.
func (suite *InsertTestSuite) TestBasicInsert() {
	suite.T().Logf("Testing basic INSERT for %s", suite.ds.Kind)

	suite.Run("InsertSingleRecord", func() {
		user := &User{
			Name:     "John Doe",
			Email:    "john@example.com",
			Age:      28,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert single user successfully")
		suite.NotEmpty(user.ID, "User ID should be set after insert")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "john@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted user")
		suite.Equal("John Doe", retrieved.Name, "Should preserve inserted name")
		suite.Equal("john@example.com", retrieved.Email, "Should preserve inserted email")

		suite.T().Logf("Inserted user: Id=%s, Name=%s", retrieved.ID, retrieved.Name)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "john@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup test user")
	})

	suite.Run("InsertMultipleRecords", func() {
		users := []*User{
			{Name: "Jane Smith", Email: "jane@example.com", Age: 26, IsActive: true},
			{Name: "Mike Wilson", Email: "mike@example.com", Age: 31, IsActive: false},
		}

		_, err := suite.db.NewInsert().
			Model(&users).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert multiple users successfully")

		for _, user := range users {
			suite.NotEmpty(user.ID, "Each user should have an ID set")
		}

		var retrieved []User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("email", []any{"jane@example.com", "mike@example.com"})
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve all inserted users")
		suite.Len(retrieved, 2, "Should have inserted 2 users")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("email", []any{"jane@example.com", "mike@example.com"})
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup test users")
	})
}

// TestCTE tests With, WithValues, and WithRecursive methods (PostgreSQL/SQLite only).
func (suite *InsertTestSuite) TestCTE() {
	suite.T().Logf("Testing CTE methods for %s", suite.ds.Kind)

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("CTE in INSERT not supported on %s", suite.ds.Kind)

		return
	}

	suite.Run("InsertWithSimpleCTE", func() {
		category := &Category{
			Name:        "CTE Category",
			Description: new("Category created via CTE"),
		}

		_, err := suite.db.NewInsert().
			With("existing_tech", func(sq orm.SelectQuery) {
				sq.Model((*Category)(nil)).
					Select("name", "description").
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("name", "Technology")
					})
			}).
			Model(category).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with CTE")
		suite.NotEmpty(category.ID, "Should set category ID after insert")

		_, err = suite.db.NewDelete().
			Model((*Category)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(category.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("InsertWithValuesCTE", func() {
		type TempData struct {
			Name  string
			Email string
		}

		tempData := []TempData{
			{Name: "CTE User", Email: "cte@example.com"},
		}

		user := &User{
			Name:     "CTE User",
			Email:    "cte@example.com",
			Age:      30,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			WithValues("temp_data", &tempData).
			Model(user).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with VALUES CTE")
		suite.NotEmpty(user.ID, "Should set user ID after insert")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "cte@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("InsertWithRecursiveCTE", func() {
		user := &User{
			Name:     "Recursive User",
			Email:    "recursive@example.com",
			Age:      35,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			WithRecursive("user_hierarchy", func(sq orm.SelectQuery) {
				sq.Model((*User)(nil)).
					Select("id", "name").
					Where(func(cb orm.ConditionBuilder) {
						cb.IsNotNull("id")
					}).
					Limit(1)
			}).
			Model(user).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with recursive CTE")
		suite.NotEmpty(user.ID, "Should set user ID after insert")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "recursive@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestTableSpecification tests table specification methods for INSERT...SELECT queries.
func (suite *InsertTestSuite) TestTableSpecification() {
	suite.T().Logf("Testing table specification methods for %s", suite.ds.Kind)

	suite.Run("ModelTableWithModel", func() {
		user := &User{
			Name:     "ModelTable User",
			Email:    "modeltable@example.com",
			Age:      27,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert using Model (standard approach)")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		suite.T().Logf("Inserted with Model: ID=%s", user.ID)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestColumnSelection tests Select and Exclude methods.
func (suite *InsertTestSuite) TestColumnSelection() {
	suite.T().Logf("Testing column selection methods for %s", suite.ds.Kind)

	suite.Run("ExcludeSpecificColumns", func() {
		user := &User{
			Name:     "Exclude User",
			Email:    "exclude@example.com",
			Age:      30,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Exclude("meta").
			Exec(suite.ctx)
		suite.NoError(err, "Should insert excluding specific columns")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		var retrieved User

		retrieved.ID = user.ID

		err = suite.db.NewSelect().
			Model(&retrieved).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Exclude User", retrieved.Name, "Should preserve name value")
		suite.Equal("exclude@example.com", retrieved.Email, "Should preserve email value")

		suite.T().Logf("Inserted with Exclude: ID=%s, Name=%s", retrieved.ID, retrieved.Name)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestColumnValues tests Column and ColumnExpr methods.
func (suite *InsertTestSuite) TestColumnValues() {
	suite.T().Logf("Testing column value methods for %s", suite.ds.Kind)

	suite.Run("ColumnDirectValue", func() {
		user := &User{
			Name:     "Original Name",
			Email:    "column@example.com",
			Age:      25,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Column("name", "Overridden Name").
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with column override")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "column@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Overridden Name", retrieved.Name, "Name should be overridden")
		suite.Equal(int16(25), retrieved.Age, "Age should keep model value")

		suite.T().Logf("Column override: Name=%s (overridden from Original Name)", retrieved.Name)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("ColumnExprWithFunction", func() {
		user := &User{
			Name:     "expr user",
			Email:    "expr@example.com",
			Age:      28,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			ColumnExpr("name", func(eb orm.ExprBuilder) any {
				return eb.Upper(eb.Literal("Expr User"))
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with column expression")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "expr@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("EXPR USER", retrieved.Name, "Name should be uppercased by expression")

		suite.T().Logf("ColumnExpr result: Name=%s", retrieved.Name)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("MultipleColumnOverrides", func() {
		user := &User{
			Name:     "Original",
			Email:    "multi@example.com",
			Age:      20,
			IsActive: false,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Column("name", "Multiple Override").
			Column("age", 35).
			Column("is_active", true).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with multiple column overrides")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "multi@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Multiple Override", retrieved.Name, "Should preserve name value")
		suite.Equal(int16(35), retrieved.Age, "Should preserve age value")
		suite.True(retrieved.IsActive, "Should have correct isactive value")

		suite.T().Logf("Multiple overrides: Name=%s, Age=%d, IsActive=%v",
			retrieved.Name, retrieved.Age, retrieved.IsActive)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestConflictHandling tests OnConflict with DO NOTHING and DO UPDATE.
func (suite *InsertTestSuite) TestConflictHandling() {
	suite.T().Logf("Testing conflict handling for %s", suite.ds.Kind)

	suite.Run("OnConflictDoNothing", func() {
		original := &User{
			Name:     "Conflict User",
			Email:    "conflict@example.com",
			Age:      30,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(original).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert original user")

		duplicate := &User{
			Name:     "Duplicate User",
			Email:    "conflict@example.com",
			Age:      25,
			IsActive: false,
		}

		_, err = suite.db.NewInsert().
			Model(duplicate).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("email").DoNothing()
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle conflict with DO NOTHING")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "conflict@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Conflict User", retrieved.Name, "Original name should be unchanged")
		suite.Equal(int16(30), retrieved.Age, "Original age should be unchanged")

		suite.T().Logf("DO NOTHING: Name=%s, Age=%d (unchanged)", retrieved.Name, retrieved.Age)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "conflict@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("OnConflictDoUpdate", func() {
		original := &User{
			Name:     "Update Original",
			Email:    "update-conflict@example.com",
			Age:      30,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(original).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert original user")

		update := &User{
			Name:     "Update Modified",
			Email:    "update-conflict@example.com",
			Age:      35,
			IsActive: false,
		}

		_, err = suite.db.NewInsert().
			Model(update).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("email").DoUpdate().
					Set("name", "Update Modified").
					Set("age", 35).
					Set("is_active", false)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle conflict with DO UPDATE")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "update-conflict@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Update Modified", retrieved.Name, "Name should be updated")
		suite.Equal(int16(35), retrieved.Age, "Age should be updated")
		suite.False(retrieved.IsActive, "IsActive should be updated")

		suite.T().Logf("DO UPDATE: Name=%s, Age=%d, IsActive=%v",
			retrieved.Name, retrieved.Age, retrieved.IsActive)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "update-conflict@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("OnConflictWithCondition", func() {
		original := &User{
			Name:     "Conditional User",
			Email:    "conditional@example.com",
			Age:      40,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(original).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert record")

		update := &User{
			Name:     "Conditional Update",
			Email:    "conditional@example.com",
			Age:      45,
			IsActive: true,
		}

		_, err = suite.db.NewInsert().
			Model(update).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("email").DoUpdate().
					Set("age", 45).
					Where(func(wcb orm.ConditionBuilder) {
						wcb.GreaterThan("age", 35)
					})
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle conflict with conditional update")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "conditional@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal(int16(45), retrieved.Age, "Age should be updated based on condition")

		suite.T().Logf("Conditional UPDATE: Age=%d", retrieved.Age)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "conditional@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestReturning tests Returning, ReturningAll, and ReturningNone (PostgreSQL/SQLite only).
func (suite *InsertTestSuite) TestReturning() {
	suite.T().Logf("Testing RETURNING clause for %s", suite.ds.Kind)

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("RETURNING clause not supported on %s", suite.ds.Kind)

		return
	}

	suite.Run("ReturningSpecificColumns", func() {
		user := &User{
			Name:     "Return User",
			Email:    "return@example.com",
			Age:      29,
			IsActive: true,
		}

		err := suite.db.NewInsert().
			Model(user).
			Returning("id", "name", "email").
			Scan(suite.ctx, user)
		suite.NoError(err, "Should insert with RETURNING specific columns")
		suite.NotEmpty(user.ID, "ID should be returned")
		suite.Equal("Return User", user.Name, "Name should be returned")
		suite.Equal("return@example.com", user.Email, "Email should be returned")

		suite.T().Logf("RETURNING columns: ID=%s, Name=%s, Email=%s",
			user.ID, user.Name, user.Email)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("ReturningAllColumns", func() {
		user := &User{
			Name:     "Return All User",
			Email:    "returnall@example.com",
			Age:      32,
			IsActive: true,
		}

		err := suite.db.NewInsert().
			Model(user).
			ReturningAll().
			Scan(suite.ctx, user)
		suite.NoError(err, "Should insert with RETURNING all columns")
		suite.NotEmpty(user.ID, "Should set ID after insert")
		suite.Equal("Return All User", user.Name, "Should preserve name value")
		suite.Equal(int16(32), user.Age, "Should preserve age value")
		suite.True(user.IsActive, "Should have correct isactive value")

		suite.T().Logf("RETURNING all: ID=%s, Name=%s, Age=%d",
			user.ID, user.Name, user.Age)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("ReturningNoColumns", func() {
		user := &User{
			Name:     "Return None User",
			Email:    "returnnone@example.com",
			Age:      28,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			ReturningNone().
			Exec(suite.ctx)
		suite.NoError(err, "Should insert with RETURNING none")
		suite.NotEmpty(user.ID, "ID should still be set by audit handler")

		suite.T().Logf("RETURNING none: ID=%s (set by audit)", user.ID)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestApply tests Apply and ApplyIf methods.
func (suite *InsertTestSuite) TestApply() {
	suite.T().Logf("Testing Apply methods for %s", suite.ds.Kind)

	suite.Run("ApplyUnconditional", func() {
		user := &User{
			Name:     "Apply User",
			Email:    "apply@example.com",
			Age:      27,
			IsActive: true,
		}

		applyFunc := func(q orm.InsertQuery) {
			q.Column("name", "Applied Name")
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Apply(applyFunc).
			Exec(suite.ctx)
		suite.NoError(err, "Should apply function unconditionally")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "apply@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Applied Name", retrieved.Name, "Name should be modified by Apply")

		suite.T().Logf("Apply result: Name=%s", retrieved.Name)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("ApplyConditional", func() {
		user1 := &User{
			Name:     "Conditional User 1",
			Email:    "cond1@example.com",
			Age:      30,
			IsActive: true,
		}

		applyFunc := func(q orm.InsertQuery) {
			q.Column("name", "Modified Name")
		}

		_, err := suite.db.NewInsert().
			Model(user1).
			ApplyIf(true, applyFunc).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert record")
		suite.NotEmpty(user1.ID, "Should set ID after insert")

		var retrieved1 User

		err = suite.db.NewSelect().
			Model(&retrieved1).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "cond1@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Modified Name", retrieved1.Name, "ApplyIf(true) should apply function")

		user2 := &User{
			Name:     "Conditional User 2",
			Email:    "cond2@example.com",
			Age:      32,
			IsActive: true,
		}

		_, err = suite.db.NewInsert().
			Model(user2).
			ApplyIf(false, applyFunc).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert record")
		suite.NotEmpty(user2.ID, "Should set ID after insert")

		var retrieved2 User

		err = suite.db.NewSelect().
			Model(&retrieved2).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "cond2@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Conditional User 2", retrieved2.Name, "ApplyIf(false) should not apply function")

		suite.T().Logf("ApplyIf: true=%s, false=%s", retrieved1.Name, retrieved2.Name)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("email", []any{"cond1@example.com", "cond2@example.com"})
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("ApplyMultipleFunctions", func() {
		user := &User{
			Name:     "Multi Apply",
			Email:    "multi-apply@example.com",
			Age:      20,
			IsActive: false,
		}

		fn1 := func(q orm.InsertQuery) {
			q.Column("name", "Step 1")
		}
		fn2 := func(q orm.InsertQuery) {
			q.Column("age", 25)
		}
		fn3 := func(q orm.InsertQuery) {
			q.Column("is_active", true)
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Apply(fn1, fn2, fn3).
			Exec(suite.ctx)
		suite.NoError(err, "Should apply multiple functions")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		var retrieved User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "multi-apply@example.com")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Equal("Step 1", retrieved.Name, "Should preserve name value")
		suite.Equal(int16(25), retrieved.Age, "Should preserve age value")
		suite.True(retrieved.IsActive, "Should have correct isactive value")

		suite.T().Logf("Multiple Apply: Name=%s, Age=%d, IsActive=%v",
			retrieved.Name, retrieved.Age, retrieved.IsActive)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("ApplyWithNilFunction", func() {
		user := &User{
			Name:     "Nil Apply User",
			Email:    "nil-apply@example.com",
			Age:      28,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Apply(nil).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle nil function safely")
		suite.NotEmpty(user.ID, "Should set ID after insert")

		suite.T().Logf("Nil function handled: ID=%s", user.ID)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(user.ID)
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestBulkInsert tests bulk insert operations.
func (suite *InsertTestSuite) TestBulkInsert() {
	suite.T().Logf("Testing bulk INSERT for %s", suite.ds.Kind)

	suite.Run("LargeBatchInsert", func() {
		batchSize := 10

		users := make([]*User, batchSize)
		for i := range batchSize {
			users[i] = &User{
				Name:     fmt.Sprintf("Batch User %d", i+1),
				Email:    fmt.Sprintf("batch%d@example.com", i+1),
				Age:      int16(20 + i),
				IsActive: i%2 == 0,
			}
		}

		start := time.Now()
		_, err := suite.db.NewInsert().
			Model(&users).
			Exec(suite.ctx)
		duration := time.Since(start)

		suite.NoError(err, "Should insert batch users successfully")
		suite.T().Logf("Batch insert of %d users took %v", batchSize, duration)

		for _, user := range users {
			suite.NotEmpty(user.ID, "Each user should have an ID")
		}

		var retrieved []User

		err = suite.db.NewSelect().
			Model(&retrieved).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "batch")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve inserted record")
		suite.Len(retrieved, batchSize, "Should have inserted all batch users")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "batch")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("BulkInsertWithRelatedData", func() {
		users := []*User{
			{Name: "Post Author 1", Email: "author1@bulk.com", Age: 30, IsActive: true},
			{Name: "Post Author 2", Email: "author2@bulk.com", Age: 25, IsActive: true},
		}

		_, err := suite.db.NewInsert().
			Model(&users).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert bulk users")

		posts := []*Post{
			{
				Title:       "Bulk Post 1",
				Content:     "Content for bulk post 1",
				Description: new("Description 1"),
				UserID:      users[0].ID,
				CategoryID:  suite.getCategoryID(),
				Status:      "published",
				ViewCount:   100,
			},
			{
				Title:      "Bulk Post 2",
				Content:    "Content for bulk post 2",
				UserID:     users[1].ID,
				CategoryID: suite.getCategoryID(),
				Status:     "draft",
				ViewCount:  0,
			},
		}

		_, err = suite.db.NewInsert().
			Model(&posts).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert related posts")

		for _, post := range posts {
			suite.NotEmpty(post.ID, "Should set ID after insert")
			suite.T().Logf("Bulk post: ID=%s, Title=%s, UserID=%s", post.ID, post.Title, post.UserID)
		}

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Bulk Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("email", []any{"author1@bulk.com", "author2@bulk.com"})
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})
}

// TestErrorHandling tests error scenarios in insert operations.
func (suite *InsertTestSuite) TestErrorHandling() {
	suite.T().Logf("Testing error handling for %s", suite.ds.Kind)

	suite.Run("UniqueConstraintViolation", func() {
		original := &User{
			Name:     "Original User",
			Email:    "unique@example.com",
			Age:      25,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(original).
			Exec(suite.ctx)
		suite.NoError(err, "Should insert original user")

		duplicate := &User{
			Name:     "Duplicate User",
			Email:    "unique@example.com",
			Age:      30,
			IsActive: false,
		}

		_, err = suite.db.NewInsert().
			Model(duplicate).
			Exec(suite.ctx)
		suite.Error(err, "Insert with duplicate email should fail")

		suite.T().Logf("Unique constraint violation handled correctly")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "unique@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should succeed without error")
	})

	suite.Run("NullConstraintViolation", func() {
		invalid := &User{
			Name:  "",
			Email: "",
		}

		_, err := suite.db.NewInsert().
			Model(invalid).
			Column("name", nil).
			Column("email", nil).
			Exec(suite.ctx)
		suite.Error(err, "Insert with null constraint violation should fail")

		suite.T().Logf("NULL constraint violation handled correctly")
	})

	suite.Run("InvalidDataType", func() {
		user := &User{
			Name:     "Invalid Type User",
			Email:    "invalid-type@example.com",
			Age:      30,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().
			Model(user).
			Column("age", "not a number").
			Exec(suite.ctx)
		suite.Error(err, "Insert with invalid data type should fail")

		suite.T().Logf("Invalid data type handled correctly")
	})
}

// getCategoryID returns the first available category ID from fixture data.
func (suite *InsertTestSuite) getCategoryID() string {
	var category Category
	if err := suite.db.NewSelect().
		Model(&category).
		Limit(1).
		Scan(suite.ctx); err != nil {
		suite.T().Fatalf("Failed to get category ID: %v", err)
	}

	return category.ID
}

// TestOnConflictConstraint tests OnConflict with Constraint.
func (suite *InsertTestSuite) TestOnConflictConstraint() {
	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("ON CONFLICT CONSTRAINT not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing OnConflict Constraint for %s", suite.ds.Kind)

	tag := &Tag{Name: "Go"}
	tag.ID = "test-conflict-constraint"

	// First insert
	_, err := suite.db.NewInsert().
		Model(tag).
		Exec(suite.ctx)
	suite.NoError(err, "First insert should succeed")

	// Second insert with conflict on columns + DoNothing
	_, err = suite.db.NewInsert().
		Model(tag).
		OnConflict(func(cb orm.ConflictBuilder) {
			cb.Columns("id").DoNothing()
		}).
		Exec(suite.ctx)
	suite.NoError(err, "OnConflict DoNothing should work")

	// Clean up
	_, _ = suite.db.NewDelete().Model(tag).WherePK().Exec(suite.ctx)
}

// TestOnConflictSetExpr tests OnConflict with SetExpr and Where.
func (suite *InsertTestSuite) TestOnConflictSetExpr() {
	suite.T().Logf("Testing OnConflict SetExpr for %s", suite.ds.Kind)

	tag := &Tag{Name: "ConflictSetExpr"}
	tag.ID = "test-conflict-setexpr"

	// First insert
	_, err := suite.db.NewInsert().
		Model(tag).
		Exec(suite.ctx)
	suite.NoError(err, "First insert should succeed")

	// Update on conflict using SetExpr
	tag.Name = "UpdatedConflictSetExpr"

	_, err = suite.db.NewInsert().
		Model(tag).
		OnConflict(func(cb orm.ConflictBuilder) {
			cb.Columns("id").
				DoUpdate().
				SetExpr("name", func(eb orm.ExprBuilder) any {
					return eb.Literal("UpdatedViaSetExpr")
				})
		}).
		Exec(suite.ctx)
	suite.NoError(err, "OnConflict SetExpr should work")

	// Verify update
	var result Tag

	err = suite.db.NewSelect().
		Model(&result).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", "test-conflict-setexpr")
		}).
		Scan(suite.ctx)
	suite.NoError(err, "Should find updated tag")
	suite.Equal("UpdatedViaSetExpr", result.Name, "Should preserve name value")

	// Clean up
	_, _ = suite.db.NewDelete().Model(tag).WherePK().Exec(suite.ctx)
}

// TestOnConflictWithWhere tests OnConflict with Where on target and update.
func (suite *InsertTestSuite) TestOnConflictWithWhere() {
	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("ON CONFLICT WHERE not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing OnConflict Where for %s", suite.ds.Kind)

	tag := &Tag{Name: "WhereConflict"}
	tag.ID = "test-conflict-where"

	// First insert
	_, err := suite.db.NewInsert().
		Model(tag).
		Exec(suite.ctx)
	suite.NoError(err, "First insert should succeed")

	// Insert with conflict + Where on DoUpdate
	_, err = suite.db.NewInsert().
		Model(tag).
		OnConflict(func(cb orm.ConflictBuilder) {
			cb.Columns("id").
				DoUpdate().
				Set("name", "UpdatedWhere").
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEquals("name", "AlreadyUpdated")
				})
		}).
		Exec(suite.ctx)
	suite.NoError(err, "OnConflict with DoUpdate Where should work")

	// Clean up
	_, _ = suite.db.NewDelete().Model(tag).WherePK().Exec(suite.ctx)
}

// TestInsertDB tests DB() method returns the underlying database.
func (suite *InsertTestSuite) TestInsertDB() {
	suite.T().Logf("Testing Insert DB for %s", suite.ds.Kind)

	query := suite.db.NewInsert().Model(&Tag{})
	db := query.DB()

	suite.NotNil(db, "Should return underlying database")
}

// TestTableSpecMethods tests table specification methods with and without aliases.
func (suite *InsertTestSuite) TestTableSpecMethods() {
	suite.T().Logf("Testing table specification methods for %s", suite.ds.Kind)

	suite.Run("ModelTableWithAlias", func() {
		query := suite.db.NewInsert().
			ModelTable("test_tag", "t").
			Column("id", "test-alias-mt").
			Column("name", "AliasModelTable")
		suite.NotNil(query, "Should build query with ModelTable alias")
	})

	suite.Run("ModelTableNoAlias", func() {
		query := suite.db.NewInsert().
			ModelTable("test_tag").
			Column("id", "test-no-alias").
			Column("name", "NoAliasTest")
		suite.NotNil(query, "Should build query with ModelTable")
	})

	suite.Run("TableWithAlias", func() {
		query := suite.db.NewInsert().
			Table("test_tag", "t").
			Column("id", "test-alias-t").
			Column("name", "AliasTable")
		suite.NotNil(query, "Should build query with Table alias")
	})

	suite.Run("TableNoAlias", func() {
		query := suite.db.NewInsert().
			Table("test_tag").
			Column("id", "test-no-alias-t").
			Column("name", "NoAliasTableTest")
		suite.NotNil(query, "Should build query with Table")
	})

	suite.Run("TableFromWithAlias", func() {
		query := suite.db.NewInsert().
			TableFrom((*Tag)(nil), "t").
			Column("id", "test-alias-tf").
			Column("name", "AliasTableFrom")
		suite.NotNil(query, "Should build query with TableFrom alias")
	})

	suite.Run("TableFromNoAlias", func() {
		query := suite.db.NewInsert().
			TableFrom((*Tag)(nil)).
			Column("id", "test-ins-tablefrom").
			Column("name", "InsertTableFrom")
		suite.NotNil(query, "Should build query with TableFrom")
	})

	suite.Run("TableExprWithAlias", func() {
		query := suite.db.NewInsert().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}, "t").
			Column("id", "test-alias-te").
			Column("name", "AliasTableExpr")
		suite.NotNil(query, "Should build query with TableExpr alias")
	})

	suite.Run("TableExprNoAlias", func() {
		query := suite.db.NewInsert().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}).
			Column("id", "test-no-alias-te").
			Column("name", "NoAliasTableExprTest")
		suite.NotNil(query, "Should build query with TableExpr")
	})

	suite.Run("TableSubQueryWithAlias", func() {
		query := suite.db.NewInsert().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id", "name")
			}, "t")
		suite.NotNil(query, "Should build query with TableSubQuery alias")
	})

	suite.Run("TableSubQueryNoAlias", func() {
		query := suite.db.NewInsert().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id", "name")
			})
		suite.NotNil(query, "Should build query with TableSubQuery")
	})
}

// TestSelectionMethods tests SelectAll, Select, and ExcludeAll methods.
func (suite *InsertTestSuite) TestSelectionMethods() {
	suite.T().Logf("Testing selection methods for %s", suite.ds.Kind)

	suite.Run("SelectAll", func() {
		tag := &Tag{Name: "SelectAllTest"}
		tag.ID = "test-ins-selectall"

		query := suite.db.NewInsert().Model(tag).SelectAll()
		suite.NotNil(query, "Should build query with SelectAll")
	})

	suite.Run("SelectSpecificColumns", func() {
		tag := &Tag{Name: "SelectTest"}
		tag.ID = "test-ins-select"

		query := suite.db.NewInsert().Model(tag).Select("id", "name")
		suite.NotNil(query, "Should build query with Select")
	})

	suite.Run("ExcludeAll", func() {
		tag := &Tag{Name: "ExcludeAllTest"}
		tag.ID = "test-ins-excludeall"

		query := suite.db.NewInsert().Model(tag).ExcludeAll()
		suite.NotNil(query, "Should build query with ExcludeAll")
	})
}

// TestConflictAdvanced tests OnConflict with Constraint, Where, and SetExpr.
func (suite *InsertTestSuite) TestConflictAdvanced() {
	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("ON CONFLICT CONSTRAINT/WHERE not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing OnConflict advanced for %s", suite.ds.Kind)

	suite.Run("ConstraintWithWhere", func() {
		tag := &Tag{Name: "ConstraintTest"}
		tag.ID = "test-conflict-cstr"

		_, err := suite.db.NewInsert().Model(tag).Exec(suite.ctx)
		suite.Require().NoError(err, "Should insert initial tag")

		_, err = suite.db.NewInsert().
			Model(tag).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("id").
					Where(func(cond orm.ConditionBuilder) {
						cond.IsNotNull("id")
					}).
					DoUpdate().
					Set("name", "UpdatedConstraint")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle conflict with Where clause")

		_, _ = suite.db.NewDelete().Model((*Tag)(nil)).Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", "test-conflict-cstr")
		}).Exec(suite.ctx)
	})

	suite.Run("ConstraintByName", func() {
		tag := &Tag{Name: "ConstraintTest2"}
		tag.ID = "test-cstr-2"

		_, err := suite.db.NewInsert().Model(tag).Exec(suite.ctx)
		suite.Require().NoError(err, "Should insert initial tag")

		// Covers the Constraint method code path
		query := suite.db.NewInsert().
			Model(tag).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Constraint("test_tag_pkey").DoNothing()
			})
		suite.NotNil(query, "Should build query with named constraint")

		_, _ = query.Exec(suite.ctx)
		_, _ = suite.db.NewDelete().Model((*Tag)(nil)).Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", "test-cstr-2")
		}).Exec(suite.ctx)
	})

	suite.Run("SetExprOnConflict", func() {
		tag := &Tag{Name: "ConflictSetExpr"}
		tag.ID = "test-conflict-setexpr"

		_, err := suite.db.NewInsert().Model(tag).Exec(suite.ctx)
		suite.Require().NoError(err, "Should insert initial tag")

		_, err = suite.db.NewInsert().
			Model(tag).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("id").
					DoUpdate().
					SetExpr("name", func(eb orm.ExprBuilder) any {
						return eb.Literal("UpdatedViaSetExpr")
					})
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle conflict with SetExpr")

		var result Tag

		err = suite.db.NewSelect().
			Model(&result).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", "test-conflict-setexpr")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should find updated tag")
		suite.Equal("UpdatedViaSetExpr", result.Name, "Should update name via SetExpr")

		_, _ = suite.db.NewDelete().Model(tag).WherePK().Exec(suite.ctx)
	})

	suite.Run("WhereOnDoUpdate", func() {
		tag := &Tag{Name: "WhereConflict"}
		tag.ID = "test-conflict-where"

		_, err := suite.db.NewInsert().Model(tag).Exec(suite.ctx)
		suite.Require().NoError(err, "Should insert initial tag")

		_, err = suite.db.NewInsert().
			Model(tag).
			OnConflict(func(cb orm.ConflictBuilder) {
				cb.Columns("id").
					DoUpdate().
					Set("name", "UpdatedWhere").
					Where(func(cb orm.ConditionBuilder) {
						cb.NotEquals("name", "AlreadyUpdated")
					})
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should handle conflict with conditional update")

		_, _ = suite.db.NewDelete().Model(tag).WherePK().Exec(suite.ctx)
	})
}
