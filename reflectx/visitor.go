package reflectx

import (
	"container/list"
	"reflect"

	"github.com/coldsmirk/go-collections"
)

// TraversalMode defines the traversal strategy for visiting struct fields and methods.
type TraversalMode int

const (
	DepthFirst TraversalMode = iota
	BreadthFirst
)

// VisitAction represents the action to take after visiting a node.
type VisitAction int

const (
	Continue VisitAction = iota
	Stop
	SkipChildren
)

// TagConfig configures which tagged fields should be recursively traversed.
type TagConfig struct {
	Name  string
	Value string
}

// VisitorConfig configures how the traversal should be performed.
type VisitorConfig struct {
	TraversalMode TraversalMode
	Recursive     bool
	DiveTag       TagConfig
	MaxDepth      int
}

// VisitorOption configures visitor behavior.
type VisitorOption func(*VisitorConfig)

// WithTraversalMode selects the traversal strategy. The default is DepthFirst.
func WithTraversalMode(mode TraversalMode) VisitorOption {
	return func(c *VisitorConfig) { c.TraversalMode = mode }
}

// WithDisableRecursive limits the traversal to the root struct's own fields,
// skipping anonymous embeds and dive-tagged fields. Recursion is enabled by default.
func WithDisableRecursive() VisitorOption {
	return func(c *VisitorConfig) { c.Recursive = false }
}

// WithDiveTag overrides the struct tag that marks non-anonymous fields for
// recursion. The default is tag "visit" with value "dive".
func WithDiveTag(tagName, tagValue string) VisitorOption {
	return func(c *VisitorConfig) { c.DiveTag = TagConfig{Name: tagName, Value: tagValue} }
}

// WithMaxDepth caps how deep the traversal descends. Zero (the default) means unlimited.
func WithMaxDepth(maxDepth int) VisitorOption {
	return func(c *VisitorConfig) { c.MaxDepth = maxDepth }
}

func defaultVisitorConfig() VisitorConfig {
	return VisitorConfig{
		TraversalMode: DepthFirst,
		Recursive:     true,
		DiveTag:       TagConfig{Name: "visit", Value: "dive"},
	}
}

// Visitor defines callback functions for struct traversal.
type Visitor struct {
	VisitStruct StructVisitor
	VisitField  FieldVisitor
	VisitMethod MethodVisitor
}

type (
	StructVisitor func(structType reflect.Type, structValue reflect.Value, depth int) VisitAction
	FieldVisitor  func(field reflect.StructField, fieldValue reflect.Value, depth int) VisitAction
	MethodVisitor func(method reflect.Method, methodValue reflect.Value, depth int) VisitAction
)

// TypeVisitor defines callback functions for type-only traversal.
type TypeVisitor struct {
	VisitStructType StructTypeVisitor
	VisitFieldType  FieldTypeVisitor
	VisitMethodType MethodTypeVisitor
}

type (
	StructTypeVisitor func(structType reflect.Type, depth int) VisitAction
	FieldTypeVisitor  func(field reflect.StructField, depth int) VisitAction
	MethodTypeVisitor func(method reflect.Method, receiverType reflect.Type, depth int) VisitAction
)

// VisitFor visits a struct type T using type visitor callbacks.
func VisitFor[T any](visitor TypeVisitor, opts ...VisitorOption) {
	VisitType(reflect.TypeFor[T](), visitor, opts...)
}

// VisitOf visits a struct value using visitor callbacks.
func VisitOf(value any, visitor Visitor, opts ...VisitorOption) {
	Visit(reflect.ValueOf(value), visitor, opts...)
}

// Visit traverses a struct using visitor callbacks.
func Visit(target reflect.Value, visitor Visitor, opts ...VisitorOption) {
	config := defaultVisitorConfig()
	for _, opt := range opts {
		opt(&config)
	}

	if !target.IsValid() {
		return
	}

	for target.Kind() == reflect.Pointer {
		if target.IsNil() {
			return
		}

		target = target.Elem()
	}

	if target.Kind() != reflect.Struct {
		return
	}

	if config.TraversalMode == DepthFirst {
		visitDepthFirst(target, config, visitor, collections.NewHashSet[reflect.Type](), 0, nil)
	} else {
		visitBreadthFirst(target, config, visitor)
	}
}

