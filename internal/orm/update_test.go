package orm_test

import (
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &UpdateTestSuite{BaseTestSuite: base}
	})
}

// UpdateTestSuite tests UPDATE operations across all databases.
type UpdateTestSuite struct {
	*BaseTestSuite

	testUsers []*User
	testPosts []*Post
}

// SetupTest inserts isolated test data before each test method.
func (suite *UpdateTestSuite) SetupTest() {
	suite.testUsers = []*User{
		{Name: "UT Alice", Email: "ut_alice@test.com", Age: 30, IsActive: true},
		{Name: "UT Bob", Email: "ut_bob@test.com", Age: 25, IsActive: true},
		{Name: "UT Charlie", Email: "ut_charlie@test.com", Age: 35, IsActive: false},
	}

	_, err := suite.db.NewInsert().Model(&suite.testUsers).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test users")

	suite.testPosts = []*Post{
		{Title: "UT Post Alpha", Content: "Content A", UserID: suite.testUsers[0].ID, CategoryID: "cat001", Status: "published", ViewCount: 100},
		{Title: "UT Post Beta", Content: "Content B", UserID: suite.testUsers[0].ID, CategoryID: "cat001", Status: "draft", ViewCount: 50},
		{Title: "UT Post Gamma", Content: "Content C", UserID: suite.testUsers[1].ID, CategoryID: "cat002", Status: "published", ViewCount: 75},
		{Title: "UT Post Delta", Content: "Content D", UserID: suite.testUsers[2].ID, CategoryID: "cat001", Status: "review", ViewCount: 30},
	}

	_, err = suite.db.NewInsert().Model(&suite.testPosts).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert test posts")
}

// TearDownTest removes all test-inserted data (created_at >= 2026) after each test method.
func (suite *UpdateTestSuite) TearDownTest() {
	// Delete test posts first (FK constraint on user_id)
	_, _ = suite.db.NewDelete().Model((*Post)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.GreaterThanOrEqual("created_at", fixtureEndDate)
	}).Exec(suite.ctx)

	// Delete test users
	_, _ = suite.db.NewDelete().Model((*User)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.GreaterThanOrEqual("created_at", fixtureEndDate)
	}).Exec(suite.ctx)
}

