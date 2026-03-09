package orm_test

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &DeleteTestSuite{BaseTestSuite: base}
	})
}

// DeleteTestSuite tests DELETE operations across all databases.
type DeleteTestSuite struct {
	*BaseTestSuite
}

// TestCTE tests Common Table Expression methods (With, WithValues, WithRecursive).
func (suite *DeleteTestSuite) TestCTE() {
	suite.T().Logf("Testing CTE methods for %s", suite.ds.Kind)

	suite.Run("WithBasicCTE", func() {
		testUsers := []*User{
			{Name: "CTE Basic 1", Email: "cte_basic_1@example.com", Age: 25, IsActive: true},
			{Name: "CTE Basic 2", Email: "cte_basic_2@example.com", Age: 30, IsActive: false},
			{Name: "CTE Basic 3", Email: "cte_basic_3@example.com", Age: 35, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for CTE")

		result, err := suite.db.NewDelete().
			With("inactive_users", func(query orm.SelectQuery) {
				query.Model((*User)(nil)).
					Select("id").
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("is_active", false).
							StartsWith("email", "cte_basic_")
					})
			}).
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.InSubQuery("id", func(subquery orm.SelectQuery) {
					subquery.Table("inactive_users").Select("id")
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WITH clause should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 inactive user")

		suite.T().Logf("Deleted %d users using basic CTE", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "cte_basic_")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining CTE test users")
	})

	suite.Run("WithValuesCTE", func() {
		testPosts := []*Post{
			{Title: "CTE Values Post 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft", ViewCount: 10},
			{Title: "CTE Values Post 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published", ViewCount: 20},
			{Title: "CTE Values Post 3", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "review", ViewCount: 30},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for CTE VALUES")

		type StatusValue struct {
			Status string `bun:"status"`
		}

		statusValues := []StatusValue{
			{Status: "draft"},
			{Status: "review"},
		}

		result, err := suite.db.NewDelete().
			WithValues("target_statuses", &statusValues).
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.InSubQuery("status", func(subquery orm.SelectQuery) {
					subquery.Table("target_statuses").Select("status")
				}).
					StartsWith("title", "CTE Values Post")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WITH VALUES should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete 2 posts with draft or review status")

		suite.T().Logf("Deleted %d posts using CTE VALUES", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "CTE Values Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining CTE test posts")
	})

	suite.Run("WithRecursiveCTE", func() {
		if suite.ds.Kind == config.SQLite {
			suite.T().Skipf("SQLite recursive CTE with DELETE requires special handling on %s", suite.ds.Kind)

			return
		}

		testCategories := []*Category{
			{Name: "Parent Category", Description: nil},
			{Name: "Child Category 1", Description: nil},
			{Name: "Child Category 2", Description: nil},
		}

		_, err := suite.db.NewInsert().Model(&testCategories).Exec(suite.ctx)
		suite.NoError(err, "Should insert test categories")

		testPosts := []*Post{
			{Title: "Recursive Post 1", Content: "Content", UserID: "user1", CategoryID: testCategories[0].ID, Status: "published"},
			{Title: "Recursive Post 2", Content: "Content", UserID: "user1", CategoryID: testCategories[1].ID, Status: "published"},
		}

		_, err = suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for recursive CTE")

		result, err := suite.db.NewDelete().
			WithRecursive("category_tree", func(query orm.SelectQuery) {
				query.Model((*Category)(nil)).
					Select("id", "parent_id").
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("name", "Parent Category")
					}).
					UnionAll(func(unionQuery orm.SelectQuery) {
						unionQuery.Model((*Category)(nil)).
							Select("id", "parent_id").
							JoinTable("category_tree", func(cb orm.ConditionBuilder) {
								cb.EqualsColumn("parent_id", "ct.id")
							}, "ct")
					})
			}).
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.InSubQuery("category_id", func(subquery orm.SelectQuery) {
					subquery.Table("category_tree").Select("id")
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WITH RECURSIVE should work when supported")

		rowsAffected, _ := result.RowsAffected()
		suite.T().Logf("Deleted %d posts using recursive CTE", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Recursive Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining recursive test posts")

		_, err = suite.db.NewDelete().
			Model((*Category)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Contains("name", "Category")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup test categories")
	})
}

// TestTableSource tests table source methods (Model, ModelTable, Table, TableFrom, TableExpr, TableSubQuery).
func (suite *DeleteTestSuite) TestTableSource() {
	suite.T().Logf("Testing table source methods for %s", suite.ds.Kind)

	suite.Run("ModelAndModelTable", func() {
		testUsers := []*User{
			{Name: "Table User 1", Email: "table1@example.com", Age: 25, IsActive: true},
			{Name: "Table User 2", Email: "table2@example.com", Age: 30, IsActive: false},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "table1@example.com")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Model method should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 user using Model")

		suite.T().Logf("Deleted %d user using Model method", rowsAffected)

		result, err = suite.db.NewDelete().
			ModelTable("test_user", "u").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("u.email", "table2@example.com")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "ModelTable method should work correctly")

		rowsAffected, err = result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 user using ModelTable")

		suite.T().Logf("Deleted %d user using ModelTable method", rowsAffected)
	})

	suite.Run("TableAndTableFrom", func() {
		testPosts := []*Post{
			{Title: "Table Source Post 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
			{Title: "Table Source Post 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published"},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts")

		result, err := suite.db.NewDelete().
			Table("test_post", "p").
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("p.title", "Table Source Post").
					Equals("p.status", "draft")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Table method should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 post using Table")

		suite.T().Logf("Deleted %d post using Table method", rowsAffected)

		result, err = suite.db.NewDelete().
			TableFrom((*Post)(nil), "p").
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("p.title", "Table Source Post").
					Equals("p.status", "published")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "TableFrom method should work correctly")

		rowsAffected, err = result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 post using TableFrom")

		suite.T().Logf("Deleted %d post using TableFrom method", rowsAffected)
	})

	suite.Run("TableExpr", func() {
		testUsers := []*User{
			{Name: "Expr User", Email: "expr@example.com", Age: 28, IsActive: false},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test user for TableExpr")

		result, err := suite.db.NewDelete().
			Table("test_user", "u").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("u.email", "expr@example.com")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Table method should work for TableExpr test")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 user")

		suite.T().Logf("Deleted %d user using Table method (TableExpr functionality)", rowsAffected)
	})
}

// TestFiltering tests filtering methods (Where, WherePK, WhereDeleted, IncludeDeleted).
func (suite *DeleteTestSuite) TestFiltering() {
	suite.T().Logf("Testing filtering methods for %s", suite.ds.Kind)

	suite.Run("WhereCondition", func() {
		testUsers := []*User{
			{Name: "Filter User 1", Email: "filter1@example.com", Age: 25, IsActive: true},
			{Name: "Filter User 2", Email: "filter2@example.com", Age: 30, IsActive: false},
			{Name: "Filter User 3", Email: "filter3@example.com", Age: 35, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for filtering")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("is_active", false)
			}).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "filter")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WHERE with conditions should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 inactive user")

		suite.T().Logf("Deleted %d user using WHERE method", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "filter")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining filter test users")
	})

	suite.Run("WherePrimaryKey", func() {
		testUser := &User{
			Name:     "PK Delete User",
			Email:    "pk@example.com",
			Age:      28,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().Model(testUser).Exec(suite.ctx)
		suite.NoError(err, "Should insert test user for WherePK")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(testUser.ID)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WherePK should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 user by primary key")

		suite.T().Logf("Deleted user by primary key: %s", testUser.ID)
	})

	suite.Run("ComplexWhereConditions", func() {
		testPosts := []*Post{
			{Title: "Complex Filter 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft", ViewCount: 5},
			{Title: "Complex Filter 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published", ViewCount: 50},
			{Title: "Complex Filter 3", Content: "Content", UserID: "user2", CategoryID: "cat1", Status: "published", ViewCount: 150},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for complex filtering")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Complex Filter").
					Equals("status", "published").
					Group(func(innerCb orm.ConditionBuilder) {
						innerCb.LessThan("view_count", 100).OrEquals("user_id", "user2")
					})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Complex WHERE conditions should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete 2 posts matching complex conditions")

		suite.T().Logf("Deleted %d posts using complex WHERE conditions", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Complex Filter")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining complex filter test posts")
	})

	suite.Run("WhereWithSubquery", func() {
		testUsers := []*User{
			{Name: "Subquery User 1", Email: "subquser1@example.com", Age: 25, IsActive: false},
			{Name: "Subquery User 2", Email: "subquser2@example.com", Age: 30, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for subquery filtering")

		testPosts := []*Post{
			{Title: "Subquery Post 1", Content: "Content", UserID: testUsers[0].ID, CategoryID: "cat1", Status: "published"},
			{Title: "Subquery Post 2", Content: "Content", UserID: testUsers[1].ID, CategoryID: "cat1", Status: "published"},
		}

		_, err = suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for subquery filtering")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.InSubQuery("user_id", func(subquery orm.SelectQuery) {
					subquery.Model((*User)(nil)).
						Select("id").
						Where(func(cb orm.ConditionBuilder) {
							cb.Equals("is_active", false).
								StartsWith("email", "subquser")
						})
				}).
					StartsWith("title", "Subquery Post")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WHERE with subquery should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 post by inactive users")

		suite.T().Logf("Deleted %d post using WHERE with subquery", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Subquery Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining subquery test posts")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "subquser")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup subquery test users")
	})
}

// TestSoftDelete tests WhereDeleted and IncludeDeleted methods with a soft-delete model.
func (suite *DeleteTestSuite) TestSoftDelete() {
	suite.T().Logf("Testing WhereDeleted/IncludeDeleted for %s", suite.ds.Kind)

	type SoftDeleteArticle struct {
		bun.BaseModel `bun:"table:test_delete_soft_delete,alias:tdsd"`
		orm.Model

		Title     string    `json:"title" bun:"title,notnull"`
		Status    string    `json:"status" bun:"status,notnull"`
		DeletedAt time.Time `json:"deletedAt" bun:",soft_delete,nullzero"`
	}

	err := suite.db.ResetModel(suite.ctx, (*SoftDeleteArticle)(nil))

	suite.Require().NoError(err, "Should reset soft delete table")
	defer func() {
		_, _ = suite.db.NewDropTable().Model((*SoftDeleteArticle)(nil)).IfExists().Exec(suite.ctx)
	}()

	records := []*SoftDeleteArticle{
		{Title: "Article A", Status: "draft"},
		{Title: "Article B", Status: "published"},
		{Title: "Article C", Status: "archived"},
	}

	_, err = suite.db.NewInsert().Model(&records).Exec(suite.ctx)
	suite.Require().NoError(err, "Should insert soft delete test records")

	// Soft delete the first record
	_, err = suite.db.NewDelete().
		Model((*SoftDeleteArticle)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", records[0].ID)
		}).
		Exec(suite.ctx)
	suite.Require().NoError(err, "Should soft delete first record")

	suite.Run("WhereDeleted", func() {
		// Delete (force) among soft-deleted records only
		result, err := suite.db.NewDelete().
			Model((*SoftDeleteArticle)(nil)).
			WhereDeleted().
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", records[0].ID)
			}).
			ForceDelete().
			Exec(suite.ctx)
		suite.Require().NoError(err, "WhereDeleted should target soft-deleted rows")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(1), rowsAffected, "Should force delete 1 soft-deleted record")
	})

	suite.Run("IncludeDeleted", func() {
		// Re-insert and soft delete again for this sub-test
		newRecord := &SoftDeleteArticle{Title: "Article D", Status: "draft"}

		_, err := suite.db.NewInsert().Model(newRecord).Exec(suite.ctx)
		suite.Require().NoError(err, "Should insert new record for sub-test")

		_, err = suite.db.NewDelete().
			Model((*SoftDeleteArticle)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("id", newRecord.ID)
			}).
			Exec(suite.ctx)
		suite.Require().NoError(err, "Should soft delete new record")

		// IncludeDeleted should see all records
		result, err := suite.db.NewDelete().
			Model((*SoftDeleteArticle)(nil)).
			IncludeDeleted().
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", []string{records[1].ID, newRecord.ID})
			}).
			ForceDelete().
			Exec(suite.ctx)
		suite.Require().NoError(err, "IncludeDeleted should allow deleting all records")

		rowsAffected, _ := result.RowsAffected()
		suite.Equal(int64(2), rowsAffected, "Should force delete 2 records (1 active + 1 soft-deleted)")
	})
}