// VisitType traverses a struct type using type visitor callbacks.
func VisitType(targetType reflect.Type, visitor TypeVisitor, opts ...VisitorOption) {
	config := defaultVisitorConfig()
	for _, opt := range opts {
		opt(&config)
	}

	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}

	if targetType.Kind() != reflect.Struct {
		return
	}

	if config.TraversalMode == DepthFirst {
		visitTypeDepthFirst(targetType, config, visitor, collections.NewHashSet[reflect.Type](), 0, nil)
	} else {
		visitTypeBreadthFirst(targetType, config, visitor)
	}
}

func visitDepthFirst(target reflect.Value, config VisitorConfig, visitor Visitor, ancestors collections.Set[reflect.Type], depth int, parentIndexPath []int) VisitAction {
	if config.MaxDepth > 0 && depth >= config.MaxDepth {
		return Continue
	}

	for target.Kind() == reflect.Pointer {
		if target.IsNil() {
			return Continue
		}

		target = target.Elem()
	}

	if target.Kind() != reflect.Struct {
		return Continue
	}

	targetType := target.Type()
	if ancestors.Contains(targetType) {
		return Continue
	}

	ancestors.Add(targetType)
	defer ancestors.Remove(targetType)

	if visitor.VisitStruct != nil {
		if action := visitor.VisitStruct(targetType, target, depth); action != Continue {
			return action
		}
	}

	for i := range target.NumField() {
		field := target.Field(i)
		fieldType := targetType.Field(i)

		if !field.CanInterface() {
			continue
		}

		fieldTypeCopy := fieldWithAbsoluteIndex(fieldType, parentIndexPath)

		if visitor.VisitField != nil {
			switch action := visitor.VisitField(fieldTypeCopy, field, depth); action {
			case Stop:
				return Stop
			case SkipChildren:
				continue
			}
		}

		if config.Recursive && shouldRecurse(fieldType, config.DiveTag) {
			if visitDepthFirst(field, config, visitor, ancestors, depth+1, fieldTypeCopy.Index) == Stop {
				return Stop
			}
		}
	}

	if visitMethods(target, visitor.VisitMethod, depth) == Stop {
		return Stop
	}

	return Continue
}

func visitBreadthFirst(target reflect.Value, config VisitorConfig, visitor Visitor) {
	type queueItem struct {
		value           reflect.Value
		depth           int
		parentIndexPath []int
		ancestors       collections.Set[reflect.Type]
	}

	queue := list.New()
	queue.PushBack(queueItem{target, 0, nil, collections.NewHashSet[reflect.Type]()})

	for queue.Len() > 0 {
		item := queue.Remove(queue.Front()).(queueItem)
		current, depth, parentIndexPath, ancestors := item.value, item.depth, item.parentIndexPath, item.ancestors

		if config.MaxDepth > 0 && depth >= config.MaxDepth {
			continue
		}

		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				break
			}

			current = current.Elem()
		}

		if current.Kind() != reflect.Struct {
			continue
		}

		currentType := current.Type()
		if ancestors.Contains(currentType) {
			continue
		}

		childAncestors := ancestors.Clone()
		childAncestors.Add(currentType)

		if visitor.VisitStruct != nil {
			if visitor.VisitStruct(currentType, current, depth) == Stop {
				return
			}
		}

		for i := range current.NumField() {
			field := current.Field(i)
			fieldType := currentType.Field(i)

			if !field.CanInterface() {
				continue
			}

			fieldTypeCopy := fieldWithAbsoluteIndex(fieldType, parentIndexPath)
			skipChildren := false

			if visitor.VisitField != nil {
				switch action := visitor.VisitField(fieldTypeCopy, field, depth); action {
				case Stop:
					return
				case SkipChildren:
					skipChildren = true
				}
			}

			if !skipChildren && config.Recursive && shouldRecurse(fieldType, config.DiveTag) {
				queue.PushBack(queueItem{field, depth + 1, fieldTypeCopy.Index, childAncestors})
			}
		}

		if visitMethods(current, visitor.VisitMethod, depth) == Stop {
			return
		}
	}
}