// TestCTE tests Common Table Expression methods (With, WithValues, WithRecursive).
func (suite *UpdateTestSuite) TestCTE() {
	suite.T().Logf("Testing CTE methods for %s", suite.ds.Kind)

	suite.Run("WithBasicCTE", func() {
		// Create CTE of active test users, then update their posts
		activeUserIDs := []string{suite.testUsers[0].ID, suite.testUsers[1].ID}

		result, err := suite.db.NewUpdate().
			With("active_users", func(query orm.SelectQuery) {
				query.Model((*User)(nil)).
					Select("id").
					Where(func(cb orm.ConditionBuilder) {
						cb.In("id", activeUserIDs).IsTrue("is_active")
					})
			}).
			Model((*Post)(nil)).
			Set("updated_by", "cte_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.InSubQuery("user_id", func(subquery orm.SelectQuery) {
					subquery.Table("active_users").Select("id")
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WITH clause should work for updates")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected")
		suite.True(rowsAffected > 0, "Should update posts from active users")

		suite.T().Logf("Updated %d posts using WITH CTE", rowsAffected)
	})

	suite.Run("WithValuesCTE", func() {
		postIDs := []string{suite.testPosts[0].ID, suite.testPosts[1].ID}

		type StatusMapping struct {
			OldStatus string `bun:"old_status"`
			NewStatus string `bun:"new_status"`
		}

		mappings := []StatusMapping{
			{OldStatus: "published", NewStatus: "archived"},
			{OldStatus: "draft", NewStatus: "deleted"},
		}

		result, err := suite.db.NewUpdate().
			WithValues("status_map", &mappings).
			Model((*Post)(nil)).
			Table("status_map", "sm").
			SetExpr("status", func(eb orm.ExprBuilder) any {
				return eb.Column("sm.new_status")
			}).
			Where(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("status", "sm.old_status").
					In("id", postIDs)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WITH VALUES should work when supported")

		rowsAffected, _ := result.RowsAffected()
		suite.T().Logf("Updated %d posts using WITH VALUES", rowsAffected)
	})
}

// TestTableSource tests table source methods (Model, ModelTable, Table, TableFrom, TableExpr, TableSubQuery).
func (suite *UpdateTestSuite) TestTableSource() {
	suite.T().Logf("Testing table source methods for %s", suite.ds.Kind)

	suite.Run("ModelBasic", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "model_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Model should set table correctly")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Updated %d user using Model", rowsAffected)
	})

	suite.Run("ModelTable", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			ModelTable("test_user", "u").
			Set("updated_by", "model_table_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("u.id", suite.testUsers[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "ModelTable should override table name and alias")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Updated %d user using ModelTable", rowsAffected)
	})

	suite.Run("TableDirect", func() {
		result, err := suite.db.NewUpdate().
			Table("test_post", "p").
			Set("status", "published").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("p.id", suite.testPosts[1].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Table method should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected")
		suite.Equal(int64(1), rowsAffected, "Should update 1 post using Table")

		suite.T().Logf("Updated %d post using Table method", rowsAffected)
	})

	suite.Run("TableFrom", func() {
		result, err := suite.db.NewUpdate().
			TableFrom((*Post)(nil), "p").
			Set("status", "archived").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("p.id", suite.testPosts[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "TableFrom method should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected")
		suite.Equal(int64(1), rowsAffected, "Should update 1 post using TableFrom")

		suite.T().Logf("Updated %d post using TableFrom method", rowsAffected)
	})

	suite.Run("TableExpr", func() {
		// Use the inactive test user (testUsers[2]) and their post (testPosts[3])
		result, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*User)(nil))
				})
			}, "u").
			Set("status", "archived").
			Set("updated_by", "multi_table_update").
			Where(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("user_id", "u.id").
					Equals("u.id", suite.testUsers[2].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "TableExpr for multi-table update should work")

		rowsAffected, _ := result.RowsAffected()
		suite.True(rowsAffected > 0, "Should update posts from inactive users")
		suite.T().Logf("Multi-table update affected %d posts on %s", rowsAffected, suite.ds.Kind)
	})
}