// TestOrdering tests ordering methods combined with Limit (MySQL only).
func (suite *DeleteTestSuite) TestOrdering() {
	suite.T().Logf("Testing ordering methods for %s", suite.ds.Kind)

	if suite.ds.Kind == config.Postgres || suite.ds.Kind == config.SQLite {
		suite.T().Skipf("%s doesn't support DELETE with ORDER BY/LIMIT in current configuration", suite.ds.Kind)

		return
	}

	suite.Run("OrderByAscending", func() {
		testUsers := []*User{
			{Name: "Order User C", Email: "order_c@example.com", Age: 35, IsActive: true},
			{Name: "Order User A", Email: "order_a@example.com", Age: 25, IsActive: true},
			{Name: "Order User B", Email: "order_b@example.com", Age: 30, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for ordering")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "order_")
			}).
			OrderBy("age").
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "OrderBy should work correctly with Limit")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 user (youngest)")

		var remainingUsers []User

		err = suite.db.NewSelect().
			Model(&remainingUsers).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "order_")
			}).
			OrderBy("age").
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve remaining users")
		suite.Len(remainingUsers, 2, "Should have 2 remaining users")
		suite.True(remainingUsers[0].Age >= 30, "Youngest user should be deleted")

		suite.T().Logf("Deleted youngest user using OrderBy, %d users remain", len(remainingUsers))

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "order_")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining order test users")
	})

	suite.Run("OrderByDescending", func() {
		testPosts := []*Post{
			{Title: "Order Post A", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published", ViewCount: 10},
			{Title: "Order Post B", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published", ViewCount: 50},
			{Title: "Order Post C", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published", ViewCount: 100},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for descending ordering")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Order Post")
			}).
			OrderByDesc("view_count").
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "OrderByDesc should work correctly with Limit")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 post (highest view count)")

		var remainingPosts []Post

		err = suite.db.NewSelect().
			Model(&remainingPosts).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Order Post")
			}).
			OrderByDesc("view_count").
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve remaining posts")
		suite.Len(remainingPosts, 2, "Should have 2 remaining posts")
		suite.True(remainingPosts[0].ViewCount <= 50, "Highest view count post should be deleted")

		suite.T().Logf("Deleted highest view count post using OrderByDesc, %d posts remain", len(remainingPosts))

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Order Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining order test posts")
	})

	suite.Run("OrderByExpression", func() {
		testUsers := []*User{
			{Name: "Expr Order 1", Email: "exprorder1@example.com", Age: 25, IsActive: true},
			{Name: "Expr Order 2", Email: "exprorder2@example.com", Age: 30, IsActive: false},
			{Name: "Expr Order 3", Email: "exprorder3@example.com", Age: 35, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for expression ordering")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "exprorder")
			}).
			OrderByExpr(func(eb orm.ExprBuilder) any {
				return eb.Case(func(cb orm.CaseBuilder) {
					cb.When(func(cond orm.ConditionBuilder) {
						cond.IsTrue("is_active")
					}).Then(eb.Column("age"))
					cb.Else(eb.Literal("0"))
				})
			}).
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "OrderByExpr should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.True(rowsAffected >= 1, "Should delete at least 1 user")

		suite.T().Logf("Deleted %d user using OrderByExpr", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "exprorder")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining expr order test users")
	})
}

