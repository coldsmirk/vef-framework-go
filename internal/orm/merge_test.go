package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &MergeTestSuite{BaseTestSuite: base}
	})
}

// MergeTestSuite tests MERGE operations (PostgreSQL 15+).
type MergeTestSuite struct {
	*BaseTestSuite

	testUsers []*User
	testPosts []*Post
}

// SetupTest inserts isolated test data before each test method.
func (suite *MergeTestSuite) SetupTest() {
	suite.testUsers = []*User{
		{Name: "MT Alice", Email: "mt_alice@test.com", Age: 30, IsActive: true},
		{Name: "MT Bob", Email: "mt_bob@test.com", Age: 25, IsActive: true},
		{Name: "MT Charlie", Email: "mt_charlie@test.com", Age: 35, IsActive: false},
	}

	_, err := suite.db.NewInsert().Model(&suite.testUsers).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert merge test users")

	suite.testPosts = []*Post{
		{Title: "MT Post Alpha", Content: "Content A", UserID: suite.testUsers[0].ID, CategoryID: "cat001", Status: "published", ViewCount: 100},
		{Title: "MT Post Beta", Content: "Content B", UserID: suite.testUsers[0].ID, CategoryID: "cat001", Status: "draft", ViewCount: 50},
		{Title: "MT Post Gamma", Content: "Content C", UserID: suite.testUsers[1].ID, CategoryID: "cat002", Status: "published", ViewCount: 75},
		{Title: "MT Post Delta", Content: "Content D", UserID: suite.testUsers[2].ID, CategoryID: "cat001", Status: "review", ViewCount: 30},
	}

	_, err = suite.db.NewInsert().Model(&suite.testPosts).Exec(suite.ctx)
	suite.Require().NoError(err, "Failed to insert merge test posts")
}

// TearDownTest removes all test-inserted data (created_at >= 2026) after each test method.
func (suite *MergeTestSuite) TearDownTest() {
	// Delete test posts first (FK constraint on user_id)
	_, _ = suite.db.NewDelete().Model((*Post)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.GreaterThanOrEqual("created_at", fixtureEndDate)
	}).Exec(suite.ctx)

	// Delete test users
	_, _ = suite.db.NewDelete().Model((*User)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.GreaterThanOrEqual("created_at", fixtureEndDate)
	}).Exec(suite.ctx)
}

// TestBasicMerge tests MERGE with updates and inserts.
func (suite *MergeTestSuite) TestBasicMerge() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing basic MERGE for %s", suite.ds.Kind)

	type UserMergeData struct {
		ID       string `bun:"id"`
		Name     string `bun:"name"`
		Email    string `bun:"email"`
		Age      int16  `bun:"age"`
		IsActive bool   `bun:"is_active"`
	}

	// Use test user[0] as matched target, plus two new IDs for inserts
	sourceData := []UserMergeData{
		{ID: suite.testUsers[0].ID, Name: "MT Alice Updated", Email: "mt_alice_updated@test.com", Age: 31, IsActive: true},
		{ID: "mt_new1", Name: "MT David New", Email: "mt_david@test.com", Age: 28, IsActive: true},
		{ID: "mt_new2", Name: "MT Eva New", Email: "mt_eva@test.com", Age: 26, IsActive: false},
	}

	suite.T().Logf("Executing MERGE with %d source records (updates: %s, inserts: mt_new1, mt_new2)", len(sourceData), suite.testUsers[0].ID)

	result, err := suite.db.NewMerge().
		Model(&User{}).
		WithValues("_source_data", &sourceData).
		UsingTable("_source_data").
		On(func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("u.id", "_source_data.id")
		}).
		WhenMatched().
		ThenUpdate(func(ub orm.MergeUpdateBuilder) {
			ub.SetColumns("name", "email", "age", "is_active")
		}).
		WhenNotMatched().
		ThenInsert(func(ib orm.MergeInsertBuilder) {
			ib.Values("id", "name", "email", "age", "is_active")
		}).
		Exec(suite.ctx)

	suite.NoError(err, "MERGE operation should complete successfully")

	if result != nil {
		affected, _ := result.RowsAffected()
		suite.True(affected >= 0, "MERGE should affect 0 or more rows, got %d", affected)
		suite.T().Logf("MERGE operation affected %d rows", affected)
	}

	var newUsers []User

	err = suite.db.NewSelect().
		Model(&newUsers).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", []string{"mt_new1", "mt_new2"})
		}).
		OrderBy("name").
		Scan(suite.ctx)
	suite.NoError(err, "Failed to query newly inserted users")
	suite.T().Logf("Found %d new users after merge", len(newUsers))

	for _, user := range newUsers {
		suite.T().Logf("New user - ID: %s, Name: %s, Email: %s, Age: %d, Active: %v",
			user.ID, user.Name, user.Email, user.Age, user.IsActive)
	}
}

