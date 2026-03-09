package tree

import (
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestNode struct {
	ID       string     `json:"id"`
	ParentID string     `json:"parentId"`
	Name     string     `json:"name"`
	Children []TestNode `json:"children"`
}

type TestCategory struct {
	CategoryID    string         `json:"categoryId"`
	ParentCatID   string         `json:"parentCatId"`
	CategoryName  string         `json:"categoryName"`
	SubCategories []TestCategory `json:"subCategories"`
	Level         int            `json:"level"`
}

func createTestNodeAdapter() Adapter[TestNode] {
	return Adapter[TestNode]{
		GetID:       func(node TestNode) string { return node.ID },
		GetParentID: func(node TestNode) *string { return lo.EmptyableToPtr(node.ParentID) },
		GetChildren: func(node TestNode) []TestNode { return node.Children },
		SetChildren: func(node *TestNode, children []TestNode) { node.Children = children },
	}
}

func createTestCategoryAdapter() Adapter[TestCategory] {
	return Adapter[TestCategory]{
		GetID:       func(cat TestCategory) string { return cat.CategoryID },
		GetParentID: func(cat TestCategory) *string { return lo.EmptyableToPtr(cat.ParentCatID) },
		GetChildren: func(cat TestCategory) []TestCategory { return cat.SubCategories },
		SetChildren: func(cat *TestCategory, children []TestCategory) { cat.SubCategories = children },
	}
}

func createTestNodes() []TestNode {
	return []TestNode{
		{ID: "1", ParentID: "", Name: "Root 1"},
		{ID: "2", ParentID: "1", Name: "Child 1-1"},
		{ID: "3", ParentID: "1", Name: "Child 1-2"},
		{ID: "4", ParentID: "2", Name: "Child 1-1-1"},
		{ID: "5", ParentID: "2", Name: "Child 1-1-2"},
		{ID: "6", ParentID: "", Name: "Root 2"},
		{ID: "7", ParentID: "6", Name: "Child 2-1"},
		{ID: "8", ParentID: "nonexistent", Name: "Orphan"},
	}
}

func createComplexTestNodes() []TestNode {
	return []TestNode{
		{ID: "root1", ParentID: "", Name: "Root 1"},
		{ID: "root2", ParentID: "", Name: "Root 2"},
		{ID: "a", ParentID: "root1", Name: "A"},
		{ID: "b", ParentID: "root1", Name: "B"},
		{ID: "c", ParentID: "a", Name: "C"},
		{ID: "d", ParentID: "a", Name: "D"},
		{ID: "e", ParentID: "b", Name: "E"},
		{ID: "f", ParentID: "c", Name: "F"},
		{ID: "g", ParentID: "c", Name: "G"},
		{ID: "h", ParentID: "root2", Name: "H"},
		{ID: "i", ParentID: "h", Name: "I"},
	}
}

func findNodeByID(nodes []TestNode, id string) *TestNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}

	return nil
}

func findCategoryByID(categories []TestCategory, id string) *TestCategory {
	for i := range categories {
		if categories[i].CategoryID == id {
			return &categories[i]
		}
	}

	return nil
}

