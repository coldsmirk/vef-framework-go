package orm_test

import (
	"context"
	"database/sql"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
)

// UserFavorite represents a user's favorite post with composite primary key (user_id, post_id).
type UserFavorite struct {
	bun.BaseModel `bun:"table:test_user_favorite,alias:uf"`

	UserID    string    `json:"userId"    bun:"user_id,pk,notnull"`
	PostID    string    `json:"postId"    bun:"post_id,pk,notnull"`
	CreatedAt time.Time `json:"createdAt" bun:"created_at,notnull,default:CURRENT_TIMESTAMP"`

	// Relations
	User *User `json:"user" bun:"rel:belongs-to,join:user_id=id"`
	Post *Post `json:"post" bun:"rel:belongs-to,join:post_id=id"`
}

// User represents a user in the system.
type User struct {
	bun.BaseModel `bun:"table:test_user,alias:u"`
	orm.Model

	Name     string         `json:"name"     bun:"name,notnull"`
	Email    string         `json:"email"    bun:"email,notnull,unique"`
	Age      int16          `json:"age"      bun:"age,notnull,default:0"`
	IsActive bool           `json:"isActive" bun:"is_active,notnull"`
	Meta     map[string]any `json:"meta"     bun:"meta"`

	// Relations
	Posts []Post `json:"posts" bun:"rel:has-many,join:id=user_id"`
}

// Post represents a blog post or article.
type Post struct {
	bun.BaseModel `bun:"table:test_post,alias:p"`
	orm.Model

	Title       string  `json:"title"       bun:"title,notnull"`
	Content     string  `json:"content"     bun:"content,notnull"`
	Description *string `json:"description" bun:"description"`
	UserID      string  `json:"userId"      bun:"user_id,notnull"`
	CategoryID  string  `json:"categoryId"  bun:"category_id,notnull"`
	Status      string  `json:"status"      bun:"status,notnull,default:'draft'"`
	ViewCount   int     `json:"viewCount"   bun:"view_count,notnull,default:0"`

	// Relations
	User     *User     `json:"user"     bun:"rel:belongs-to,join:user_id=id"`
	Category *Category `json:"category" bun:"rel:belongs-to,join:category_id=id"`
}

// Tag represents a content tag (uses IDModel only, no audit fields).
type Tag struct {
	bun.BaseModel `bun:"table:test_tag,alias:t"`
	orm.IDModel

	Name        string  `json:"name"        bun:"name,notnull,unique"`
	Description *string `json:"description" bun:"description"`
}

// PostTag represents the many-to-many relationship between posts and tags (uses IDModel only).
type PostTag struct {
	bun.BaseModel `bun:"table:test_post_tag,alias:pt"`
	orm.IDModel

	PostID string `json:"postId" bun:"post_id,notnull"`
	TagID  string `json:"tagId"  bun:"tag_id,notnull"`

	// Relations
	Post *Post `json:"post" bun:"rel:belongs-to,join:post_id=id"`
	Tag  *Tag  `json:"tag"  bun:"rel:belongs-to,join:tag_id=id"`
}

// Category represents a content category (uses IDModel + CreatedModel, no update audit).
type Category struct {
	bun.BaseModel `bun:"table:test_category,alias:c"`
	orm.IDModel
	orm.CreatedModel

	Name        string  `json:"name"        bun:"name,notnull,unique"`
	Description *string `json:"description" bun:"description"`
	ParentID    *string `json:"parentId"    bun:"parent_id"`

	// Relations
	Posts    []Post     `json:"posts"    bun:"rel:has-many,join:id=category_id"`
	Parent   *Category  `json:"parent"   bun:"rel:belongs-to,join:parent_id=id"`
	Children []Category `json:"children" bun:"rel:has-many,join:id=parent_id"`
}

// Comment represents a user comment on a post with tree structure for replies.
type Comment struct {
	bun.BaseModel `bun:"table:test_comment,alias:cm"`
	orm.Model

	Content  string  `json:"content"  bun:"content,notnull"`
	PostID   string  `json:"postId"   bun:"post_id,notnull"`
	UserID   string  `json:"userId"   bun:"user_id,notnull"`
	ParentID *string `json:"parentId" bun:"parent_id"`
	Likes    int     `json:"likes"    bun:"likes,notnull,default:0"`

	// Relations
	Post     *Post     `json:"post"     bun:"rel:belongs-to,join:post_id=id"`
	User     *User     `json:"user"     bun:"rel:belongs-to,join:user_id=id"`
	Parent   *Comment  `json:"parent"   bun:"rel:belongs-to,join:parent_id=id"`
	Children []Comment `json:"children" bun:"rel:has-many,join:id=parent_id"`
}

// fixtureEndDate is the upper bound for fixture data timestamps.
// All fixture data has created_at in 2025; test-inserted data will have 2026+ timestamps.
var fixtureEndDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// fixtureScope limits query to fixture data only (created_at < 2026).
func fixtureScope(cb orm.ConditionBuilder) {
	cb.LessThan("created_at", fixtureEndDate)
}

// BaseTestSuite contains all the actual test methods and works with orm.DB interface.
// This suite will be run against multiple databases to verify cross-database compatibility.
type BaseTestSuite struct {
	suite.Suite

	ctx   context.Context
	db    orm.DB
	rawDB *sql.DB
	ds    *config.DataSourceConfig
}

// selectUsers returns a User select query scoped to fixture data.
func (suite *BaseTestSuite) selectUsers() orm.SelectQuery {
	return suite.db.NewSelect().Model((*User)(nil)).Where(fixtureScope)
}

// selectPosts returns a Post select query scoped to fixture data.
func (suite *BaseTestSuite) selectPosts() orm.SelectQuery {
	return suite.db.NewSelect().Model((*Post)(nil)).Where(fixtureScope)
}

// selectCategories returns a Category select query scoped to fixture data.
func (suite *BaseTestSuite) selectCategories() orm.SelectQuery {
	return suite.db.NewSelect().Model((*Category)(nil)).Where(fixtureScope)
}

// selectUserFavorites returns a UserFavorite select query scoped to fixture data.
func (suite *BaseTestSuite) selectUserFavorites() orm.SelectQuery {
	return suite.db.NewSelect().Model((*UserFavorite)(nil)).Where(fixtureScope)
}