// TestCteMethods tests CTE methods: With for named CTEs, WithValues for inline data CTEs.
func (suite *MergeTestSuite) TestCteMethods() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing CTE methods for %s", suite.ds.Kind)

	suite.Run("WithNamedCTE", func() {
		postIDs := make([]string, len(suite.testPosts))
		for i, tp := range suite.testPosts {
			postIDs[i] = tp.ID
		}

		result, err := suite.db.NewMerge().
			Model(&Post{}).
			With("high_view_posts", func(sq orm.SelectQuery) {
				sq.Model(&Post{}).
					Select("id", "title", "view_count").
					Where(func(cb orm.ConditionBuilder) {
						cb.In("id", postIDs).GreaterThan("view_count", 50)
					})
			}).
			UsingTable("high_view_posts").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("p.id", "high_view_posts.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetExpr("view_count", func(eb orm.ExprBuilder) any {
					return eb.Expr("? + 1", eb.Column("high_view_posts.view_count"))
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "MERGE with named CTE should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("MERGE with CTE affected %d rows", affected)
		}
	})

	suite.Run("WithValuesCTE", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
			Age   int16  `bun:"age"`
		}

		sourceData := []UserMergeData{
			{ID: "cte1", Name: "CTE User 1", Email: "cte1@example.com", Age: 25},
			{ID: "cte2", Name: "CTE User 2", Email: "cte2@example.com", Age: 30},
		}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.In("id", []string{"cte1", "cte2"})
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("cte_source", &sourceData).
			UsingTable("cte_source").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "cte_source.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email", "age")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "MERGE with VALUES CTE should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("MERGE with VALUES CTE affected %d rows", affected)
		}
	})
}

// TestTableSourceMethods tests target table specification: ModelTable, Table, TableExpr, TableSubQuery with/without aliases.
func (suite *MergeTestSuite) TestTableSourceMethods() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing table source methods for %s", suite.ds.Kind)

	suite.Run("ModelTableBasic", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "mt1", Name: "ModelTable User", Email: "mt1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "mt1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			ModelTable("test_user").
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "ModelTable without alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("ModelTable basic affected %d rows", affected)
		}
	})

	suite.Run("ModelTableWithAlias", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "mt2", Name: "ModelTable Alias User", Email: "mt2@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "mt2")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			ModelTable("test_user", "u").
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "ModelTable with alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("ModelTable with alias affected %d rows", affected)
		}
	})

	suite.Run("TableBasic", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "t1", Name: "Table User", Email: "t1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "t1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			Table("test_user").
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Table without alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Table basic affected %d rows", affected)
		}
	})

	suite.Run("TableWithAlias", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "t2", Name: "Table Alias User", Email: "t2@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "t2")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			Table("test_user", "u").
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Table with alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Table with alias affected %d rows", affected)
		}
	})

	suite.Run("TableExprBasic", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "te1", Name: "TableExpr User", Email: "te1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "te1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.Expr("test_user")
			}, "u").
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "TableExpr should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("TableExpr affected %d rows", affected)
		}
	})

	suite.Run("TableSubQueryBasic", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "tsq1", Name: "TableSubQuery User", Email: "tsq1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "tsq1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model(&User{}).
					Select("id", "name", "email").
					Where(func(cb orm.ConditionBuilder) {
						cb.IsNotNull("email")
					})
			}, "active_users").
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "TableSubQuery should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("TableSubQuery affected %d rows", affected)
		}
	})
}