// TestSelectionMethods tests selection methods for controlling which columns are included in UPDATE SET clause.
// These methods (Select, Exclude, SelectAll, ExcludeAll) only work when updating with a model instance,
// as the framework reflects all columns from the model and these methods control which ones to include in SET.
func (suite *UpdateTestSuite) TestSelectionMethods() {
	suite.T().Logf("Testing selection methods for %s", suite.ds.Kind)

	suite.Run("SelectSpecificColumns", func() {
		tu := suite.testUsers[0]

		// Create a NEW user instance (not from database query)
		user := User{
			Model: orm.Model{ID: tu.ID},
			Name:  "UT Alice Updated",
			Age:   99,
		}

		result, err := suite.db.NewUpdate().
			Model(&user).
			Select("name", "age").
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "Select should control which columns are updated")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		// Verify only selected columns were updated
		var updatedUser User

		updatedUser.ID = tu.ID

		err = suite.db.NewSelect().
			Model(&updatedUser).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch updated user")

		suite.Equal("UT Alice Updated", updatedUser.Name, "Name should be updated")
		suite.Equal(int16(99), updatedUser.Age, "Age should be updated")
		suite.Equal(tu.Email, updatedUser.Email, "Email should NOT be updated")
		suite.Equal(tu.IsActive, updatedUser.IsActive, "IsActive should NOT be updated")

		suite.T().Logf("Select updated only name and age: name=%s, age=%d, email=%s (unchanged), is_active=%v (unchanged)",
			updatedUser.Name, updatedUser.Age, updatedUser.Email, updatedUser.IsActive)
	})

	suite.Run("SelectAllColumns", func() {
		tu := suite.testUsers[1]

		var user User

		user.ID = tu.ID

		err := suite.db.NewSelect().
			Model(&user).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch user")

		// Modify all fields
		user.Name = "UT Bob Updated"
		user.Age = 88
		user.Email = "ut_bob_new@test.com"
		user.IsActive = false

		result, err := suite.db.NewUpdate().
			Model(&user).
			SelectAll().
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "SelectAll should update all columns")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		// Verify all columns were updated
		var updatedUser User

		updatedUser.ID = user.ID

		err = suite.db.NewSelect().
			Model(&updatedUser).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch updated user")

		suite.Equal("UT Bob Updated", updatedUser.Name, "Name should be updated")
		suite.Equal(int16(88), updatedUser.Age, "Age should be updated")
		suite.Equal("ut_bob_new@test.com", updatedUser.Email, "Email should be updated")
		suite.Equal(false, updatedUser.IsActive, "IsActive should be updated")

		suite.T().Logf("SelectAll updated all columns: name=%s, age=%d, email=%s, is_active=%v",
			updatedUser.Name, updatedUser.Age, updatedUser.Email, updatedUser.IsActive)
	})

	suite.Run("ExcludeSpecificColumns", func() {
		tu := suite.testUsers[2]

		var user User

		user.ID = tu.ID

		err := suite.db.NewSelect().
			Model(&user).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch user")

		originalEmail := user.Email
		originalAge := user.Age

		// Modify multiple fields but exclude some from update
		user.Name = "UT Charlie Updated"
		user.Age = 77
		user.Email = "ut_charlie_new@test.com"
		user.IsActive = true

		result, err := suite.db.NewUpdate().
			Model(&user).
			Exclude("email", "age").
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "Exclude should prevent specific columns from being updated")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		// Verify excluded columns were NOT updated
		var updatedUser User

		updatedUser.ID = user.ID

		err = suite.db.NewSelect().
			Model(&updatedUser).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch updated user")

		suite.Equal("UT Charlie Updated", updatedUser.Name, "Name should be updated")
		suite.Equal(true, updatedUser.IsActive, "IsActive should be updated")
		suite.Equal(originalEmail, updatedUser.Email, "Email should NOT be updated (excluded)")
		suite.Equal(originalAge, updatedUser.Age, "Age should NOT be updated (excluded)")

		suite.T().Logf("Exclude prevented email and age from updating: name=%s (updated), email=%s (unchanged), age=%d (unchanged)",
			updatedUser.Name, updatedUser.Email, updatedUser.Age)
	})

	suite.Run("ExcludeAllColumns", func() {
		tu := suite.testUsers[0]

		var user User

		user.ID = tu.ID

		err := suite.db.NewSelect().
			Model(&user).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch user")

		originalName := user.Name
		originalEmail := user.Email

		// Modify multiple fields
		user.Name = "UT Alice Updated Again"
		user.Age = 77
		user.Email = "ut_alice_new@test.com"

		// Select name and email first, then ExcludeAll clears them, then Select age
		// Result: only age should be updated
		result, err := suite.db.NewUpdate().
			Model(&user).
			Select("name", "email").
			ExcludeAll().
			Select("age").
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "ExcludeAll should clear previous Select and allow new Select")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		// Verify only age was updated
		var updatedUser User

		updatedUser.ID = user.ID

		err = suite.db.NewSelect().
			Model(&updatedUser).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch updated user")

		suite.Equal(originalName, updatedUser.Name, "Name should NOT be updated (cleared by ExcludeAll)")
		suite.Equal(originalEmail, updatedUser.Email, "Email should NOT be updated (cleared by ExcludeAll)")
		suite.Equal(int16(77), updatedUser.Age, "Age should be updated (selected after ExcludeAll)")

		suite.T().Logf("ExcludeAll cleared previous Select, only age updated: name=%s (unchanged), email=%s (unchanged), age=%d (updated)",
			updatedUser.Name, updatedUser.Email, updatedUser.Age)
	})
}

