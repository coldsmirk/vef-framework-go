package orm_test

import (
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

func init() {
	registry.Add(func(base *BaseTestSuite) suite.TestingSuite {
		return &CBAnyAllTestSuite{
			ConditionBuilderTestSuite: &ConditionBuilderTestSuite{BaseTestSuite: base},
		}
	})
}

// CBAnyAllTestSuite tests ANY and ALL subquery condition methods.
type CBAnyAllTestSuite struct {
	*ConditionBuilderTestSuite
}

// TestEqualsAny tests EqualsAny and OrEqualsAny conditions.
func (suite *CBAnyAllTestSuite) TestEqualsAny() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ANY/ALL not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing EqualsAny condition for %s", suite.ds.Kind)

	suite.Run("BasicEqualsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.EqualsAny("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{25, 30})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")

		for _, user := range users {
			suite.True(user.Age == 25 || user.Age == 30, "Age should be 25 or 30")
		}
	})

	suite.Run("OrEqualsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrEqualsAny("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 45)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestNotEqualsAny tests NotEqualsAny and OrNotEqualsAny conditions.
func (suite *CBAnyAllTestSuite) TestNotEqualsAny() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ANY/ALL not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing NotEqualsAny condition for %s", suite.ds.Kind)

	suite.Run("BasicNotEqualsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.NotEqualsAny("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 25)
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrNotEqualsAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrNotEqualsAny("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 30)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestGreaterThanAnyAll tests GreaterThanAny, GreaterThanAll and Or variants.
func (suite *CBAnyAllTestSuite) TestGreaterThanAnyAll() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ANY/ALL not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing GreaterThanAny/All for %s", suite.ds.Kind)

	suite.Run("GreaterThanAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanAny("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{25, 30})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users with age > ANY(25,30)")
	})

	suite.Run("OrGreaterThanAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrGreaterThanAny("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 40)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("GreaterThanAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{25, 30})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users with age > ALL(25,30)")

		for _, user := range users {
			suite.True(user.Age > 30, "Age should be > 30")
		}
	})

	suite.Run("OrGreaterThanAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrGreaterThanAll("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 40)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestGreaterThanOrEqualAnyAll tests GTE Any/All and Or variants.
func (suite *CBAnyAllTestSuite) TestGreaterThanOrEqualAnyAll() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ANY/ALL not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing GreaterThanOrEqualAny/All for %s", suite.ds.Kind)

	suite.Run("GreaterThanOrEqualAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqualAny("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 30)
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrGreaterThanOrEqualAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrGreaterThanOrEqualAny("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 40)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("GreaterThanOrEqualAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.GreaterThanOrEqualAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{25, 30})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrGreaterThanOrEqualAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 21).
						OrGreaterThanOrEqualAll("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 40)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestLessThanAnyAll tests LT Any/All and Or variants.
func (suite *CBAnyAllTestSuite) TestLessThanAnyAll() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ANY/ALL not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing LessThanAny/All for %s", suite.ds.Kind)

	suite.Run("LessThanAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanAny("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{30, 40})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users with age < ANY(30,40)")
	})

	suite.Run("OrLessThanAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 45).
						OrLessThanAny("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 25)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("LessThanAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{30, 40})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users with age < ALL(30,40)")

		for _, user := range users {
			suite.True(user.Age < 30, "Age should be < 30")
		}
	})

	suite.Run("OrLessThanAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 45).
						OrLessThanAll("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 25)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}

// TestLessThanOrEqualAnyAll tests LTE Any/All and Or variants.
func (suite *CBAnyAllTestSuite) TestLessThanOrEqualAnyAll() {
	if suite.ds.Kind == config.SQLite {
		suite.T().Skipf("ANY/ALL not supported on %s", suite.ds.Kind)
	}

	suite.T().Logf("Testing LessThanOrEqualAny/All for %s", suite.ds.Kind)

	suite.Run("LessThanOrEqualAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualAny("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.Equals("age", 30)
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrLessThanOrEqualAny", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 45).
						OrLessThanOrEqualAny("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 25)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("LessThanOrEqualAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.LessThanOrEqualAll("age", func(sq orm.SelectQuery) {
						sq.Model((*User)(nil)).
							Select("age").
							Where(func(cb orm.ConditionBuilder) {
								cb.In("age", []int16{30, 40})
							})
					})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})

	suite.Run("OrLessThanOrEqualAll", func() {
		users := suite.assertQueryReturnsUsers(
			suite.selectUsers().
				Where(func(cb orm.ConditionBuilder) {
					cb.Equals("age", 45).
						OrLessThanOrEqualAll("age", func(sq orm.SelectQuery) {
							sq.Model((*User)(nil)).
								Select("age").
								Where(func(cb orm.ConditionBuilder) {
									cb.Equals("age", 25)
								})
						})
				}).
				OrderBy("age"),
		)

		suite.True(len(users) > 0, "Should find users")
	})
}