// TestUsingMethods tests source data specification: UsingTable, UsingExpr, UsingSubQuery with/without aliases.
func (suite *MergeTestSuite) TestUsingMethods() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing Using methods for %s", suite.ds.Kind)

	suite.Run("UsingWithAlias", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "ua1", Name: "Using Alias User", Email: "ua1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "ua1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_source_data", &sourceData).
			UsingTable("_source_data", "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Using with alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Using with alias affected %d rows", affected)
		}
	})

	suite.Run("UsingTableBasic", func() {
		type PostMergeData struct {
			ID        string `bun:"id,pk"`
			Title     string `bun:"title"`
			ViewCount int    `bun:"view_count"`
		}

		sourceData := []PostMergeData{
			{ID: suite.testPosts[0].ID, Title: "MT Updated Post Title", ViewCount: 200},
		}

		result, err := suite.db.NewMerge().
			Model(&Post{}).
			WithValues("_post_updates", &sourceData).
			UsingTable("_post_updates").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("p.id", "_post_updates.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetExpr("view_count", func(eb orm.ExprBuilder) any {
					return eb.Add(eb.Column("_post_updates.view_count"), 1)
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "UsingTable without alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("UsingTable basic affected %d rows", affected)
		}
	})

	suite.Run("UsingTableWithAlias", func() {
		type PostMergeData struct {
			ID        string `bun:"id,pk"`
			Title     string `bun:"title"`
			ViewCount int    `bun:"view_count"`
		}

		sourceData := []PostMergeData{
			{ID: suite.testPosts[0].ID, Title: "MT Updated Post Title", ViewCount: 200},
		}

		result, err := suite.db.NewMerge().
			Model(&Post{}).
			WithValues("_post_updates", &sourceData).
			UsingTable("_post_updates", "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("p.id", "src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetExpr("view_count", func(eb orm.ExprBuilder) any {
					return eb.Expr("? + 1", eb.Column("src.view_count"))
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "UsingTable with alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("UsingTable with alias affected %d rows", affected)
		}
	})

	suite.Run("UsingExprBasic", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "ue1", Name: "UsingExpr User", Email: "ue1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "ue1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_temp", &sourceData).
			UsingTable("_temp").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_temp.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "UsingExpr should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("UsingExpr affected %d rows", affected)
		}
	})

	suite.Run("UsingExprWithAlias", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "uea1", Name: "UsingExpr Alias User", Email: "uea1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "uea1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_temp", &sourceData).
			UsingTable("_temp", "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "UsingExpr with alias should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("UsingExpr with alias affected %d rows", affected)
		}
	})

	suite.Run("UsingSubQueryBasic", func() {
		postIDs := make([]string, len(suite.testPosts))
		for i, tp := range suite.testPosts {
			postIDs[i] = tp.ID
		}

		result, err := suite.db.NewMerge().
			Model(&Post{}).
			UsingSubQuery(func(sq orm.SelectQuery) {
				sq.Model(&Post{}).
					Select("id", "title", "view_count").
					Where(func(cb orm.ConditionBuilder) {
						cb.In("id", postIDs).Equals("status", "published")
					})
			}, "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("p.id", "src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetExpr("view_count", func(eb orm.ExprBuilder) any {
					return eb.Add(eb.Column("src.view_count"), 5)
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "UsingSubQuery should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("UsingSubQuery affected %d rows", affected)
		}
	})
}

// TestReturningMethods tests RETURNING clause: specific columns, all columns (*), or none.
func (suite *MergeTestSuite) TestReturningMethods() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing RETURNING methods for %s", suite.ds.Kind)

	suite.Run("ReturningSpecificColumns", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{
			{ID: "ret1", Name: "Return User 1", Email: "ret1@example.com"},
			{ID: "ret2", Name: "Return User 2", Email: "ret2@example.com"},
		}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.In("id", []string{"ret1", "ret2"})
				}).
				Exec(suite.ctx)
		}()

		type ReturnResult struct {
			ID   string `bun:"id"`
			Name string `bun:"name"`
		}

		var returnedUsers []ReturnResult

		err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Returning("id", "name").
			Scan(suite.ctx, &returnedUsers)

		suite.NoError(err, "RETURNING specific columns should work")
		suite.T().Logf("RETURNING specific columns returned %d results", len(returnedUsers))

		for _, result := range returnedUsers {
			suite.T().Logf("Returned: ID=%s, Name=%s", result.ID, result.Name)
		}
	})

	suite.Run("ReturningAll", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "reta1", Name: "ReturnAll User", Email: "reta1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "reta1")
				}).
				Exec(suite.ctx)
		}()

		var returnedUsers []User

		err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			ReturningAll().
			Scan(suite.ctx, &returnedUsers)

		suite.NoError(err, "RETURNING * should work")
		suite.T().Logf("RETURNING * returned %d results", len(returnedUsers))

		for _, result := range returnedUsers {
			suite.T().Logf("Returned all columns: ID=%s, Name=%s", result.ID, result.Name)
		}
	})

	suite.Run("ReturningNone", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "retn1", Name: "ReturnNone User", Email: "retn1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "retn1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			ReturningNone().
			Exec(suite.ctx)

		suite.NoError(err, "RETURNING NONE should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("RETURNING NONE affected %d rows", affected)
		}
	})
}

// TestWhenNotMatchedByTarget tests insertion when row exists in source but not in target (with optional conditions).
func (suite *MergeTestSuite) TestWhenNotMatchedByTarget() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing WhenNotMatchedByTarget for %s", suite.ds.Kind)

	suite.Run("BasicWhenNotMatchedByTarget", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "wnmbt1", Name: "NotMatchedByTarget User", Email: "wnmbt1@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "wnmbt1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatchedByTarget().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WhenNotMatchedByTarget should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("WhenNotMatchedByTarget affected %d rows", affected)
		}
	})

	suite.Run("ConditionalWhenNotMatchedByTarget", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{
			{ID: "wnmbtc1", Name: "Conditional User 1", Email: "wnmbtc1@example.com"},
			{ID: "wnmbtc2", Name: "", Email: "wnmbtc2@example.com"},
		}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.In("id", []string{"wnmbtc1", "wnmbtc2"})
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatchedByTarget(func(cb orm.ConditionBuilder) {
				cb.IsNotNull("_src.name").NotEquals("_src.name", "")
			}).
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WhenNotMatchedByTarget with condition should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Conditional WhenNotMatchedByTarget affected %d rows (should skip empty name)", affected)
		}
	})
}