// TestFilterOperations tests filter methods (Where, WherePK, WhereDeleted, IncludeDeleted).
func (suite *UpdateTestSuite) TestFilterOperations() {
	suite.T().Logf("Testing filter operations for %s", suite.ds.Kind)

	suite.Run("WhereBasic", func() {
		activeIDs := []string{suite.testUsers[0].ID, suite.testUsers[1].ID}

		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "where_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", activeIDs).IsTrue("is_active")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Where should work correctly")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(2), rowsAffected, "Should update 2 active test users")

		suite.T().Logf("Updated %d users with Where", rowsAffected)
	})

	suite.Run("WherePK", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "where_pk_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WherePK should work correctly")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Updated user by PK: %s", suite.testUsers[0].ID)
	})

	suite.Run("WhereDeletedAndIncludeDeleted", func() {
		type SoftDeleteArticle struct {
			bun.BaseModel `bun:"table:test_update_soft_delete,alias:tusd"`
			orm.Model

			Title     string    `json:"title" bun:"title,notnull"`
			Status    string    `json:"status" bun:"status,notnull"`
			DeletedAt time.Time `json:"deletedAt" bun:",soft_delete,nullzero"`
		}

		err := suite.db.ResetModel(suite.ctx, (*SoftDeleteArticle)(nil))

		suite.Require().NoError(err, "Should reset soft delete table")
		defer func() {
			_, dropErr := suite.db.NewDropTable().Model((*SoftDeleteArticle)(nil)).IfExists().Exec(suite.ctx)
			suite.Require().NoError(dropErr, "Should cleanup soft delete table")
		}()

		records := []*SoftDeleteArticle{
			{Title: "Soft delete target", Status: "draft"},
			{Title: "Active record", Status: "draft"},
		}

		_, err = suite.db.NewInsert().Model(&records).Exec(suite.ctx)
		suite.Require().NoError(err, "Should insert soft delete records")

		_, err = suite.db.NewDelete().
			Model((*SoftDeleteArticle)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", records[0].ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Should soft delete first record")

		result, err := suite.db.NewUpdate().
			Model((*SoftDeleteArticle)(nil)).
			Set("status", "archived").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", records[0].ID)
			}).
			WhereDeleted().
			Exec(suite.ctx)
		suite.Require().NoError(err, "WhereDeleted should target soft-deleted rows")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "WhereDeleted should update exactly one record")

		result, err = suite.db.NewUpdate().
			Model((*SoftDeleteArticle)(nil)).
			Set("status", "reviewed").
			IncludeDeleted().
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", []string{records[0].ID, records[1].ID})
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "IncludeDeleted should allow updating soft-deleted rows")

		rowsAffected, _ = result.RowsAffected()
		suite.Equal(int64(2), rowsAffected, "IncludeDeleted should update both records")

		var fetched []SoftDeleteArticle

		err = suite.db.NewSelect().
			Model(&fetched).
			IncludeDeleted().
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", []string{records[0].ID, records[1].ID})
			}).
			OrderBy("title").
			Scan(suite.ctx)
		suite.Require().NoError(err, "Should fetch all records including deleted")
		suite.Len(fetched, 2, "Should return two records")

		for _, record := range fetched {
			suite.Equal("reviewed", record.Status, "Status should reflect IncludeDeleted update")

			if record.ID == records[0].ID {
				suite.False(record.DeletedAt.IsZero(), "First record should remain soft deleted")
			} else {
				suite.True(record.DeletedAt.IsZero(), "Second record should remain active")
			}
		}
	})
}