// TestBuild tests build functionality.
func TestBuild(t *testing.T) {
	adapter := createTestNodeAdapter()

	t.Run("BuildsSimpleTreeStructure", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "1", ParentID: "", Name: "Root"},
			{ID: "2", ParentID: "1", Name: "Child 1"},
			{ID: "3", ParentID: "1", Name: "Child 2"},
		}

		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")
		root := result[0]
		assert.Equal(t, "1", root.ID, "Should equal expected value")
		assert.Equal(t, "Root", root.Name, "Should equal expected value")
		require.Len(t, root.Children, 2, "Length should be 2")

		assert.Equal(t, "2", root.Children[0].ID, "Should equal expected value")
		assert.Equal(t, "3", root.Children[1].ID, "Should equal expected value")
		assert.Empty(t, root.Children[0].Children, "Should be empty")
		assert.Empty(t, root.Children[1].Children, "Should be empty")
	})

	t.Run("BuildsTreeWithMultipleRoots", func(t *testing.T) {
		nodes := createTestNodes()

		result := Build(nodes, adapter)

		require.Len(t, result, 3, "Length should be 3")

		root1 := findNodeByID(result, "1")
		root2 := findNodeByID(result, "6")
		orphan := findNodeByID(result, "8")

		require.NotNil(t, root1, "Should not be nil")
		require.NotNil(t, root2, "Should not be nil")
		require.NotNil(t, orphan, "Should not be nil")

		assert.Equal(t, "Root 1", root1.Name, "Should equal expected value")
		assert.Len(t, root1.Children, 2, "Length should be 2")

		assert.Equal(t, "Root 2", root2.Name, "Should equal expected value")
		assert.Len(t, root2.Children, 1, "Length should be 1")

		assert.Equal(t, "Orphan", orphan.Name, "Should equal expected value")
		assert.Empty(t, orphan.Children, "Should be empty")
	})

	t.Run("BuildsDeepNestedTree", func(t *testing.T) {
		nodes := createComplexTestNodes()

		result := Build(nodes, adapter)

		require.Len(t, result, 2, "Length should be 2")

		root1 := findNodeByID(result, "root1")
		require.NotNil(t, root1, "Should not be nil")
		require.Len(t, root1.Children, 2, "Length should be 2")

		childA := findNodeByID(root1.Children, "a")
		require.NotNil(t, childA, "Should not be nil")
		require.Len(t, childA.Children, 2, "Length should be 2")

		childC := findNodeByID(childA.Children, "c")
		require.NotNil(t, childC, "Should not be nil")
		assert.Len(t, childC.Children, 2, "Length should be 2")
	})

	t.Run("HandlesEmptySlice", func(t *testing.T) {
		var nodes []TestNode

		result := Build(nodes, adapter)

		assert.NotNil(t, result, "Should not be nil")
		assert.Empty(t, result, "Should be empty")
	})

	t.Run("HandlesSingleNode", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "1", ParentID: "", Name: "Single"},
		}

		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")
		assert.Equal(t, "1", result[0].ID, "Should equal expected value")
		assert.Equal(t, "Single", result[0].Name, "Should equal expected value")
		assert.Empty(t, result[0].Children, "Should be empty")
	})

	t.Run("HandlesNodesWithEmptyIDs", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "", ParentID: "", Name: "Empty ID"},
			{ID: "1", ParentID: "", Name: "Valid"},
		}

		result := Build(nodes, adapter)

		require.Len(t, result, 2, "Length should be 2")

		validNode := findNodeByID(result, "1")
		require.NotNil(t, validNode, "Should not be nil")
		assert.Equal(t, "Valid", validNode.Name, "Should equal expected value")
	})

	t.Run("HandlesCircularReferencesGracefully", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "1", ParentID: "2", Name: "Node 1"},
			{ID: "2", ParentID: "1", Name: "Node 2"},
		}

		result := Build(nodes, adapter)

		require.Empty(t, result, "Should be empty")
	})

	t.Run("HandlesPartialCircularReferences", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "root", ParentID: "", Name: "Root"},
			{ID: "1", ParentID: "2", Name: "Node 1"},
			{ID: "2", ParentID: "1", Name: "Node 2"},
			{ID: "3", ParentID: "root", Name: "Node 3"},
		}

		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")
		root := result[0]
		assert.Equal(t, "root", root.ID, "Should equal expected value")
		require.Len(t, root.Children, 1, "Length should be 1")
		assert.Equal(t, "3", root.Children[0].ID, "Should equal expected value")
	})

	t.Run("WorksWithDifferentDataTypes", func(t *testing.T) {
		categories := []TestCategory{
			{CategoryID: "tech", ParentCatID: "", CategoryName: "Technology", Level: 1},
			{CategoryID: "software", ParentCatID: "tech", CategoryName: "Software", Level: 2},
			{CategoryID: "hardware", ParentCatID: "tech", CategoryName: "Hardware", Level: 2},
			{CategoryID: "ai", ParentCatID: "software", CategoryName: "AI", Level: 3},
		}

		categoryAdapter := createTestCategoryAdapter()
		result := Build(categories, categoryAdapter)

		require.Len(t, result, 1, "Length should be 1")
		tech := result[0]
		assert.Equal(t, "tech", tech.CategoryID, "Should equal expected value")
		assert.Equal(t, "Technology", tech.CategoryName, "Should equal expected value")
		require.Len(t, tech.SubCategories, 2, "Length should be 2")

		software := findCategoryByID(tech.SubCategories, "software")
		require.NotNil(t, software, "Should not be nil")
		require.Len(t, software.SubCategories, 1, "Length should be 1")
		assert.Equal(t, "ai", software.SubCategories[0].CategoryID, "Should equal expected value")
	})
}

