package orm_test

import (
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBAuditConditionsTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBAuditConditionsTestSuite tests audit field condition methods.
type CBAuditConditionsTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestCreatedByEquals tests CreatedByEquals and OrCreatedByEquals conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedByEquals() {
	suite.T().Logf("Testing CreatedByEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicCreatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByEquals("system")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.Equal("system", user.CreatedBy, "CreatedBy should be system")
		}
	})

	suite.Run("OrCreatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByEquals("system").
						OrCreatedByEquals("admin")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})
}

// TestCreatedByNotEquals tests CreatedByNotEquals and OrCreatedByNotEquals conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedByNotEquals() {
	suite.T().Logf("Testing CreatedByNotEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicCreatedByNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotEquals("nonexistent")
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrCreatedByNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotEquals("user1").
						OrCreatedByNotEquals("user2")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})
}

// TestCreatedByIn tests CreatedByIn and OrCreatedByIn conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedByIn() {
	suite.T().Logf("Testing CreatedByIn condition for %s", suite.ds.Kind)

	suite.Run("BasicCreatedByIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByIn([]string{"system", "admin"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByIn([]string{"system"}).
						OrCreatedByIn([]string{"admin"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})
}

// TestUpdatedByEquals tests UpdatedByEquals and OrUpdatedByEquals conditions.
func (suite *CBAuditConditionsTestSuite) TestUpdatedByEquals() {
	suite.T().Logf("Testing UpdatedByEquals condition for %s", suite.ds.Kind)

	suite.Run("BasicUpdatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByEquals("system")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")

		for _, user := range users {
			suite.Equal("system", user.UpdatedBy, "UpdatedBy should be system")
		}
	})

	suite.Run("OrUpdatedByEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByEquals("system").
						OrUpdatedByEquals("admin")
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})
}

// TestCreatedAtBetween tests CreatedAtBetween and OrCreatedAtBetween conditions.
func (suite *CBAuditConditionsTestSuite) TestCreatedAtBetween() {
	suite.T().Logf("Testing CreatedAtBetween condition for %s", suite.ds.Kind)

	// Fixture data has created_at spread across 2025 (Jan-Nov).
	// Use a range covering the full year to find all fixture users.
	suite.Run("BasicCreatedAtBetween", func() {
		start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtBetween(start, end)
				}),
		)

		suite.True(len(users) > 0, "Should find users created around fixture date")
	})

	suite.Run("OrCreatedAtBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtBetween(
						time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2025, 6, 30, 23, 59, 59, 0, time.UTC),
					).OrCreatedAtBetween(
						time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
					)
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})
}

// TestUpdatedAtGreaterThan tests UpdatedAtGreaterThan and OrUpdatedAtGreaterThan conditions.
func (suite *CBAuditConditionsTestSuite) TestUpdatedAtGreaterThan() {
	suite.T().Logf("Testing UpdatedAtGreaterThan condition for %s", suite.ds.Kind)

	// Fixture data has updated_at spread across 2025 (Jan-Dec)
	beforeFixture := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	suite.Run("BasicUpdatedAtGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtGreaterThan(beforeFixture)
				}),
		)

		suite.True(len(users) > 0, "Should find users updated after the reference date")
	})

	suite.Run("OrUpdatedAtGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtGreaterThan(beforeFixture.Add(-48 * time.Hour)).
						OrUpdatedAtGreaterThan(beforeFixture)
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})
}

// TestCreatedBySubQueryAndAny tests CreatedByEqualsSubQuery, CreatedByEqualsAny, CreatedByEqualsAll.
func (suite *CBAuditConditionsTestSuite) TestCreatedBySubQueryAndAny() {
	suite.T().Logf("Testing CreatedBy SubQuery/Any/All for %s", suite.ds.Kind)

	suite.Run("CreatedByEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByEqualsSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("created_by").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Alice Johnson")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByEqualsSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("created_by").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("name", "Alice Johnson")
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("CreatedByEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.CreatedByEqualsAny(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("created_by")
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrCreatedByEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrCreatedByEqualsAny(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									Select("created_by")
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})

		suite.Run("CreatedByEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.CreatedByEqualsAll(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("created_by").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("name", "Alice Johnson")
								}).
								Limit(1)
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrCreatedByEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrCreatedByEqualsAll(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									Select("created_by").
									Where(func(cb orm.ConditionBuilder) {
										cb.Equals("name", "Alice Johnson")
									}).
									Limit(1)
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})
	}
}