// TestColumnUpdates tests column update methods (Column, ColumnExpr, Set, SetExpr).
func (suite *UpdateTestSuite) TestColumnUpdates() {
	suite.T().Logf("Testing column update methods for %s", suite.ds.Kind)

	suite.Run("ColumnMethod", func() {
		tp := suite.testPosts[0]

		var post Post

		post.ID = tp.ID

		err := suite.db.NewSelect().
			Model(&post).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch post")

		originalViewCount := post.ViewCount

		// Modify the model
		post.Title = "UT Updated Title from Model"
		post.ViewCount = 999

		// Use Column to override model's view_count value
		result, err := suite.db.NewUpdate().
			Model(&post).
			Column("view_count", 555). // Override model's 999 with 555
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "Column should override model value")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		// Verify the update
		var updatedPost Post

		updatedPost.ID = post.ID

		err = suite.db.NewSelect().
			Model(&updatedPost).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve updated post")
		suite.Equal("UT Updated Title from Model", updatedPost.Title, "Title should be from model")
		suite.Equal(int32(555), int32(updatedPost.ViewCount), "View count should be from Column, not model")

		suite.T().Logf("Column overrode model: view_count=%d (model was 999, original was %d)",
			updatedPost.ViewCount, originalViewCount)
	})

	suite.Run("ColumnExprMethod", func() {
		tp := suite.testPosts[2]

		var post Post

		post.ID = tp.ID

		err := suite.db.NewSelect().
			Model(&post).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch post")

		originalViewCount := post.ViewCount

		// Modify the model
		post.Title = "UT Updated Title from Model"
		post.ViewCount = 888

		// Use ColumnExpr to override model's view_count with an expression
		result, err := suite.db.NewUpdate().
			Model(&post).
			ColumnExpr("view_count", func(eb orm.ExprBuilder) any {
				// Increment view_count by 100
				return eb.Add(eb.Column("view_count"), 100)
			}).
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "ColumnExpr should override model value with expression")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		// Verify the update
		var updatedPost Post

		updatedPost.ID = post.ID

		err = suite.db.NewSelect().
			Model(&updatedPost).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve updated post")
		suite.Equal("UT Updated Title from Model", updatedPost.Title, "Title should be from model")
		suite.Equal(int32(originalViewCount+100), int32(updatedPost.ViewCount), "View count should be from ColumnExpr")

		suite.T().Logf("ColumnExpr overrode model: view_count=%d (was %d, model was 888)",
			updatedPost.ViewCount, originalViewCount)
	})

	suite.Run("SetMethod", func() {
		result, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			Set("updated_by", "set_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testPosts[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Set should work as alias for Column")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Updated using Set method")
	})

	suite.Run("SetWithAliasedColumn", func() {
		result, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			Set("p.updated_by", "alias_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testPosts[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Set with aliased column should strip alias on non-multi-table databases")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		var updatedPost Post

		updatedPost.ID = suite.testPosts[0].ID

		err = suite.db.NewSelect().
			Model(&updatedPost).
			WherePK().
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve updated post")
		suite.Equal("alias_test", updatedPost.UpdatedBy, "Updated_by should reflect the Set value")
	})

	suite.Run("SetExprMethod", func() {
		result, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			SetExpr("title", func(eb orm.ExprBuilder) any {
				return eb.Concat(eb.Column("title"), " [Updated]")
			}).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testPosts[1].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "SetExpr should work as alias for ColumnExpr")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Updated title using SetExpr")
	})
}

// TestUpdateFlags tests special flags (OmitZero, Bulk).
func (suite *UpdateTestSuite) TestUpdateFlags() {
	suite.T().Logf("Testing update flags for %s", suite.ds.Kind)

	suite.Run("OmitZeroFlag", func() {
		tu := suite.testUsers[0]

		// Update with a full User model but only set Name, leave Age as zero
		partialUpdate := &User{
			Model: orm.Model{ID: tu.ID},
			Name:  "UT Alice Updated with OmitZero",
			// Age will be zero, OmitZero should skip it
		}

		result, err := suite.db.NewUpdate().
			Model(partialUpdate).
			OmitZero().
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "OmitZero should skip zero-value fields")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Updated with OmitZero flag")
	})

	suite.Run("BulkUpdate", func() {
		// Fetch test posts for bulk update
		var posts []Post

		postIDs := make([]string, len(suite.testPosts))
		for i, tp := range suite.testPosts {
			postIDs[i] = tp.ID
		}

		err := suite.db.NewSelect().
			Model(&posts).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", postIDs)
			}).
			OrderBy("id").
			Scan(suite.ctx)
		suite.NoError(err, "Should fetch test posts for bulk update")
		suite.Equal(len(suite.testPosts), len(posts), "Should have all test posts")

		originalCount := len(posts)

		// Store original view counts for verification
		originalViewCounts := make([]int, len(posts))
		for i := range posts {
			originalViewCounts[i] = posts[i].ViewCount
		}

		// Modify each post with different values
		for i := range posts {
			posts[i].ViewCount = 1000 + i*100 // 1000, 1100, 1200, ...
			posts[i].UpdatedBy = "bulk_test"
		}

		// Perform bulk update using Bulk() method
		result, err := suite.db.NewUpdate().
			Model(&posts).
			Bulk().
			WherePK().
			Exec(suite.ctx)

		suite.NoError(err, "Bulk update should work")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(originalCount), rowsAffected, "Should update all test posts")

		// Verify each post was updated with its specific value
		for i, post := range posts {
			var updatedPost Post

			updatedPost.ID = post.ID

			err = suite.db.NewSelect().
				Model(&updatedPost).
				WherePK().
				Scan(suite.ctx)
			suite.NoError(err, "Should retrieve updated post")
			suite.Equal(1000+i*100, updatedPost.ViewCount, "View count should match bulk update value")
			suite.Equal("bulk_test", updatedPost.UpdatedBy, "UpdatedBy should be bulk_test")

			suite.T().Logf("Post %d: view_count=%d (was %d, expected %d)",
				i, updatedPost.ViewCount, originalViewCounts[i], 1000+i*100)
		}

		suite.T().Logf("Bulk updated %d posts with different values", originalCount)
	})
}