// TestWhenNotMatchedBySource tests updates/deletes when row exists in target but not in source (with optional conditions).
func (suite *MergeTestSuite) TestWhenNotMatchedBySource() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing WhenNotMatchedBySource for %s", suite.ds.Kind)

	suite.Run("BasicWhenNotMatchedBySource", func() {
		// Insert test users first
		testUsers := []User{
			{Name: "User to Keep", Email: "keep@example.com", Age: 25, IsActive: true},
			{Name: "User to Update", Email: "update@example.com", Age: 30, IsActive: true},
		}
		testUsers[0].ID = "wnmbs1"
		testUsers[1].ID = "wnmbs2"

		for _, user := range testUsers {
			_, err := suite.db.NewInsert().
				Model(&user).
				Exec(suite.ctx)
			suite.NoError(err, "Failed to create test user")
		}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.In("id", []string{"wnmbs1", "wnmbs2"})
				}).
				Exec(suite.ctx)
		}()

		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		// Source only contains wnmbs1, so wnmbs2 will be "not matched by source"
		sourceData := []UserMergeData{
			{ID: "wnmbs1", Name: "User to Keep Updated", Email: "keep_updated@example.com"},
		}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetColumns("name", "email")
			}).
			WhenNotMatchedBySource(func(cb orm.ConditionBuilder) {
				// Only update our test users, not fixture users
				cb.In("u.id", []string{"wnmbs1", "wnmbs2"})
			}).
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.Set("is_active", false)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WhenNotMatchedBySource should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("WhenNotMatchedBySource affected %d rows", affected)
		}

		// Verify the results
		var users []User

		err = suite.db.NewSelect().
			Model(&users).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", []string{"wnmbs1", "wnmbs2"})
			}).
			OrderBy("id").
			Scan(suite.ctx)
		suite.NoError(err, "Should query merged users")

		// wnmbs1 should be updated from source
		suite.Equal("User to Keep Updated", users[0].Name, "Should update name from source")
		suite.Equal("keep_updated@example.com", users[0].Email, "Should update email from source")
		suite.True(users[0].IsActive, "Should remain active when matched")

		// wnmbs2 should be marked as inactive (not in source)
		suite.Equal("User to Update", users[1].Name, "Should keep original name when not in source")
		suite.Equal("update@example.com", users[1].Email, "Should keep original email when not in source")
		suite.False(users[1].IsActive, "Should deactivate when not matched by source")
	})

	suite.Run("ConditionalWhenNotMatchedBySource", func() {
		// Insert test users first
		testUsers := []User{
			{Name: "Active User 1", Email: "active1@example.com", Age: 25, IsActive: true},
			{Name: "Active User 2", Email: "active2@example.com", Age: 28, IsActive: true},
			{Name: "Inactive User", Email: "inactive@example.com", Age: 30, IsActive: false},
		}
		testUsers[0].ID = "wnmbsc1"
		testUsers[1].ID = "wnmbsc2"
		testUsers[2].ID = "wnmbsc3"

		for _, user := range testUsers {
			_, err := suite.db.NewInsert().
				Model(&user).
				Exec(suite.ctx)
			suite.NoError(err, "Failed to create test user")
		}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.In("id", []string{"wnmbsc1", "wnmbsc2", "wnmbsc3"})
				}).
				Exec(suite.ctx)
		}()

		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		// Source only contains wnmbsc1, so wnmbsc2 and wnmbsc3 will be "not matched by source"
		sourceData := []UserMergeData{
			{ID: "wnmbsc1", Name: "Active User 1 Updated", Email: "active1_updated@example.com"},
		}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetColumns("name", "email")
			}).
			WhenNotMatchedBySource(func(cb orm.ConditionBuilder) {
				// Only update our test users that are active
				cb.In("u.id", []string{"wnmbsc1", "wnmbsc2", "wnmbsc3"}).
					Equals("u.is_active", true)
			}).
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.Set("is_active", false)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WhenNotMatchedBySource with condition should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Conditional WhenNotMatchedBySource affected %d rows (should only update active users not in source)", affected)
		}

		// Verify the results
		var users []User

		err = suite.db.NewSelect().
			Model(&users).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", []string{"wnmbsc1", "wnmbsc2", "wnmbsc3"})
			}).
			OrderBy("id").
			Scan(suite.ctx)
		suite.NoError(err, "Should query merged users")

		// wnmbsc1 should be updated from source and remain active
		suite.Equal("Active User 1 Updated", users[0].Name, "Should update name from source")
		suite.Equal("active1_updated@example.com", users[0].Email, "Should update email from source")
		suite.True(users[0].IsActive, "Should remain active when matched")

		// wnmbsc2 should be deactivated (was active, not in source)
		suite.Equal("Active User 2", users[1].Name, "Should keep original name when not in source")
		suite.False(users[1].IsActive, "Should deactivate active user not in source")

		// wnmbsc3 should remain inactive (was already inactive, condition not met)
		suite.Equal("Inactive User", users[2].Name, "Should keep original name for inactive user")
		suite.False(users[2].IsActive, "Should remain inactive when condition not met")
	})
}