func visitTypeDepthFirst(targetType reflect.Type, config VisitorConfig, visitor TypeVisitor, ancestors collections.Set[reflect.Type], depth int, parentIndexPath []int) VisitAction {
	if config.MaxDepth > 0 && depth >= config.MaxDepth {
		return Continue
	}

	if ancestors.Contains(targetType) {
		return Continue
	}

	ancestors.Add(targetType)
	defer ancestors.Remove(targetType)

	if visitor.VisitStructType != nil {
		if action := visitor.VisitStructType(targetType, depth); action != Continue {
			return action
		}
	}

	for field := range targetType.Fields() {
		if !field.IsExported() {
			continue
		}

		fieldCopy := fieldWithAbsoluteIndex(field, parentIndexPath)

		if visitor.VisitFieldType != nil {
			switch action := visitor.VisitFieldType(fieldCopy, depth); action {
			case Stop:
				return Stop
			case SkipChildren:
				continue
			}
		}

		if config.Recursive && shouldRecurse(field, config.DiveTag) {
			if visitTypeDepthFirst(Indirect(field.Type), config, visitor, ancestors, depth+1, fieldCopy.Index) == Stop {
				return Stop
			}
		}
	}

	if visitMethodTypes(targetType, visitor.VisitMethodType, depth) == Stop {
		return Stop
	}

	return Continue
}

func visitTypeBreadthFirst(targetType reflect.Type, config VisitorConfig, visitor TypeVisitor) {
	type queueItem struct {
		structType      reflect.Type
		depth           int
		parentIndexPath []int
		ancestors       collections.Set[reflect.Type]
	}

	queue := list.New()
	queue.PushBack(queueItem{targetType, 0, nil, collections.NewHashSet[reflect.Type]()})

	for queue.Len() > 0 {
		item := queue.Remove(queue.Front()).(queueItem)
		current := Indirect(item.structType)
		depth, parentIndexPath, ancestors := item.depth, item.parentIndexPath, item.ancestors

		if config.MaxDepth > 0 && depth >= config.MaxDepth {
			continue
		}

		if current.Kind() != reflect.Struct {
			continue
		}

		if ancestors.Contains(current) {
			continue
		}

		childAncestors := ancestors.Clone()
		childAncestors.Add(current)

		if visitor.VisitStructType != nil {
			if visitor.VisitStructType(current, depth) == Stop {
				return
			}
		}

		for field := range current.Fields() {
			if !field.IsExported() {
				continue
			}

			fieldCopy := fieldWithAbsoluteIndex(field, parentIndexPath)
			skipChildren := false

			if visitor.VisitFieldType != nil {
				switch action := visitor.VisitFieldType(fieldCopy, depth); action {
				case Stop:
					return
				case SkipChildren:
					skipChildren = true
				}
			}

			if !skipChildren && config.Recursive && shouldRecurse(field, config.DiveTag) {
				queue.PushBack(queueItem{field.Type, depth + 1, fieldCopy.Index, childAncestors})
			}
		}

		if visitMethodTypes(current, visitor.VisitMethodType, depth) == Stop {
			return
		}
	}
}

func visitMethods(target reflect.Value, visitor MethodVisitor, depth int) VisitAction {
	if visitor == nil {
		return Continue
	}

	ptrTarget := addressablePointer(target)
	ptrType := ptrTarget.Type()

	for i := range ptrTarget.NumMethod() {
		if visitor(ptrType.Method(i), ptrTarget.Method(i), depth) == Stop {
			return Stop
		}
	}

	return Continue
}

func visitMethodTypes(targetType reflect.Type, visitor MethodTypeVisitor, depth int) VisitAction {
	if visitor == nil {
		return Continue
	}

	ptrType := reflect.PointerTo(targetType)
	for method := range ptrType.Methods() {
		if visitor(method, ptrType, depth) == Stop {
			return Stop
		}
	}

	return Continue
}

func shouldRecurse(field reflect.StructField, diveTag TagConfig) bool {
	if field.Anonymous {
		return Indirect(field.Type).Kind() == reflect.Struct
	}

	if diveTag.Name != "" && diveTag.Value != "" && field.Tag.Get(diveTag.Name) == diveTag.Value {
		return Indirect(field.Type).Kind() == reflect.Struct
	}

	return false
}

func buildAbsoluteIndexPath(parentIndexPath []int, field reflect.StructField) []int {
	if len(parentIndexPath) > 0 {
		fullIndexPath := make([]int, len(parentIndexPath)+len(field.Index))
		copy(fullIndexPath, parentIndexPath)
		copy(fullIndexPath[len(parentIndexPath):], field.Index)

		return fullIndexPath
	}

	return append([]int(nil), field.Index...)
}

func fieldWithAbsoluteIndex(field reflect.StructField, parentIndexPath []int) reflect.StructField {
	field.Index = buildAbsoluteIndexPath(parentIndexPath, field)

	return field
}