// TestFindNode tests find node functionality.
func TestFindNode(t *testing.T) {
	adapter := createTestNodeAdapter()

	t.Run("FindsRootNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		result, found := FindNode(tree, "1", adapter)

		assert.True(t, found, "Should be found")
		assert.Equal(t, "1", result.ID, "Should equal expected value")
		assert.Equal(t, "Root 1", result.Name, "Should equal expected value")
	})

	t.Run("FindsDeepNestedNode", func(t *testing.T) {
		nodes := createComplexTestNodes()
		tree := Build(nodes, adapter)

		result, found := FindNode(tree, "f", adapter)

		assert.True(t, found, "Should be found")
		assert.Equal(t, "f", result.ID, "Should equal expected value")
		assert.Equal(t, "F", result.Name, "Should equal expected value")
	})

	t.Run("FindsLeafNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		result, found := FindNode(tree, "4", adapter)

		assert.True(t, found, "Should be found")
		assert.Equal(t, "4", result.ID, "Should equal expected value")
		assert.Equal(t, "Child 1-1-1", result.Name, "Should equal expected value")
	})

	t.Run("FindsIntermediateNodeWithChildren", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		result, found := FindNode(tree, "2", adapter)

		assert.True(t, found, "Should be found")
		assert.Equal(t, "2", result.ID, "Should equal expected value")
		assert.Equal(t, "Child 1-1", result.Name, "Should equal expected value")
		assert.Len(t, result.Children, 2, "Length should be 2")
	})

	t.Run("ReturnsFalseForNonExistentNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		result, found := FindNode(tree, "nonexistent", adapter)

		assert.False(t, found, "Should not be found")
		assert.Equal(t, "", result.ID, "Should equal expected value")
	})

	t.Run("ReturnsFalseForEmptyTargetID", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		_, found := FindNode(tree, "", adapter)

		assert.False(t, found, "Should not be found")
	})

	t.Run("HandlesEmptyTree", func(t *testing.T) {
		var tree []TestNode

		_, found := FindNode(tree, "1", adapter)

		assert.False(t, found, "Should not be found")
	})

	t.Run("FindsNodesInDifferentBranches", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		result1, found1 := FindNode(tree, "2", adapter)
		assert.True(t, found1, "Should be found")
		assert.Equal(t, "2", result1.ID, "Should equal expected value")

		result2, found2 := FindNode(tree, "7", adapter)
		assert.True(t, found2, "Should be found")
		assert.Equal(t, "7", result2.ID, "Should equal expected value")

		result3, found3 := FindNode(tree, "8", adapter)
		assert.True(t, found3, "Should be found")
		assert.Equal(t, "8", result3.ID, "Should equal expected value")
	})

	t.Run("WorksWithDifferentDataTypes", func(t *testing.T) {
		categories := []TestCategory{
			{CategoryID: "tech", ParentCatID: "", CategoryName: "Technology"},
			{CategoryID: "software", ParentCatID: "tech", CategoryName: "Software"},
			{CategoryID: "ai", ParentCatID: "software", CategoryName: "AI"},
		}

		categoryAdapter := createTestCategoryAdapter()
		tree := Build(categories, categoryAdapter)

		result, found := FindNode(tree, "ai", categoryAdapter)

		assert.True(t, found, "Should be found")
		assert.Equal(t, "ai", result.CategoryID, "Should equal expected value")
		assert.Equal(t, "AI", result.CategoryName, "Should equal expected value")
	})

	t.Run("FindsFirstOccurrenceWithDuplicateIDs", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "1", ParentID: "", Name: "Root"},
			{ID: "2", ParentID: "1", Name: "Child 1"},
			{ID: "2", ParentID: "1", Name: "Child 2"},
		}

		tree := Build(nodes, adapter)
		result, found := FindNode(tree, "2", adapter)

		assert.True(t, found, "Should be found")
		assert.Equal(t, "2", result.ID, "Should equal expected value")
		assert.Contains(t, []string{"Child 1", "Child 2"}, result.Name, "Should contain expected value")
	})
}