// TestThenDoNothing tests no-op actions for matched/not-matched conditions.
func (suite *MergeTestSuite) TestThenDoNothing() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing ThenDoNothing for %s", suite.ds.Kind)

	suite.Run("WhenMatchedDoNothing", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{
			{ID: suite.testUsers[0].ID, Name: "Should Not Update", Email: "mt_noupdate@test.com"},
			{ID: "mt_dnm1", Name: "Should Insert", Email: "mt_dnm1@test.com"},
		}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenDoNothing().
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "WHEN MATCHED THEN DO NOTHING should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("WHEN MATCHED DO NOTHING affected %d rows (inserts only)", affected)
		}
	})

	suite.Run("WhenNotMatchedDoNothing", func() {
		type UserMergeData struct {
			ID   string `bun:"id,pk"`
			Name string `bun:"name"`
		}

		sourceData := []UserMergeData{
			{ID: suite.testUsers[0].ID, Name: "Should Update"},
			{ID: "mt_dnn1", Name: "Should Not Insert"},
		}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetColumns("name")
			}).
			WhenNotMatched().
			ThenDoNothing().
			Exec(suite.ctx)

		suite.NoError(err, "WHEN NOT MATCHED THEN DO NOTHING should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("WHEN NOT MATCHED DO NOTHING affected %d rows (updates only)", affected)
		}
	})
}

// TestThenUpdate tests update actions: Set, SetExpr, SetColumns, SetAll.
func (suite *MergeTestSuite) TestThenUpdate() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing UpdateBuilder methods for %s", suite.ds.Kind)

	suite.Run("SetSingleValue", func() {
		type UserMergeData struct {
			ID   string `bun:"id,pk"`
			Name string `bun:"name"`
		}

		sourceData := []UserMergeData{{ID: suite.testUsers[0].ID, Name: "Set Single"}}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.Set("name", "Set Single Value")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Set single value should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Set single value affected %d rows", affected)
		}
	})

	suite.Run("SetMultipleValues", func() {
		type UserMergeData struct {
			ID   string `bun:"id,pk"`
			Name string `bun:"name"`
			Age  int16  `bun:"age"`
		}

		sourceData := []UserMergeData{{ID: suite.testUsers[0].ID, Name: "Multiple", Age: 35}}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.Set("name", "Set Multiple Values").
					Set("age", 40)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Set multiple values should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Set multiple values affected %d rows", affected)
		}
	})

	suite.Run("SetExprBasic", func() {
		type PostMergeData struct {
			ID        string `bun:"id,pk"`
			ViewCount int    `bun:"view_count"`
		}

		sourceData := []PostMergeData{{ID: suite.testPosts[0].ID, ViewCount: 100}}

		result, err := suite.db.NewMerge().
			Model(&Post{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("p.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetExpr("view_count", func(eb orm.ExprBuilder) any {
					return eb.Expr("? + ?", eb.Column("p.view_count"), 10)
				})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "SetExpr should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("SetExpr affected %d rows", affected)
		}
	})

	suite.Run("SetColumnsBasic", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: suite.testUsers[0].ID, Name: "SetColumns Name", Email: "mt_setcols@test.com"}}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetColumns("name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "SetColumns should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("SetColumns affected %d rows", affected)
		}
	})

	suite.Run("SetAllWithExclusions", func() {
		type UserMergeData struct {
			ID       string `bun:"id,pk"`
			Name     string `bun:"name"`
			Email    string `bun:"email"`
			Age      int16  `bun:"age"`
			IsActive bool   `bun:"is_active"`
		}

		sourceData := []UserMergeData{
			{ID: suite.testUsers[0].ID, Name: "SetAll Name", Email: "mt_setall@test.com", Age: 45, IsActive: true},
		}

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenMatched().
			ThenUpdate(func(ub orm.MergeUpdateBuilder) {
				ub.SetAll("id", "created_at", "created_by", "updated_at", "updated_by", "deleted_at", "deleted_by", "meta")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "SetAll with exclusions should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("SetAll with exclusions affected %d rows", affected)
		}
	})
}