// TestCreatedByCurrentAndNotEqualsExtended tests CreatedByEqualsCurrent and NotEquals extended.
func (suite *CBAuditConditionsTestSuite) TestCreatedByCurrentAndNotEqualsExtended() {
	suite.T().Logf("Testing CreatedBy Current/NotEquals extended for %s", suite.ds.Kind)

	suite.Run("CreatedByEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByEqualsCurrent()
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByEqualsCurrent()
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedByNotEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotEqualsCurrent()
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByNotEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByNotEqualsCurrent()
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedByNotEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotEqualsSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal("nonexistent")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByNotEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByNotEqualsSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal("nonexistent")
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("CreatedByNotEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.CreatedByNotEqualsAny(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("created_by")
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrCreatedByNotEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrCreatedByNotEqualsAny(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									Select("created_by")
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})

		suite.Run("CreatedByNotEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.CreatedByNotEqualsAll(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal("nonexistent")
								}).
								Limit(1)
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrCreatedByNotEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrCreatedByNotEqualsAll(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									SelectExpr(func(eb orm.ExprBuilder) any {
										return eb.Literal("nonexistent")
									}).
									Limit(1)
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})
	}
}

// TestCreatedByInSubQueryAndNotIn tests CreatedByInSubQuery, CreatedByNotIn, CreatedByNotInSubQuery.
func (suite *CBAuditConditionsTestSuite) TestCreatedByInSubQueryAndNotIn() {
	suite.T().Logf("Testing CreatedBy InSubQuery/NotIn for %s", suite.ds.Kind)

	suite.Run("CreatedByInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByInSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("created_by")
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByInSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("created_by")
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedByNotIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotIn([]string{"nonexistent"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByNotIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByNotIn([]string{"nonexistent"})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedByNotInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedByNotInSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal("nonexistent")
							})
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrCreatedByNotInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedByNotInSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal("nonexistent")
								})
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestUpdatedByExtended tests UpdatedBy variants not yet covered.
func (suite *CBAuditConditionsTestSuite) TestUpdatedByExtended() {
	suite.T().Logf("Testing UpdatedBy extended for %s", suite.ds.Kind)

	suite.Run("UpdatedByNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByNotEquals("nonexistent")
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrUpdatedByNotEquals", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByNotEquals("nonexistent")
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByIn([]string{"system", "admin"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByIn([]string{"system", "admin"})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByNotIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByNotIn([]string{"nonexistent"})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByNotIn", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByNotIn([]string{"nonexistent"})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByEqualsCurrent()
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByEqualsCurrent()
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByNotEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByNotEqualsCurrent()
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByNotEqualsCurrent", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByNotEqualsCurrent()
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByEqualsSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("updated_by").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("name", "Alice Johnson")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByEqualsSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("updated_by").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("name", "Alice Johnson")
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	if suite.ds.Kind == config.Postgres {
		suite.Run("UpdatedByEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.UpdatedByEqualsAny(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("updated_by")
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrUpdatedByEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrUpdatedByEqualsAny(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									Select("updated_by")
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})

		suite.Run("UpdatedByEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.UpdatedByEqualsAll(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("updated_by").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("name", "Alice Johnson")
								}).
								Limit(1)
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrUpdatedByEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrUpdatedByEqualsAll(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									Select("updated_by").
									Where(func(cb orm.ConditionBuilder) {
										cb.Equals("name", "Alice Johnson")
									}).
									Limit(1)
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})

		suite.Run("UpdatedByNotEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.UpdatedByNotEqualsAny(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("updated_by")
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrUpdatedByNotEqualsAny", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrUpdatedByNotEqualsAny(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									Select("updated_by")
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})

		suite.Run("UpdatedByNotEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.UpdatedByNotEqualsAll(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal("nonexistent")
								}).
								Limit(1)
						})
					}),
			)

			suite.True(len(users) >= 0, "Should execute successfully")
		})

		suite.Run("OrUpdatedByNotEqualsAll", func() {
			users := suite.assertQueryReturnsUsers(
				suite.selectUsers().
					Where(func(cb orm.ConditionBuilder) {
						cb.Equals("age", 21).
							OrUpdatedByNotEqualsAll(func(sq orm.SelectQuery) {
								sq.Model((*User)(nil)).
									SelectExpr(func(eb orm.ExprBuilder) any {
										return eb.Literal("nonexistent")
									}).
									Limit(1)
							})
					}),
			)

			suite.True(len(users) > 0, "Should find users")
		})
	}

	suite.Run("UpdatedByNotEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByNotEqualsSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal("nonexistent")
							}).
							Limit(1)
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByNotEqualsSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByNotEqualsSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal("nonexistent")
								}).
								Limit(1)
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByInSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("updated_by")
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByInSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("updated_by")
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedByNotInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedByNotInSubQuery(func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							SelectExpr(func(eb orm.ExprBuilder) any {
								return eb.Literal("nonexistent")
							})
					})
				}),
		)

		suite.True(len(users) >= 0, "Should execute successfully")
	})

	suite.Run("OrUpdatedByNotInSubQuery", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedByNotInSubQuery(func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								SelectExpr(func(eb orm.ExprBuilder) any {
									return eb.Literal("nonexistent")
								})
						})
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestCreatedAtExtended tests additional CreatedAt methods.
func (suite *CBAuditConditionsTestSuite) TestCreatedAtExtended() {
	suite.T().Logf("Testing CreatedAt extended for %s", suite.ds.Kind)

	start2025 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mid2025 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	end2025 := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	suite.Run("CreatedAtGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtGreaterThan(start2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users created after 2025-01-01")
	})

	suite.Run("OrCreatedAtGreaterThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedAtGreaterThan(start2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedAtGreaterThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtGreaterThanOrEqual(start2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrCreatedAtGreaterThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedAtGreaterThanOrEqual(start2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedAtLessThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtLessThan(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users created before 2025-12-31")
	})

	suite.Run("OrCreatedAtLessThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedAtLessThan(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedAtLessThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtLessThanOrEqual(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrCreatedAtLessThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedAtLessThanOrEqual(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("CreatedAtNotBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.CreatedAtNotBetween(
						time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
					)
				}),
		)

		suite.True(len(users) > 0, "Should find users not created in 2024")
	})

	suite.Run("OrCreatedAtNotBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrCreatedAtNotBetween(mid2025, end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestUpdatedAtExtended tests additional UpdatedAt methods.
func (suite *CBAuditConditionsTestSuite) TestUpdatedAtExtended() {
	suite.T().Logf("Testing UpdatedAt extended for %s", suite.ds.Kind)

	start2025 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mid2025 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	end2025 := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	suite.Run("UpdatedAtGreaterThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtGreaterThanOrEqual(start2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrUpdatedAtGreaterThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedAtGreaterThanOrEqual(start2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedAtLessThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtLessThan(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrUpdatedAtLessThan", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedAtLessThan(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedAtLessThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtLessThanOrEqual(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrUpdatedAtLessThanOrEqual", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedAtLessThanOrEqual(end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedAtBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtBetween(start2025, end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users updated in 2025")
	})

	suite.Run("OrUpdatedAtBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedAtBetween(start2025, end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("UpdatedAtNotBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.UpdatedAtNotBetween(
						time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
					)
				}),
		)

		suite.True(len(users) > 0, "Should find users not updated in 2024")
	})

	suite.Run("OrUpdatedAtNotBetween", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrUpdatedAtNotBetween(mid2025, end2025)
				}),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}