// TestFindNodePath tests find node path functionality.
func TestFindNodePath(t *testing.T) {
	adapter := createTestNodeAdapter()

	t.Run("FindsPathToRootNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "1", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 1, "Length should be 1")
		assert.Equal(t, "1", path[0].ID, "Should equal expected value")
		assert.Equal(t, "Root 1", path[0].Name, "Should equal expected value")
	})

	t.Run("FindsPathToDeepNestedNode", func(t *testing.T) {
		nodes := createComplexTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "f", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 4, "Length should be 4")
		assert.Equal(t, "root1", path[0].ID, "Should equal expected value")
		assert.Equal(t, "a", path[1].ID, "Should equal expected value")
		assert.Equal(t, "c", path[2].ID, "Should equal expected value")
		assert.Equal(t, "f", path[3].ID, "Should equal expected value")
	})

	t.Run("FindsPathToImmediateChild", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "2", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 2, "Length should be 2")
		assert.Equal(t, "1", path[0].ID, "Should equal expected value")
		assert.Equal(t, "2", path[1].ID, "Should equal expected value")
	})

	t.Run("FindsPathToLeafNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "4", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 3, "Length should be 3")
		assert.Equal(t, "1", path[0].ID, "Should equal expected value")
		assert.Equal(t, "2", path[1].ID, "Should equal expected value")
		assert.Equal(t, "4", path[2].ID, "Should equal expected value")
	})

	t.Run("FindsPathToOrphanNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "8", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 1, "Length should be 1")
		assert.Equal(t, "8", path[0].ID, "Should equal expected value")
		assert.Equal(t, "Orphan", path[0].Name, "Should equal expected value")
	})

	t.Run("ReturnsNilForNonExistentNode", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "nonexistent", adapter)

		assert.False(t, found, "Should not be found")
		assert.Nil(t, path, "Should be nil")
	})

	t.Run("ReturnsNilForEmptyTargetID", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "", adapter)

		assert.False(t, found, "Should not be found")
		assert.Nil(t, path, "Should be nil")
	})

	t.Run("HandlesEmptyTree", func(t *testing.T) {
		var tree []TestNode

		path, found := FindNodePath(tree, "1", adapter)

		assert.False(t, found, "Should not be found")
		assert.Nil(t, path, "Should be nil")
	})

	t.Run("FindsPathsInDifferentBranches", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path1, found1 := FindNodePath(tree, "5", adapter)
		assert.True(t, found1, "Should be found")
		require.Len(t, path1, 3, "Length should be 3")
		assert.Equal(t, "1", path1[0].ID, "Should equal expected value")
		assert.Equal(t, "2", path1[1].ID, "Should equal expected value")
		assert.Equal(t, "5", path1[2].ID, "Should equal expected value")

		path2, found2 := FindNodePath(tree, "7", adapter)
		assert.True(t, found2, "Should be found")
		require.Len(t, path2, 2, "Length should be 2")
		assert.Equal(t, "6", path2[0].ID, "Should equal expected value")
		assert.Equal(t, "7", path2[1].ID, "Should equal expected value")
	})

	t.Run("PathContainsCompleteNodeData", func(t *testing.T) {
		nodes := createTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "4", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 3, "Length should be 3")

		assert.Equal(t, "Root 1", path[0].Name, "Should equal expected value")
		assert.Equal(t, "Child 1-1", path[1].Name, "Should equal expected value")
		assert.Equal(t, "Child 1-1-1", path[2].Name, "Should equal expected value")

		assert.Equal(t, "", path[0].ParentID, "Should equal expected value")
		assert.Equal(t, "1", path[1].ParentID, "Should equal expected value")
		assert.Equal(t, "2", path[2].ParentID, "Should equal expected value")
	})

	t.Run("WorksWithDifferentDataTypes", func(t *testing.T) {
		categories := []TestCategory{
			{CategoryID: "tech", ParentCatID: "", CategoryName: "Technology", Level: 1},
			{CategoryID: "software", ParentCatID: "tech", CategoryName: "Software", Level: 2},
			{CategoryID: "ai", ParentCatID: "software", CategoryName: "AI", Level: 3},
		}

		categoryAdapter := createTestCategoryAdapter()
		tree := Build(categories, categoryAdapter)

		path, found := FindNodePath(tree, "ai", categoryAdapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 3, "Length should be 3")
		assert.Equal(t, "tech", path[0].CategoryID, "Should equal expected value")
		assert.Equal(t, "software", path[1].CategoryID, "Should equal expected value")
		assert.Equal(t, "ai", path[2].CategoryID, "Should equal expected value")

		assert.Equal(t, 1, path[0].Level, "Should equal expected value")
		assert.Equal(t, 2, path[1].Level, "Should equal expected value")
		assert.Equal(t, 3, path[2].Level, "Should equal expected value")
	})

	t.Run("FindsCorrectPathInComplexTree", func(t *testing.T) {
		nodes := createComplexTestNodes()
		tree := Build(nodes, adapter)

		path, found := FindNodePath(tree, "g", adapter)

		assert.True(t, found, "Should be found")
		require.Len(t, path, 4, "Length should be 4")
		assert.Equal(t, "root1", path[0].ID, "Should equal expected value")
		assert.Equal(t, "a", path[1].ID, "Should equal expected value")
		assert.Equal(t, "c", path[2].ID, "Should equal expected value")
		assert.Equal(t, "g", path[3].ID, "Should equal expected value")
	})
}