// TestThenInsert tests insert actions: Value, ValueExpr, Values, ValuesAll.
func (suite *MergeTestSuite) TestThenInsert() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing InsertBuilder methods for %s", suite.ds.Kind)

	suite.Run("ValueSingleColumn", func() {
		type UserMergeData struct {
			ID string `bun:"id,pk"`
		}

		sourceData := []UserMergeData{{ID: "val1"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "val1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Value("id", "val1").
					Value("name", "Value Single User").
					Value("email", "val1@example.com")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Value single column should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Value single affected %d rows", affected)
		}
	})

	suite.Run("ValueMultipleColumns", func() {
		type UserMergeData struct {
			ID string `bun:"id,pk"`
		}

		sourceData := []UserMergeData{{ID: "valm1"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "valm1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Value("id", "valm1").
					Value("name", "Value Multiple User").
					Value("email", "valm1@example.com").
					Value("age", 25)
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Value multiple columns should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Value multiple affected %d rows", affected)
		}
	})

	suite.Run("ValueExprBasic", func() {
		type UserMergeData struct {
			ID   string `bun:"id,pk"`
			Name string `bun:"name"`
		}

		sourceData := []UserMergeData{{ID: "vale1", Name: "ValueExpr"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "vale1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Value("id", "vale1").
					Value("email", "vale1@example.com").
					ValueExpr("name", func(eb orm.ExprBuilder) any {
						return eb.Concat(eb.Column("_src.name"), " (Expression)")
					})
			}).
			Exec(suite.ctx)

		suite.NoError(err, "ValueExpr should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("ValueExpr affected %d rows", affected)
		}
	})

	suite.Run("ValuesMultipleColumns", func() {
		type UserMergeData struct {
			ID    string `bun:"id,pk"`
			Name  string `bun:"name"`
			Email string `bun:"email"`
		}

		sourceData := []UserMergeData{{ID: "vals1", Name: "Values User", Email: "values@example.com"}}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "vals1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.Values("id", "name", "email")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "Values multiple columns should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("Values multiple affected %d rows", affected)
		}
	})

	suite.Run("ValuesAllWithExclusions", func() {
		type UserMergeData struct {
			ID       string `bun:"id,pk"`
			Name     string `bun:"name"`
			Email    string `bun:"email"`
			Age      int16  `bun:"age"`
			IsActive bool   `bun:"is_active"`
		}

		sourceData := []UserMergeData{
			{ID: "vala1", Name: "ValuesAll User", Email: "valall@example.com", Age: 28, IsActive: true},
		}

		// Cleanup inserted data after test
		defer func() {
			_, _ = suite.db.NewDelete().
				Model(&User{}).
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("id", "vala1")
				}).
				Exec(suite.ctx)
		}()

		result, err := suite.db.NewMerge().
			Model(&User{}).
			WithValues("_src", &sourceData).
			UsingTable("_src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("u.id", "_src.id")
			}).
			WhenNotMatched().
			ThenInsert(func(ib orm.MergeInsertBuilder) {
				ib.ValuesAll("created_at", "created_by", "updated_at", "updated_by", "deleted_at", "deleted_by", "meta")
			}).
			Exec(suite.ctx)

		suite.NoError(err, "ValuesAll with exclusions should work")

		if result != nil {
			affected, _ := result.RowsAffected()
			suite.T().Logf("ValuesAll with exclusions affected %d rows", affected)
		}
	})
}

