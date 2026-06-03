package tree

import (
	"github.com/samber/lo"
)

// Adapter provides functions to access tree node properties.
type Adapter[T any] struct {
	// GetID returns the unique identifier of a node. Nodes with an empty ID are ignored.
	GetID func(T) string
	// GetParentID returns the identifier of a node's parent, or nil if the node is a root.
	GetParentID func(T) *string
	// GetChildren returns the children already attached to a node.
	GetChildren func(T) []T
	// SetChildren assigns the children of a node in place via the given pointer.
	SetChildren func(*T, []T)
}

// Build constructs a tree structure from a flat slice of nodes using the provided adapter.
// A node is treated as a root when its parent ID is nil or refers to an ID not present in
// nodes. Nodes whose parent chain never reaches such a root (for example a closed cycle
// like 1->2->1) are unreachable and therefore omitted from the result.
func Build[T any](nodes []T, adapter Adapter[T]) []T {
	if len(nodes) == 0 {
		return []T{}
	}

	nodeMap := make(map[string]*T, len(nodes))
	childrenMap := make(map[string][]*T)

	for i := range nodes {
		node := &nodes[i]
		if id := adapter.GetID(*node); id != "" {
			nodeMap[id] = node
		}
	}

	for i := range nodes {
		node := &nodes[i]
		if parentID := adapter.GetParentID(*node); parentID != nil {
			childrenMap[*parentID] = append(childrenMap[*parentID], node)
		}
	}

	visited := make(map[string]bool)

	var setChildrenRecursively func(*T)

	setChildrenRecursively = func(nodePtr *T) {
		id := adapter.GetID(*nodePtr)
		if id == "" || visited[id] {
			return
		}

		visited[id] = true

		childPointers, exists := childrenMap[id]
		if !exists {
			return
		}

		for _, childPointer := range childPointers {
			setChildrenRecursively(childPointer)
		}

		children := make([]T, len(childPointers))
		for i, childPointer := range childPointers {
			children[i] = *childPointer
		}

		adapter.SetChildren(nodePtr, children)
	}

	for i := range nodes {
		setChildrenRecursively(&nodes[i])
	}

	roots := make([]T, 0)
	for _, node := range nodes {
		parentID := adapter.GetParentID(node)
		if parentID == nil || nodeMap[*parentID] == nil {
			roots = append(roots, node)
		}
	}

	return roots
}

// FindNode searches for a node with the given ID in the tree and returns it if found.
func FindNode[T any](roots []T, targetID string, adapter Adapter[T]) (T, bool) {
	if targetID == "" {
		return lo.Empty[T](), false
	}

	return findNodeRecursive(roots, targetID, adapter)
}

// FindNodePath returns the path from root to the target node if found.
func FindNodePath[T any](roots []T, targetID string, adapter Adapter[T]) ([]T, bool) {
	if targetID == "" {
		return nil, false
	}

	for _, root := range roots {
		if path, found := findNodePathRecursive(root, targetID, nil, adapter); found {
			return path, true
		}
	}

	return nil, false
}

func findNodeRecursive[T any](nodes []T, targetID string, adapter Adapter[T]) (T, bool) {
	for _, node := range nodes {
		if adapter.GetID(node) == targetID {
			return node, true
		}

		if found, ok := findNodeRecursive(adapter.GetChildren(node), targetID, adapter); ok {
			return found, true
		}
	}

	return lo.Empty[T](), false
}

func findNodePathRecursive[T any](node T, targetID string, currentPath []T, adapter Adapter[T]) ([]T, bool) {
	path := append(currentPath, node)

	if adapter.GetID(node) == targetID {
		return path, true
	}

	for _, child := range adapter.GetChildren(node) {
		if result, found := findNodePathRecursive(child, targetID, path, adapter); found {
			return result, true
		}
	}

	return nil, false
}