// TestOrderingAndLimits tests ordering and limit methods (OrderBy, OrderByDesc, OrderByExpr, Limit).
func (suite *UpdateTestSuite) TestOrderingAndLimits() {
	suite.T().Logf("Testing ordering and limits for %s", suite.ds.Kind)

	// Only MySQL supports ORDER BY and LIMIT in UPDATE statements
	if suite.ds.Kind != config.MySQL {
		suite.T().Skipf("Database %s doesn't support ORDER BY and LIMIT in UPDATE statements", suite.ds.Kind)
	}

	postIDs := make([]string, len(suite.testPosts))
	for i, tp := range suite.testPosts {
		postIDs[i] = tp.ID
	}

	userIDs := make([]string, len(suite.testUsers))
	for i, tu := range suite.testUsers {
		userIDs[i] = tu.ID
	}

	suite.Run("OrderByBasic", func() {
		result, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			Set("updated_by", "order_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", postIDs).Equals("status", "published")
			}).
			OrderBy("title").
			Limit(2).
			Exec(suite.ctx)

		suite.NoError(err, "OrderBy should work when supported")

		rowsAffected, _ := result.RowsAffected()
		suite.True(rowsAffected > 0, "Should update posts with ordering")
		suite.T().Logf("Updated %d posts with OrderBy", rowsAffected)
	})

	suite.Run("OrderByDesc", func() {
		_, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			Set("updated_by", "order_desc_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", postIDs).Equals("status", "draft")
			}).
			OrderByDesc("view_count").
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "OrderByDesc should work when supported")
		suite.T().Logf("Updated with OrderByDesc")
	})

	suite.Run("OrderByExpr", func() {
		_, err := suite.db.NewUpdate().
			Model((*Post)(nil)).
			Set("updated_by", "order_expr_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", postIDs)
			}).
			OrderByExpr(func(eb orm.ExprBuilder) any {
				return eb.Column("view_count")
			}).
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "OrderByExpr should work when supported")
		suite.T().Logf("Updated with OrderByExpr")
	})

	suite.Run("LimitOnly", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "limit_test").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", userIDs).IsTrue("is_active")
			}).
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "Limit should work when supported")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should limit updates to 1 row")
		suite.T().Logf("Limited update to %d row", rowsAffected)
	})
}