// TestLimit tests the Limit method (MySQL only).
func (suite *DeleteTestSuite) TestLimit() {
	suite.T().Logf("Testing Limit method for %s", suite.ds.Kind)

	if suite.ds.Kind == config.Postgres || suite.ds.Kind == config.SQLite {
		suite.T().Skipf("%s doesn't support DELETE with LIMIT in current configuration", suite.ds.Kind)

		return
	}

	suite.Run("LimitBasic", func() {
		testPosts := []*Post{
			{Title: "Limit Post 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
			{Title: "Limit Post 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
			{Title: "Limit Post 3", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
			{Title: "Limit Post 4", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for limit")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Limit Post")
			}).
			Limit(2).
			Exec(suite.ctx)

		suite.NoError(err, "Limit should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete exactly 2 posts")

		var remainingPosts []Post

		err = suite.db.NewSelect().
			Model(&remainingPosts).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Limit Post")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve remaining posts")
		suite.Len(remainingPosts, 2, "Should have 2 remaining posts")

		suite.T().Logf("Deleted %d posts with Limit, %d posts remain", rowsAffected, len(remainingPosts))

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Limit Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining limit test posts")
	})

	suite.Run("LimitWithOrdering", func() {
		testUsers := []*User{
			{Name: "Limit User 1", Email: "limituser1@example.com", Age: 20, IsActive: true},
			{Name: "Limit User 2", Email: "limituser2@example.com", Age: 30, IsActive: true},
			{Name: "Limit User 3", Email: "limituser3@example.com", Age: 40, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for limit with ordering")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "limituser")
			}).
			OrderBy("age").
			Limit(1).
			Exec(suite.ctx)

		suite.NoError(err, "Limit with ordering should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete exactly 1 user (youngest)")

		var remainingUsers []User

		err = suite.db.NewSelect().
			Model(&remainingUsers).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "limituser")
			}).
			OrderBy("age").
			Scan(suite.ctx)
		suite.NoError(err, "Should retrieve remaining users")
		suite.Len(remainingUsers, 2, "Should have 2 remaining users")
		suite.True(remainingUsers[0].Age >= 30, "Youngest user should be deleted")

		suite.T().Logf("Deleted youngest user with Limit and OrderBy, %d users remain", len(remainingUsers))

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "limituser")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining limit test users")
	})

	suite.Run("LimitEdgeCases", func() {
		testPosts := []*Post{
			{Title: "Edge Limit 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published"},
			{Title: "Edge Limit 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published"},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for edge cases")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Edge Limit")
			}).
			Limit(10).
			Exec(suite.ctx)

		suite.NoError(err, "Limit larger than row count should work")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete all 2 posts even with Limit(10)")

		suite.T().Logf("Deleted all %d posts with Limit(10)", rowsAffected)
	})
}