// TestAdapterEdgeCases tests Adapter edge cases scenarios.
func TestAdapterEdgeCases(t *testing.T) {
	t.Run("AdapterWithNilFunctionsPanics", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "1", ParentID: "", Name: "Test"},
		}

		badAdapter := Adapter[TestNode]{}

		assert.Panics(t, func() {
			Build(nodes, badAdapter)
		}, "Should panic as expected")
	})

	t.Run("LargeTreePerformance", func(t *testing.T) {
		const nodeCount = 1000

		nodes := make([]TestNode, nodeCount)

		nodes[0] = TestNode{ID: "root", ParentID: "", Name: "Root"}
		for i := 1; i < nodeCount; i++ {
			nodes[i] = TestNode{
				ID:       fmt.Sprintf("child_%d", i),
				ParentID: "root",
				Name:     fmt.Sprintf("Child %d", i),
			}
		}

		adapter := createTestNodeAdapter()
		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")
		assert.Equal(t, "root", result[0].ID, "Should equal expected value")
		assert.Len(t, result[0].Children, nodeCount-1, "Length should be nodeCount-1")
	})

	t.Run("DeepNestingPerformance", func(t *testing.T) {
		const depth = 100

		nodes := make([]TestNode, depth)

		nodes[0] = TestNode{ID: "0", ParentID: "", Name: "Root"}
		for i := 1; i < depth; i++ {
			nodes[i] = TestNode{
				ID:       fmt.Sprintf("%d", i),
				ParentID: fmt.Sprintf("%d", i-1),
				Name:     fmt.Sprintf("Level %d", i),
			}
		}

		adapter := createTestNodeAdapter()
		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")

		current := result[0]

		depthCount := 1
		for len(current.Children) > 0 {
			current = current.Children[0]
			depthCount++
		}

		assert.Equal(t, depth, depthCount, "Should equal expected value")
	})

	t.Run("NodesWithSpecialCharactersInIDs", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "root/path", ParentID: "", Name: "Root with slash"},
			{ID: "child@domain.com", ParentID: "root/path", Name: "Child with email"},
			{ID: "special#$%^&*()", ParentID: "root/path", Name: "Special chars"},
			{ID: "unicode_测试_🌟", ParentID: "child@domain.com", Name: "Unicode"},
		}

		adapter := createTestNodeAdapter()
		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")
		root := result[0]
		assert.Equal(t, "root/path", root.ID, "Should equal expected value")
		require.Len(t, root.Children, 2, "Length should be 2")

		emailChild := findNodeByID(root.Children, "child@domain.com")
		require.NotNil(t, emailChild, "Should not be nil")
		require.Len(t, emailChild.Children, 1, "Length should be 1")
		assert.Equal(t, "unicode_测试_🌟", emailChild.Children[0].ID, "Should equal expected value")
	})

	t.Run("ConcurrentReadSafety", func(*testing.T) {
		nodes := createComplexTestNodes()
		adapter := createTestNodeAdapter()
		tree := Build(nodes, adapter)

		done := make(chan bool, 10)

		for range 10 {
			go func() {
				defer func() { done <- true }()

				FindNode(tree, "f", adapter)
				FindNodePath(tree, "g", adapter)
				FindNode(tree, "root1", adapter)

				for _, root := range tree {
					_ = root.Children
					for _, child := range root.Children {
						_ = child.Name
					}
				}
			}()
		}

		for range 10 {
			<-done
		}
	})

	t.Run("AdapterFunctionConsistency", func(t *testing.T) {
		nodes := []TestNode{
			{ID: "1", ParentID: "", Name: "Root", Children: []TestNode{}},
		}

		adapter := createTestNodeAdapter()

		node := nodes[0]
		assert.Equal(t, "1", adapter.GetID(node), "Should equal expected value")
		assert.Nil(t, adapter.GetParentID(node), "Should equal expected value")
		assert.Empty(t, adapter.GetChildren(node), "Should be empty")

		newChildren := []TestNode{{ID: "child", Name: "Test Child"}}
		adapter.SetChildren(&nodes[0], newChildren)
		assert.Equal(t, newChildren, nodes[0].Children, "Should equal expected value")
	})
}

// TestAdapterBenchmarkScenarios tests Adapter benchmark scenarios.
func TestAdapterBenchmarkScenarios(t *testing.T) {
	t.Run("BalancedTreeStructure", func(t *testing.T) {
		const nodeCount = 100

		nodes := make([]TestNode, nodeCount)
		nodes[0] = TestNode{ID: "root", ParentID: "", Name: "Root"}

		for i := 1; i < nodeCount; i++ {
			parentIndex := (i - 1) / 3
			nodes[i] = TestNode{
				ID:       fmt.Sprintf("node_%d", i),
				ParentID: fmt.Sprintf("node_%d", parentIndex),
				Name:     fmt.Sprintf("Node %d", i),
			}
		}

		nodes[1].ParentID = "root"
		nodes[2].ParentID = "root"
		nodes[3].ParentID = "root"

		adapter := createTestNodeAdapter()
		result := Build(nodes, adapter)

		require.Len(t, result, 1, "Length should be 1")

		var countNodes func([]TestNode) int

		countNodes = func(nodes []TestNode) int {
			count := len(nodes)
			for _, node := range nodes {
				count += countNodes(node.Children)
			}

			return count
		}

		assert.Equal(t, nodeCount, countNodes(result), "Should equal expected value")
	})
}