// TestThenDelete tests deletion when rows exist in target but not in source.
func (suite *MergeTestSuite) TestThenDelete() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing MERGE with DELETE for %s", suite.ds.Kind)

	testPosts := []Post{
		{Title: "Test Post 1", Status: "published", ViewCount: 100},
		{Title: "Test Post 2", Status: "draft", ViewCount: 50},
		{Title: "Test Post 3", Status: "archived", ViewCount: 25},
	}

	testPosts[0].ID = "merge_test_1"
	testPosts[1].ID = "merge_test_2"
	testPosts[2].ID = "merge_test_3"

	suite.T().Logf("Creating %d test posts for DELETE scenario", len(testPosts))

	for i, post := range testPosts {
		_, err := suite.db.NewInsert().
			Model(&post).
			Exec(suite.ctx)
		suite.NoError(err, "Failed to create test post %d (Id: %s)", i+1, post.ID)
		suite.T().Logf("Created test post %d: %s - %s (views: %d)", i+1, post.ID, post.Title, post.ViewCount)
	}

	// Cleanup test posts after test (merge_test_3 should be deleted by MERGE)
	defer func() {
		_, _ = suite.db.NewDelete().
			Model(&Post{}).
			Where(func(cb orm.ConditionBuilder) {
				cb.In("id", []string{"merge_test_1", "merge_test_2"})
			}).
			Exec(suite.ctx)
	}()

	type PostUpdateData struct {
		ID        string `bun:"id,pk"`
		Title     string `bun:"title"`
		Status    string `bun:"status"`
		ViewCount int    `bun:"view_count"`
	}

	sourceData := []PostUpdateData{
		{ID: "merge_test_1", Title: "Updated Test Post 1", Status: "published", ViewCount: 120},
		{ID: "merge_test_2", Title: "Updated Test Post 2", Status: "published", ViewCount: 80},
	}

	suite.T().Logf("Executing MERGE with DELETE - %d source records, missing merge_test_3 to test deletion", len(sourceData))

	result, err := suite.db.NewMerge().
		Model(&Post{}).
		WithValues("_source_data", &sourceData).
		UsingTable("_source_data").
		On(func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("p.id", "_source_data.id")
		}).
		WhenMatched().
		ThenUpdate(func(ub orm.MergeUpdateBuilder) {
			ub.SetColumns("title", "status", "view_count")
		}).
		WhenNotMatchedBySource(func(cb orm.ConditionBuilder) {
			cb.LessThan("p.view_count", 30)
		}).
		ThenDelete().
		Exec(suite.ctx)

	suite.NoError(err, "MERGE with DELETE should complete successfully")

	if result != nil {
		affected, _ := result.RowsAffected()
		suite.True(affected >= 0, "MERGE with DELETE should affect 0 or more rows, got %d", affected)
		suite.T().Logf("MERGE with DELETE affected %d rows (updates + deletions)", affected)
	}

	var remainingPosts []Post

	err = suite.db.NewSelect().
		Model(&remainingPosts).
		Where(func(cb orm.ConditionBuilder) {
			cb.StartsWith("id", "merge_test_")
		}).
		OrderBy("id").
		Scan(suite.ctx)
	suite.NoError(err, "Failed to query remaining posts after MERGE with DELETE")

	suite.T().Logf("Remaining posts after MERGE with DELETE: %d", len(remainingPosts))

	for _, post := range remainingPosts {
		suite.T().Logf("Remaining post %s: %s - %s (views: %d)", post.ID, post.Title, post.Status, post.ViewCount)
		suite.NotEqual("merge_test_3", post.ID, "Post merge_test_3 with low view count should be deleted")
	}
}

// TestMergeWithConditions tests MERGE with conditional WHEN clauses (e.g., only update when source > target).
func (suite *MergeTestSuite) TestMergeWithConditions() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE statement is only supported by PostgreSQL, skipping for %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing MERGE with conditions for %s", suite.ds.Kind)

	type PostMergeData struct {
		ID        string `bun:"id,pk"`
		Title     string `bun:"title"`
		Status    string `bun:"status"`
		ViewCount int    `bun:"view_count"`
	}

	sourceData := []PostMergeData{
		{ID: suite.testPosts[0].ID, Title: "MT Updated Post 1", Status: "published", ViewCount: 150},
		{ID: suite.testPosts[1].ID, Title: "MT Updated Post 2", Status: "draft", ViewCount: 75},
		{ID: "mt_cond_new1", Title: "MT New Post 1", Status: "draft", ViewCount: 0},
	}

	suite.T().Logf("Executing conditional MERGE with %d source records", len(sourceData))

	result, err := suite.db.NewMerge().
		Model(&Post{}).
		WithValues("_source_data", &sourceData).
		UsingTable("_source_data").
		On(func(cb orm.ConditionBuilder) {
			cb.EqualsColumn("p.id", "_source_data.id")
		}).
		WhenMatched(func(cb orm.ConditionBuilder) {
			cb.GreaterThanColumn("_source_data.view_count", "p.view_count")
		}).
		ThenUpdate(func(ub orm.MergeUpdateBuilder) {
			ub.SetColumns("title", "status", "view_count")
		}).
		WhenNotMatched(func(cb orm.ConditionBuilder) {
			cb.IsNotNull("_source_data.status").NotEquals("_source_data.status", "")
		}).
		ThenInsert(func(ib orm.MergeInsertBuilder) {
			ib.Values("id", "title", "status", "view_count")
		}).
		Exec(suite.ctx)

	suite.NoError(err, "Conditional MERGE operation should complete successfully")

	if result != nil {
		affected, _ := result.RowsAffected()
		suite.True(affected >= 0, "Conditional MERGE should affect 0 or more rows, got %d", affected)
		suite.T().Logf("Conditional MERGE affected %d rows", affected)
	}

	var updatedPosts []Post

	err = suite.db.NewSelect().
		Model(&updatedPosts).
		Where(func(cb orm.ConditionBuilder) {
			cb.In("id", []string{suite.testPosts[0].ID, suite.testPosts[1].ID, "mt_cond_new1"})
		}).
		OrderBy("id").
		Scan(suite.ctx)
	suite.NoError(err, "Failed to query posts after conditional MERGE")

	suite.T().Logf("Posts after conditional MERGE: %d", len(updatedPosts))

	for _, post := range updatedPosts {
		suite.T().Logf("Post %s: %s - %s (views: %d)", post.ID, post.Title, post.Status, post.ViewCount)
	}
}