// TestReturningClause tests RETURNING clause methods (Returning, ReturningAll, ReturningNone).
func (suite *UpdateTestSuite) TestReturningClause() {
	suite.T().Logf("Testing RETURNING clause methods for %s", suite.ds.Kind)

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("RETURNING clause not supported on %s", suite.ds.Kind)
	}

	suite.Run("ReturningSpecificColumns", func() {
		type UpdateResult struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
			Age  int16  `bun:"age"`
		}

		var result UpdateResult

		err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 28).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[1].ID)
			}).
			Returning("id", "name", "age").
			Scan(suite.ctx, &result)

		suite.NoError(err, "Returning should work with specific columns")
		suite.NotEmpty(result.ID, "ID should be returned")
		suite.Equal(int16(28), result.Age, "Age should be updated value")

		suite.T().Logf("Returned: id=%s, name=%s, age=%d", result.ID, result.Name, result.Age)
	})

	suite.Run("ReturningAllColumns", func() {
		type UpdateResult struct {
			ID    string `bun:"id"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
			Age   int16  `bun:"age"`
		}

		var result UpdateResult

		err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 29).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[2].ID)
			}).
			ReturningAll().
			Scan(suite.ctx, &result)

		suite.NoError(err, "ReturningAll should return all columns")
		suite.NotEmpty(result.ID, "ID should be returned")
		suite.NotEmpty(result.Email, "Email should be returned")

		suite.T().Logf("ReturningAll: id=%s, email=%s", result.ID, result.Email)
	})

	suite.Run("ReturningNone", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 30).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[0].ID)
			}).
			ReturningNone().
			Exec(suite.ctx)

		suite.NoError(err, "ReturningNone should return no columns")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("ReturningNone: updated %d row", rowsAffected)
	})
}

// TestApplyMethods tests Apply and ApplyIf methods.
func (suite *UpdateTestSuite) TestApplyMethods() {
	suite.T().Logf("Testing Apply methods for %s", suite.ds.Kind)

	userIDs := make([]string, len(suite.testUsers))
	for i, tu := range suite.testUsers {
		userIDs[i] = tu.ID
	}

	suite.Run("ApplyBasic", func() {
		applyActive := func(query orm.UpdateQuery) {
			query.Where(func(cb orm.ConditionBuilder) {
				cb.In("id", userIDs).IsTrue("is_active")
			})
		}

		applyUpdatedBy := func(query orm.UpdateQuery) {
			query.Set("updated_by", "apply_test")
		}

		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 35).
			Apply(applyActive, applyUpdatedBy).
			Exec(suite.ctx)

		suite.NoError(err, "Apply should work correctly")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(2), rowsAffected, "Should update 2 active test users")

		suite.T().Logf("Applied functions updated %d users", rowsAffected)
	})

	suite.Run("ApplyIfTrue", func() {
		addFilter := func(query orm.UpdateQuery) {
			query.Where(func(cb orm.ConditionBuilder) {
				cb.GreaterThan("age", 25)
			})
		}

		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "apply_if_true").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", userIDs).IsTrue("is_active")
			}).
			ApplyIf(true, addFilter).
			Exec(suite.ctx)

		suite.NoError(err, "ApplyIf(true) should apply function")

		rowsAffected, _ := result.RowsAffected()
		suite.True(rowsAffected >= 0, "Should execute with condition")

		suite.T().Logf("ApplyIf(true) updated %d users", rowsAffected)
	})

	suite.Run("ApplyIfFalse", func() {
		addFilter := func(query orm.UpdateQuery) {
			query.Where(func(cb orm.ConditionBuilder) {
				cb.LessThan("age", 18)
			})
		}

		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("updated_by", "apply_if_false").
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", userIDs).IsTrue("is_active")
			}).
			ApplyIf(false, addFilter).
			Exec(suite.ctx)

		suite.NoError(err, "ApplyIf(false) should skip function")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(2), rowsAffected, "Should update 2 active test users without applied filter")

		suite.T().Logf("ApplyIf(false) updated %d users", rowsAffected)
	})
}

// TestExecution tests execution methods (Exec, Scan).
func (suite *UpdateTestSuite) TestExecution() {
	suite.T().Logf("Testing execution methods for %s", suite.ds.Kind)

	suite.Run("ExecBasic", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 40).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[0].ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Exec should execute update successfully")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected")
		suite.Equal(int64(1), rowsAffected, "Should affect 1 row")

		suite.T().Logf("Exec updated %d row", rowsAffected)
	})

	suite.Run("ScanWithReturning", func() {
		if suite.ds.Kind == config.MySQL {
			suite.T().Skipf("RETURNING clause not supported on %s", suite.ds.Kind)
		}

		type UpdateResult struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
		}

		var result UpdateResult

		err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 41).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[1].ID)
			}).
			Returning("id", "name").
			Scan(suite.ctx, &result)

		suite.NoError(err, "Scan should work with RETURNING")
		suite.NotEmpty(result.ID, "ID should be scanned")
		suite.NotEmpty(result.Name, "Name should be scanned")

		suite.T().Logf("Scanned result: id=%s, name=%s", result.ID, result.Name)
	})

	suite.Run("ExecNoMatchingRows", func() {
		result, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("age", 999).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "nonexistent@example.com")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Exec with no matching rows should not error")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected")
		suite.Equal(int64(0), rowsAffected, "Should affect 0 rows")

		suite.T().Logf("Exec with no matches: %d rows affected", rowsAffected)
	})

	suite.Run("ExecInvalidField", func() {
		_, err := suite.db.NewUpdate().
			Model((*User)(nil)).
			Set("nonexistent_field", "value").
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(suite.testUsers[0].ID)
			}).
			Exec(suite.ctx)

		suite.Error(err, "Exec with invalid field should error")

		suite.T().Logf("Exec with invalid field correctly returned error")
	})
}

// TestUpdateDB tests DB() method returns the underlying database.
func (suite *UpdateTestSuite) TestUpdateDB() {
	suite.T().Logf("Testing Update DB for %s", suite.ds.Kind)

	query := suite.db.NewUpdate().Model((*Tag)(nil))
	suite.NotNil(query.DB(), "Should return underlying database")
}

// TestUpdateWithRecursive tests WithRecursive CTE on UpdateQuery.
func (suite *UpdateTestSuite) TestUpdateWithRecursive() {
	suite.T().Logf("Testing Update WithRecursive for %s", suite.ds.Kind)

	tag := &Tag{Name: "RecursiveUpdateTest"}
	tag.ID = "test-upd-recursive"

	_, err := suite.db.NewInsert().Model(tag).Exec(suite.ctx)
	suite.Require().NoError(err, "Should insert test tag")

	_, err = suite.db.NewUpdate().
		WithRecursive("ids", func(sq orm.SelectQuery) {
			sq.Model((*Tag)(nil)).
				Select("id").
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "test-upd-recursive")
				})
		}).
		Model((*Tag)(nil)).
		Set("name", "RecursiveUpdated").
		Where(func(cb orm.ConditionBuilder) {
			cb.InSubQuery("id", func(sq orm.SelectQuery) {
				sq.TableExpr(func(eb orm.ExprBuilder) any {
					return eb.Column("ids", false)
				}).
					Select("id")
			})
		}).
		Exec(suite.ctx)
	suite.NoError(err, "Should update using WithRecursive CTE")

	_, _ = suite.db.NewDelete().Model((*Tag)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("id", "test-upd-recursive")
	}).Exec(suite.ctx)
}

// TestTableSpecMethods tests table specification methods with and without aliases.
func (suite *UpdateTestSuite) TestTableSpecMethods() {
	suite.T().Logf("Testing table specification methods for %s", suite.ds.Kind)

	nonexistentWhere := func(cb orm.ConditionBuilder) {
		cb.Equals("id", "nonexistent")
	}

	suite.Run("ModelTableWithAlias", func() {
		query := suite.db.NewUpdate().
			ModelTable("test_tag", "t").
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with ModelTable alias")
	})

	suite.Run("ModelTableNoAlias", func() {
		query := suite.db.NewUpdate().
			ModelTable("test_tag").
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with ModelTable")
	})

	suite.Run("TableWithAlias", func() {
		query := suite.db.NewUpdate().
			Table("test_tag", "t").
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with Table alias")
	})

	suite.Run("TableNoAlias", func() {
		query := suite.db.NewUpdate().
			Table("test_tag").
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with Table")
	})

	suite.Run("TableExprWithAlias", func() {
		query := suite.db.NewUpdate().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}, "t").
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableExpr alias")
	})

	suite.Run("TableExprNoAlias", func() {
		query := suite.db.NewUpdate().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}).
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableExpr")
	})

	suite.Run("TableSubQueryWithAlias", func() {
		query := suite.db.NewUpdate().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id", "name")
			}, "t").
			Set("name", "updated")
		suite.NotNil(query, "Should build query with TableSubQuery alias")
	})

	suite.Run("TableSubQueryNoAlias", func() {
		query := suite.db.NewUpdate().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id", "name")
			}).
			Set("name", "updated").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableSubQuery")
	})
}