// TestReturning tests Returning methods (PostgreSQL/SQLite only).
func (suite *DeleteTestSuite) TestReturning() {
	suite.T().Logf("Testing Returning methods for %s", suite.ds.Kind)

	if suite.ds.Kind == config.MySQL {
		suite.T().Skipf("RETURNING clause not supported on %s", suite.ds.Kind)

		return
	}

	suite.Run("ReturningSpecificColumns", func() {
		testUsers := []*User{
			{Name: "Return User 1", Email: "return1@example.com", Age: 25, IsActive: true},
			{Name: "Return User 2", Email: "return2@example.com", Age: 30, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for returning")

		type DeleteResult struct {
			ID    string `bun:"id"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		var returnedUsers []DeleteResult

		err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "return")
			}).
			Returning("id", "name", "email").
			Scan(suite.ctx, &returnedUsers)

		suite.NoError(err, "Should delete records with RETURNING clause")
		suite.Len(returnedUsers, 2, "Should return 2 deleted user records")

		for i, user := range returnedUsers {
			suite.NotEmpty(user.ID, "Returned ID should not be empty")
			suite.NotEmpty(user.Name, "Returned name should not be empty")
			suite.NotEmpty(user.Email, "Returned email should not be empty")
			suite.T().Logf("Returned user %d: ID=%s, Name=%s, Email=%s", i+1, user.ID, user.Name, user.Email)
		}

		var deletedUsers []User

		err = suite.db.NewSelect().
			Model(&deletedUsers).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "return")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should query deleted users")
		suite.Len(deletedUsers, 0, "Returned users should not exist in database")

		suite.T().Logf("Deleted %d users with RETURNING specific columns", len(returnedUsers))
	})

	suite.Run("ReturningAllColumns", func() {
		testPosts := []*Post{
			{Title: "Return All Post", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft", ViewCount: 10},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test post for returning all")

		var returnedPosts []Post

		err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("title", "Return All Post")
			}).
			ReturningAll().
			Scan(suite.ctx, &returnedPosts)

		suite.NoError(err, "Should delete records with RETURNING ALL")
		suite.Len(returnedPosts, 1, "Should return 1 deleted post")

		post := returnedPosts[0]
		suite.NotEmpty(post.ID, "Returned ID should not be empty")
		suite.Equal("Return All Post", post.Title, "Returned title should match")
		suite.Equal("draft", post.Status, "Returned status should match")
		suite.Equal(10, post.ViewCount, "Returned view count should match")

		suite.T().Logf("Deleted 1 post with RETURNING ALL: ID=%s, Title=%s", post.ID, post.Title)
	})

	suite.Run("ReturningNoColumns", func() {
		testUser := &User{
			Name:     "Return None User",
			Email:    "returnnone@example.com",
			Age:      28,
			IsActive: true,
		}

		_, err := suite.db.NewInsert().Model(testUser).Exec(suite.ctx)
		suite.NoError(err, "Should insert test user for returning none")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "returnnone@example.com")
			}).
			ReturningNone().
			Exec(suite.ctx)

		suite.NoError(err, "Should delete record with RETURNING NONE")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 user")

		suite.T().Logf("Deleted 1 user with RETURNING NONE (no data returned)")
	})
}

// TestForceDelete tests ForceDelete method with soft-delete scenarios.
func (suite *DeleteTestSuite) TestForceDelete() {
	suite.T().Logf("Testing ForceDelete method for %s", suite.ds.Kind)

	suite.Run("ForceDeleteNonDeleted", func() {
		testUsers := []*User{
			{Name: "Force User 1", Email: "force1@example.com", Age: 25, IsActive: true},
			{Name: "Force User 2", Email: "force2@example.com", Age: 30, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for force delete")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "force1@example.com")
			}).
			ForceDelete().
			Exec(suite.ctx)

		suite.NoError(err, "ForceDelete should work on non-deleted records")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should force delete 1 user")

		var deletedUser User

		err = suite.db.NewSelect().
			Model(&deletedUser).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "force1@example.com")
			}).
			Scan(suite.ctx)
		suite.Error(err, "Force deleted user should not exist")

		suite.T().Logf("Force deleted 1 non-deleted user")

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "force2@example.com")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining force test user")
	})

	suite.Run("ForceDeleteBehavior", func() {
		testPosts := []*Post{
			{Title: "Force Post 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published"},
			{Title: "Force Post 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for force delete behavior")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Force Post")
			}).
			ForceDelete().
			Exec(suite.ctx)

		suite.NoError(err, "ForceDelete should delete permanently")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should permanently delete 2 posts")

		var deletedPosts []Post

		err = suite.db.NewSelect().
			Model(&deletedPosts).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Force Post")
			}).
			Scan(suite.ctx)
		suite.NoError(err, "Should query force deleted posts")
		suite.Len(deletedPosts, 0, "Force deleted posts should not exist")

		suite.T().Logf("Permanently deleted %d posts with ForceDelete", rowsAffected)
	})
}

// TestApply tests Apply and ApplyIf methods for conditional query building.
func (suite *DeleteTestSuite) TestApply() {
	suite.T().Logf("Testing Apply methods for %s", suite.ds.Kind)

	suite.Run("ApplyBasic", func() {
		testUsers := []*User{
			{Name: "Apply User 1", Email: "apply1@example.com", Age: 25, IsActive: true},
			{Name: "Apply User 2", Email: "apply2@example.com", Age: 30, IsActive: false},
			{Name: "Apply User 3", Email: "apply3@example.com", Age: 35, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for apply")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Apply(
				func(query orm.DeleteQuery) {
					query.Where(func(cb orm.ConditionBuilder) {
						cb.StartsWith("email", "apply")
					})
				},
				func(query orm.DeleteQuery) {
					query.Where(func(cb orm.ConditionBuilder) {
						cb.Equals("is_active", false)
					})
				},
			).
			Exec(suite.ctx)

		suite.NoError(err, "Apply should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 inactive user (applied filter)")

		suite.T().Logf("Deleted %d user using Apply method", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "apply")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining apply test users")
	})

	suite.Run("ApplyIfTrue", func() {
		testPosts := []*Post{
			{Title: "ApplyIf Post 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
			{Title: "ApplyIf Post 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published"},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for ApplyIf true")

		result, err := suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "ApplyIf Post")
			}).
			ApplyIf(true,
				func(query orm.DeleteQuery) {
					query.Where(func(cb orm.ConditionBuilder) {
						cb.Equals("status", "draft")
					})
				},
			).
			Exec(suite.ctx)

		suite.NoError(err, "ApplyIf(true) should apply functions")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(1), rowsAffected, "Should delete 1 draft post (condition was true)")

		suite.T().Logf("Deleted %d post using ApplyIf(true)", rowsAffected)

		_, err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "ApplyIf Post")
			}).
			Exec(suite.ctx)
		suite.NoError(err, "Should cleanup remaining ApplyIf test posts")
	})

	suite.Run("ApplyIfFalse", func() {
		testUsers := []*User{
			{Name: "ApplyIf False 1", Email: "applyfalse1@example.com", Age: 25, IsActive: true},
			{Name: "ApplyIf False 2", Email: "applyfalse2@example.com", Age: 30, IsActive: false},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for ApplyIf false")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "applyfalse")
			}).
			ApplyIf(false,
				func(query orm.DeleteQuery) {
					query.Where(func(cb orm.ConditionBuilder) {
						cb.Equals("is_active", false)
					})
				},
			).
			Exec(suite.ctx)

		suite.NoError(err, "ApplyIf(false) should skip functions")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete all 2 users (no filter applied)")

		suite.T().Logf("Deleted %d users using ApplyIf(false) - no condition applied", rowsAffected)
	})
}

// TestExecution tests execution methods (Exec, Scan) with various scenarios.
func (suite *DeleteTestSuite) TestExecution() {
	suite.T().Logf("Testing execution methods for %s", suite.ds.Kind)

	suite.Run("ExecSuccess", func() {
		testUsers := []*User{
			{Name: "Exec User 1", Email: "exec1@example.com", Age: 25, IsActive: true},
			{Name: "Exec User 2", Email: "exec2@example.com", Age: 30, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users for exec")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "exec")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Exec should work correctly")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete 2 users")

		suite.T().Logf("Exec deleted %d users successfully", rowsAffected)
	})

	suite.Run("ExecNoRows", func() {
		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("email", "nonexistent@example.com")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Exec with no matching rows should not error")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(0), rowsAffected, "Should affect 0 rows")

		suite.T().Logf("Exec with no matching rows affected %d rows", rowsAffected)
	})

	suite.Run("ScanWithReturning", func() {
		if suite.ds.Kind == config.MySQL {
			suite.T().Skipf("RETURNING with Scan not supported on %s", suite.ds.Kind)

			return
		}

		testPosts := []*Post{
			{Title: "Scan Post 1", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "draft"},
			{Title: "Scan Post 2", Content: "Content", UserID: "user1", CategoryID: "cat1", Status: "published"},
		}

		_, err := suite.db.NewInsert().Model(&testPosts).Exec(suite.ctx)
		suite.NoError(err, "Should insert test posts for scan")

		type DeleteResult struct {
			ID     string `bun:"id"`
			Title  string `bun:"title"`
			Status string `bun:"status"`
		}

		var returnedPosts []DeleteResult

		err = suite.db.NewDelete().
			Model((*Post)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("title", "Scan Post")
			}).
			Returning("id", "title", "status").
			Scan(suite.ctx, &returnedPosts)

		suite.NoError(err, "Scan with RETURNING should work correctly")
		suite.Len(returnedPosts, 2, "Should return 2 deleted posts")

		for _, post := range returnedPosts {
			suite.NotEmpty(post.ID, "Returned ID should not be empty")
			suite.NotEmpty(post.Title, "Returned title should not be empty")
			suite.T().Logf("Scanned deleted post: ID=%s, Title=%s, Status=%s", post.ID, post.Title, post.Status)
		}
	})

	suite.Run("ErrorHandling", func() {
		_, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("invalid_field", "value")
			}).
			Exec(suite.ctx)

		suite.Error(err, "DELETE with invalid field should error")
		suite.T().Logf("Invalid field error correctly returned: %v", err)
	})

	suite.Run("DeleteWithWhereClause", func() {
		testUsers := []*User{
			{Name: "ExecWhere User 1", Email: "exec-where-1@example.com", Age: 20, IsActive: true},
			{Name: "ExecWhere User 2", Email: "exec-where-2@example.com", Age: 25, IsActive: true},
		}

		_, err := suite.db.NewInsert().Model(&testUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert test users")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("name", "ExecWhere User")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "DELETE with WHERE clause should work")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count")
		suite.Equal(int64(2), rowsAffected, "Should delete all test records")

		suite.T().Logf("Deleted %d users with WHERE clause", rowsAffected)
	})

	suite.Run("PerformanceBulkDelete", func() {
		batchSize := 50

		performanceUsers := make([]*User, batchSize)
		for i := range batchSize {
			performanceUsers[i] = &User{
				Name:     fmt.Sprintf("Perf User %d", i),
				Email:    fmt.Sprintf("perf-%03d@example.com", i),
				Age:      int16(20 + i%50),
				IsActive: i%2 == 0,
			}
		}

		_, err := suite.db.NewInsert().Model(&performanceUsers).Exec(suite.ctx)
		suite.NoError(err, "Should insert performance test users")

		result, err := suite.db.NewDelete().
			Model((*User)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.StartsWith("email", "perf-")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Should bulk delete performance test users")

		rowsAffected, err := result.RowsAffected()
		suite.NoError(err, "Should get rows affected count for bulk delete")
		suite.Equal(int64(batchSize), rowsAffected, "Should delete all performance test users")

		suite.T().Logf("Performance test: deleted %d users", batchSize)
	})
}

// TestDeleteDB tests DB() method returns the underlying database.
func (suite *DeleteTestSuite) TestDeleteDB() {
	suite.T().Logf("Testing Delete DB for %s", suite.ds.Kind)

	query := suite.db.NewDelete().Model((*Tag)(nil))
	suite.NotNil(query.DB(), "Should return underlying database")
}

// TestTableSpecMethods tests table specification methods with and without aliases.
func (suite *DeleteTestSuite) TestTableSpecMethods() {
	suite.T().Logf("Testing table specification methods for %s", suite.ds.Kind)

	nonexistentWhere := func(cb orm.ConditionBuilder) {
		cb.Equals("id", "nonexistent")
	}

	suite.Run("ModelTableWithAlias", func() {
		query := suite.db.NewDelete().
			ModelTable("test_tag", "t").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with ModelTable alias")
	})

	suite.Run("ModelTableNoAlias", func() {
		query := suite.db.NewDelete().
			ModelTable("test_tag").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with ModelTable")
	})

	suite.Run("TableWithAlias", func() {
		query := suite.db.NewDelete().
			Table("test_tag", "t").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with Table alias")
	})

	suite.Run("TableNoAlias", func() {
		query := suite.db.NewDelete().
			Table("test_tag").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with Table")
	})

	suite.Run("TableExprWithAlias", func() {
		query := suite.db.NewDelete().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}, "t").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableExpr alias")
	})

	suite.Run("TableExprNoAlias", func() {
		query := suite.db.NewDelete().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}).
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableExpr")
	})

	suite.Run("TableSubQueryWithAlias", func() {
		query := suite.db.NewDelete().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id", "name")
			}, "t").
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableSubQuery alias")
	})

	suite.Run("TableSubQueryNoAlias", func() {
		query := suite.db.NewDelete().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id")
			}).
			Where(nonexistentWhere)
		suite.NotNil(query, "Should build query with TableSubQuery")
	})
}