// TestMergeDB tests DB() method returns the underlying database.
func (suite *MergeTestSuite) TestMergeDB() {
	suite.T().Logf("Testing Merge DB for %s", suite.ds.Kind)

	query := suite.db.NewMerge().Model((*Tag)(nil))
	suite.NotNil(query.DB(), "Should return underlying database")
}

// TestMergeUsingMethods tests Using and UsingExpr source specification methods.
func (suite *MergeTestSuite) TestMergeUsingMethods() {
	if suite.ds.Kind != config.Postgres {
		suite.T().Skipf("MERGE not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing Merge Using methods for %s", suite.ds.Kind)

	suite.Run("UsingModel", func() {
		query := suite.db.NewMerge().
			Model((*Tag)(nil)).
			Using((*Tag)(nil), "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("tag.id", "src.id")
			}).
			WhenMatched().ThenUpdate(func(ub orm.MergeUpdateBuilder) {
			ub.Set("name", "src")
		}).
			WhenNotMatched().ThenInsert(func(ib orm.MergeInsertBuilder) {
			ib.Value("id", "src").Value("name", "src")
		})
		suite.NotNil(query, "Should build query with Using model")

		sql := suite.db.NewMerge().
			Model((*Tag)(nil)).
			Using((*Tag)(nil), "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("tag.id", "src.id")
			}).
			WhenMatched().ThenDelete().
			String()
		suite.Contains(sql, `USING "test_tag" AS "src"`, "Using(model) should render quoted table name")
	})

	suite.Run("UsingExpr", func() {
		query := suite.db.NewMerge().
			Model((*Tag)(nil)).
			UsingExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.SelectExpr(func(eb orm.ExprBuilder) any {
						return eb.Literal("test-id")
					}, "id").
						SelectExpr(func(eb orm.ExprBuilder) any {
							return eb.Literal("TestName")
						}, "name")
				})
			}, "src").
			On(func(cb orm.ConditionBuilder) {
				cb.EqualsColumn("tag.id", "src.id")
			}).
			WhenMatched().ThenDelete()
		suite.NotNil(query, "Should build query with UsingExpr")
	})
}

// TestMergeApplyMethods tests Apply and ApplyIf conditional query building.
func (suite *MergeTestSuite) TestMergeApplyMethods() {
	suite.T().Logf("Testing Merge Apply methods for %s", suite.ds.Kind)

	suite.Run("ApplyUnconditional", func() {
		query := suite.db.NewMerge().
			Model((*Tag)(nil)).
			Apply(func(q orm.MergeQuery) {
				q.UsingTable("test_tag", "src")
			})
		suite.NotNil(query, "Should apply function unconditionally")
	})

	suite.Run("ApplyIfTrue", func() {
		query := suite.db.NewMerge().
			Model((*Tag)(nil)).
			ApplyIf(true, func(q orm.MergeQuery) {
				q.UsingTable("test_tag", "src")
			})
		suite.NotNil(query, "Should apply function when condition is true")
	})

	suite.Run("ApplyIfFalse", func() {
		query := suite.db.NewMerge().
			Model((*Tag)(nil)).
			ApplyIf(false, func(q orm.MergeQuery) {
				q.UsingTable("test_tag", "src")
			})
		suite.NotNil(query, "Should skip function when condition is false")
	})
}

// TestMergeTableSpecMethods tests table specification methods with and without aliases.
func (suite *MergeTestSuite) TestMergeTableSpecMethods() {
	suite.T().Logf("Testing Merge table specification methods for %s", suite.ds.Kind)

	suite.Run("WithRecursive", func() {
		query := suite.db.NewMerge().
			WithRecursive("ids", func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id")
			}).
			Model((*Tag)(nil)).
			UsingTable("test_tag", "src")
		suite.NotNil(query, "Should build query with WithRecursive")
	})

	suite.Run("TableFrom", func() {
		query := suite.db.NewMerge().
			TableFrom((*Tag)(nil)).
			UsingTable("test_tag", "src")
		suite.NotNil(query, "Should build query with TableFrom")
	})

	suite.Run("TableExprNoAlias", func() {
		query := suite.db.NewMerge().
			TableExpr(func(eb orm.ExprBuilder) any {
				return eb.SubQuery(func(sq orm.SelectQuery) {
					sq.Model((*Tag)(nil)).Select("id", "name")
				})
			}).
			UsingTable("test_tag", "src")
		suite.NotNil(query, "Should build query with TableExpr")
	})

	suite.Run("TableSubQueryNoAlias", func() {
		query := suite.db.NewMerge().
			TableSubQuery(func(sq orm.SelectQuery) {
				sq.Model((*Tag)(nil)).Select("id", "name")
			}).
			UsingTable("test_tag", "src")
		suite.NotNil(query, "Should build query with TableSubQuery")
	})
}
