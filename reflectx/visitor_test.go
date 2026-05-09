package reflectx

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type BaseVisitorTest struct {
	BaseValue string
}

func (BaseVisitorTest) BaseMethod() string {
	return "base"
}

type VisitorTestEmbedded struct {
	BaseVisitorTest

	EmbeddedValue int
	Services      *VisitorTestServices `visit:"dive"`
}

func (VisitorTestEmbedded) EmbeddedMethod() string {
	return "embedded"
}

type VisitorTestServices struct {
	Logger VisitorTestLogger `visit:"dive"`
	Cache  *VisitorTestCache `visit:"dive"`
}

type VisitorTestLogger struct {
	Level string
}

type VisitorTestCache struct {
	Size int
}

type VisitorTestNested struct {
	VisitorTestEmbedded

	NestedValue bool
}

type VisitorSharedBase struct {
	SharedValue string
}

type VisitorBranchA struct {
	VisitorSharedBase
}

type VisitorBranchB struct {
	VisitorSharedBase
}

type VisitorSharedRoot struct {
	VisitorBranchA
	VisitorBranchB
}

// TestVisitDepthFirst tests Visit depth first scenarios.
func TestVisitDepthFirst(t *testing.T) {
	// Create test structure
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
			EmbeddedValue:   42,
			Services: &VisitorTestServices{
				Logger: VisitorTestLogger{Level: "info"},
				Cache:  &VisitorTestCache{Size: 100},
			},
		},
		NestedValue: true,
	}

	var (
		visitedStructs []string
		visitedFields  []string
		visitedMethods []string
	)

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
		VisitMethod: func(method reflect.Method, _ reflect.Value, _ int) VisitAction {
			visitedMethods = append(visitedMethods, method.Name)

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	expectedStructs := []string{"VisitorTestNested", "VisitorTestEmbedded", "BaseVisitorTest", "VisitorTestServices", "VisitorTestLogger", "VisitorTestCache"}
	assert.Equal(t, expectedStructs, visitedStructs, "Structs should be visited in depth-first order")

	assert.Contains(t, visitedFields, "NestedValue", "Should visit NestedValue field")
	assert.Contains(t, visitedFields, "EmbeddedValue", "Should visit EmbeddedValue field")
	assert.Contains(t, visitedFields, "BaseValue", "Should visit BaseValue field")
	assert.Contains(t, visitedFields, "Services", "Should visit Services field")
	assert.Contains(t, visitedFields, "Logger", "Should visit Logger field")
	assert.Contains(t, visitedFields, "Cache", "Should visit Cache field")

	assert.Contains(t, visitedMethods, "BaseMethod", "Should visit BaseMethod")
	assert.Contains(t, visitedMethods, "EmbeddedMethod", "Should visit EmbeddedMethod")
}

// TestVisitBreadthFirst tests Visit breadth first scenarios.
func TestVisitBreadthFirst(t *testing.T) {
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
			EmbeddedValue:   42,
			Services: &VisitorTestServices{
				Logger: VisitorTestLogger{Level: "info"},
				Cache:  &VisitorTestCache{Size: 100},
			},
		},
		NestedValue: true,
	}

	var (
		visitedStructs []string
		depths         []int
	)

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, depth int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())
			depths = append(depths, depth)

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst))

	require.Len(t, visitedStructs, 6, "Should visit 6 structs total")
	require.Len(t, depths, 6, "Should record 6 depths")

	assert.Equal(t, "VisitorTestNested", visitedStructs[0], "Root struct should be visited first")
	assert.Equal(t, 0, depths[0], "Root struct should be at depth 0")

	depth1Structs := []string{}

	for i, depth := range depths {
		if depth == 1 {
			depth1Structs = append(depth1Structs, visitedStructs[i])
		}
	}

	assert.Contains(t, depth1Structs, "VisitorTestEmbedded", "Embedded struct should be at depth 1")
}

// TestVisitMaxDepth tests Visit max depth scenarios.
func TestVisitMaxDepth(t *testing.T) {
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
			Services: &VisitorTestServices{
				Logger: VisitorTestLogger{Level: "info"},
			},
		},
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithMaxDepth(2))

	assert.NotContains(t, visitedStructs, "VisitorTestLogger", "Should not visit deeper structures due to MaxDepth")
}

// TestVisitStopAction tests Visit stop action scenarios.
func TestVisitStopAction(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		EmbeddedValue:   42,
	}

	var visitedFields []string

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)
			if field.Name == "EmbeddedValue" {
				return Stop // Stop traversal when we find EmbeddedValue
			}

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	assert.Contains(t, visitedFields, "EmbeddedValue", "Should visit EmbeddedValue before stopping")

	laterFieldFound := false
	embeddedValueIndex := -1

	for i, field := range visitedFields {
		if field == "EmbeddedValue" {
			embeddedValueIndex = i

			break
		}
	}

	for i := embeddedValueIndex + 1; i < len(visitedFields); i++ {
		if visitedFields[i] != "" {
			laterFieldFound = true
		}
	}

	assert.False(t, laterFieldFound, "Should not visit fields after Stop action")
}

// TestVisitSkipChildrenAction tests Visit skip children action scenarios.
func TestVisitSkipChildrenAction(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		Services: &VisitorTestServices{
			Logger: VisitorTestLogger{Level: "info"},
		},
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			if field.Name == "Services" {
				return SkipChildren // Don't traverse into Services
			}

			return Continue
		},
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	// Should not visit VisitorTestServices or its nested structures due to SkipChildren
	assert.NotContains(t, visitedStructs, "VisitorTestServices", "Should not contain value")
	assert.NotContains(t, visitedStructs, "VisitorTestLogger", "Should not contain value")
}

// TestVisitTaggedFields tests Visit tagged fields scenarios.
func TestVisitTaggedFields(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		Services: &VisitorTestServices{
			Logger: VisitorTestLogger{Level: "info"},
			Cache:  &VisitorTestCache{Size: 100},
		},
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	// Should visit Services due to visit:"dive" tag
	assert.Contains(t, visitedStructs, "VisitorTestServices", "Should contain expected value")
	// Should also visit Cache due to its visit:"dive" tag
	assert.Contains(t, visitedStructs, "VisitorTestCache", "Should contain expected value")
}

// TestVisitNoRecursion tests Visit no recursion scenarios.
func TestVisitNoRecursion(t *testing.T) {
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		},
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithDisableRecursive())

	// Should only visit the top-level struct
	assert.Equal(t, []string{"VisitorTestNested"}, visitedStructs, "Should equal expected value")
}

// TestVisitNilPointer tests Visit nil pointer scenarios.
func TestVisitNilPointer(t *testing.T) {
	var nilStruct *BaseVisitorTest

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(nilStruct), visitor)

	// Should not visit anything for nil pointer
	assert.Empty(t, visitedStructs, "Should be empty")
}

// TestVisitNonStruct tests Visit non struct scenarios.
func TestVisitNonStruct(t *testing.T) {
	testValue := "not a struct"

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testValue), visitor)

	// Should not visit anything for non-struct types
	assert.Empty(t, visitedStructs, "Should be empty")
}

// Tests for type-only visitor API

// TestVisitTypeDepthFirst tests VisitType depth first scenarios.
func TestVisitTypeDepthFirst(t *testing.T) {
	var (
		visitedTypes   []string
		visitedFields  []string
		visitedMethods []string
	)

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
		VisitMethodType: func(method reflect.Method, _ reflect.Type, _ int) VisitAction {
			visitedMethods = append(visitedMethods, method.Name)

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestNested](), visitor)

	// Verify types are visited in depth-first order
	expectedTypes := []string{"VisitorTestNested", "VisitorTestEmbedded", "BaseVisitorTest", "VisitorTestServices", "VisitorTestLogger", "VisitorTestCache"}
	assert.Equal(t, expectedTypes, visitedTypes, "Should equal expected value")

	// Verify key fields are visited
	assert.Contains(t, visitedFields, "NestedValue", "Should contain expected value")
	assert.Contains(t, visitedFields, "EmbeddedValue", "Should contain expected value")
	assert.Contains(t, visitedFields, "BaseValue", "Should contain expected value")
	assert.Contains(t, visitedFields, "Services", "Should contain expected value")
	assert.Contains(t, visitedFields, "Logger", "Should contain expected value")
	assert.Contains(t, visitedFields, "Cache", "Should contain expected value")

	// Verify methods are visited
	assert.Contains(t, visitedMethods, "BaseMethod", "Should contain expected value")
	assert.Contains(t, visitedMethods, "EmbeddedMethod", "Should contain expected value")
}

// TestVisitTypeBreadthFirst tests VisitType breadth first scenarios.
func TestVisitTypeBreadthFirst(t *testing.T) {
	var (
		visitedTypes []string
		depths       []int
	)

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, depth int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())
			depths = append(depths, depth)

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestNested](), visitor, WithTraversalMode(BreadthFirst))

	// Verify breadth-first ordering
	require.Len(t, visitedTypes, 6, "Length should be 6")
	require.Len(t, depths, 6, "Length should be 6")

	assert.Equal(t, "VisitorTestNested", visitedTypes[0], "Should equal expected value")
	assert.Equal(t, 0, depths[0], "Should equal expected value")

	// Find types at depth 1
	depth1Types := []string{}

	for i, depth := range depths {
		if depth == 1 {
			depth1Types = append(depth1Types, visitedTypes[i])
		}
	}

	assert.Contains(t, depth1Types, "VisitorTestEmbedded", "Should contain expected value")
}

// TestVisitTypeMaxDepth tests VisitType max depth scenarios.
func TestVisitTypeMaxDepth(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestNested](), visitor, WithMaxDepth(2))

	// Should not visit deeper structures due to MaxDepth
	assert.NotContains(t, visitedTypes, "VisitorTestLogger", "Should not contain value")
}

// TestVisitTypeStopAction tests VisitType stop action scenarios.
func TestVisitTypeStopAction(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())
			if structType.Name() == "VisitorTestEmbedded" {
				return Stop // Stop traversal when we find VisitorTestEmbedded
			}

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestNested](), visitor)

	// Should stop after finding VisitorTestEmbedded
	assert.Contains(t, visitedTypes, "VisitorTestEmbedded", "Should contain expected value")
	assert.NotContains(t, visitedTypes, "BaseVisitorTest", "Should not contain value")
}

// TestVisitTypeSkipChildrenAction tests VisitType skip children action scenarios.
func TestVisitTypeSkipChildrenAction(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			if field.Name == "Services" {
				return SkipChildren // Don't traverse into Services
			}

			return Continue
		},
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestEmbedded](), visitor)

	// Should not visit VisitorTestServices or its nested structures due to SkipChildren
	assert.NotContains(t, visitedTypes, "VisitorTestServices", "Should not contain value")
	assert.NotContains(t, visitedTypes, "VisitorTestLogger", "Should not contain value")
}

// TestVisitTypeNonStruct tests VisitType non struct scenarios.
func TestVisitTypeNonStruct(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[string](), visitor)

	// Should not visit anything for non-struct types
	assert.Empty(t, visitedTypes, "Should be empty")
}

// TestVisitTypePointerToStruct tests VisitType pointer to struct scenarios.
func TestVisitTypePointerToStruct(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[*BaseVisitorTest](), visitor)

	// Should visit the underlying struct type
	assert.Contains(t, visitedTypes, "BaseVisitorTest", "Should contain expected value")
}

// TestMethodVisitorCallableMethodValue tests MethodVisitor callable method value scenarios.
func TestMethodVisitorCallableMethodValue(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test_value"},
	}

	var methodResults []string

	visitor := Visitor{
		VisitMethod: func(method reflect.Method, methodValue reflect.Value, _ int) VisitAction {
			if method.Name == "BaseMethod" {
				// methodValue should be a callable method value bound to the receiver.
				results := methodValue.Call(nil)
				if len(results) > 0 {
					methodResults = append(methodResults, results[0].String())
				}
			}

			return Continue
		},
		VisitStruct: func(_ reflect.Type, _ reflect.Value, depth int) VisitAction {
			// Only visit the first-level struct to avoid repetition.
			if depth > 0 {
				return SkipChildren
			}

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	// Verify we can call methodValue directly.
	assert.Len(t, methodResults, 1, "Length should be 1")
	assert.Equal(t, "base", methodResults[0], "Should equal expected value")
}

// TestVisitorNilCheckBehavior tests Visitor nil check behavior scenarios.
func TestVisitorNilCheckBehavior(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
	}

	var (
		visitedStructs []string
		visitedFields  []string
		visitedMethods []string
	)

	// Test with only struct visitor (no field or method visitors)
	visitor1 := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
		// VisitField and VisitMethod are nil - should not be called
	}

	Visit(reflect.ValueOf(testStruct), visitor1)

	// Should visit structs but not fields or methods
	assert.Contains(t, visitedStructs, "VisitorTestEmbedded", "Should contain expected value")
	assert.Contains(t, visitedStructs, "BaseVisitorTest", "Should contain expected value")
	assert.Empty(t, visitedFields, "Should be empty")
	assert.Empty(t, visitedMethods, "Should be empty")

	// Reset and test with all visitors
	visitedStructs = nil
	visitedFields = nil
	visitedMethods = nil

	visitor2 := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
		VisitMethod: func(method reflect.Method, _ reflect.Value, _ int) VisitAction {
			visitedMethods = append(visitedMethods, method.Name)

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor2)

	// Should visit structs, fields, and methods
	assert.Contains(t, visitedStructs, "VisitorTestEmbedded", "Should contain expected value")
	assert.Contains(t, visitedFields, "BaseValue", "Should contain expected value")
	assert.Contains(t, visitedMethods, "BaseMethod", "Should contain expected value")
}

// TestVisitForGeneric tests VisitFor generic scenarios.
func TestVisitForGeneric(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	// Use the generic convenience function
	VisitFor[VisitorTestNested](visitor)

	// Should visit all struct types in the hierarchy
	assert.Contains(t, visitedTypes, "VisitorTestNested", "Should contain expected value")
	assert.Contains(t, visitedTypes, "VisitorTestEmbedded", "Should contain expected value")
	assert.Contains(t, visitedTypes, "BaseVisitorTest", "Should contain expected value")
}

// TestVisitOfConvenience tests VisitOf convenience scenarios.
func TestVisitOfConvenience(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		EmbeddedValue:   42,
	}

	var (
		visitedStructs []string
		visitedFields  []string
	)

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
	}

	// Use the convenience function
	VisitOf(testStruct, visitor)

	// Should visit structs and fields
	assert.Contains(t, visitedStructs, "VisitorTestEmbedded", "Should contain expected value")
	assert.Contains(t, visitedStructs, "BaseVisitorTest", "Should contain expected value")
	assert.Contains(t, visitedFields, "BaseValue", "Should contain expected value")
	assert.Contains(t, visitedFields, "EmbeddedValue", "Should contain expected value")
}

// Test edge cases and boundary conditions

// TestVisitEmptyStruct tests Visit empty struct scenarios.
func TestVisitEmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	testStruct := EmptyStruct{}

	var (
		visitedStructs []string
		visitedFields  []string
		visitedMethods []string
	)

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
		VisitMethod: func(method reflect.Method, _ reflect.Value, _ int) VisitAction {
			visitedMethods = append(visitedMethods, method.Name)

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	assert.Equal(t, []string{"EmptyStruct"}, visitedStructs, "Should equal expected value")
	assert.Empty(t, visitedFields, "Should be empty")
	assert.Empty(t, visitedMethods, "Should be empty")
}

// TestVisitUnexportedFields tests Visit unexported fields scenarios.
func TestVisitUnexportedFields(t *testing.T) {
	type StructWithUnexportedFields struct {
		PublicField  string
		privateField int // Should be skipped
	}

	testStruct := StructWithUnexportedFields{
		PublicField:  "public",
		privateField: 42,
	}

	var visitedFields []string

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	assert.Equal(t, []string{"PublicField"}, visitedFields, "Should equal expected value")
}

// TestVisitMultiplePointerLevels tests Visit multiple pointer levels scenarios.
func TestVisitMultiplePointerLevels(t *testing.T) {
	testStruct := BaseVisitorTest{BaseValue: "test"}
	ptrToStruct := &testStruct
	ptrToPtr := &ptrToStruct

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(ptrToPtr), visitor)

	require.Len(t, visitedStructs, 1, "Length should be 1")
	assert.Equal(t, "BaseVisitorTest", visitedStructs[0], "Should equal expected value")
}

// TestVisitInvalidValue tests Visit invalid value scenarios.
func TestVisitInvalidValue(t *testing.T) {
	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	// Test with zero value (invalid)
	var invalidValue reflect.Value
	Visit(invalidValue, visitor)

	assert.Empty(t, visitedStructs, "Should be empty")
}

// TestVisitCyclicReference tests Visit cyclic reference scenarios.
func TestVisitCyclicReference(t *testing.T) {
	// Use struct with pointer to itself to test cycle detection
	type SelfReferencing struct {
		Value string
		Next  *SelfReferencing `visit:"dive"`
	}

	node1 := &SelfReferencing{Value: "node1"}
	node2 := &SelfReferencing{Value: "node2"}
	node1.Next = node2
	node2.Next = node1 // Create cycle

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, structValue reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name()+"_"+structValue.FieldByName("Value").String())

			return Continue
		},
	}

	Visit(reflect.ValueOf(node1), visitor)

	// Should visit each instance, but prevent infinite recursion
	// Due to cycle detection, the same struct type should not cause infinite loop
	assert.NotEmpty(t, visitedStructs, "Should not be empty")
	// The exact behavior depends on implementation, but it shouldn't hang
	assert.True(t, len(visitedStructs) < 10, "Should not visit too many instances due to cycle detection")
}

// TestVisitMethodsOnNonAddressableValue tests Visit methods on non addressable value scenarios.
func TestVisitMethodsOnNonAddressableValue(t *testing.T) {
	// Create non-addressable value (result of function call)
	getValue := func() BaseVisitorTest {
		return BaseVisitorTest{BaseValue: "test"}
	}

	var visitedMethods []string

	visitor := Visitor{
		VisitMethod: func(method reflect.Method, _ reflect.Value, _ int) VisitAction {
			visitedMethods = append(visitedMethods, method.Name)

			return Continue
		},
	}

	Visit(reflect.ValueOf(getValue()), visitor)

	// Should be able to visit methods on non-addressable values
	// BaseVisitorTest has BaseMethod defined
	assert.Contains(t, visitedMethods, "BaseMethod", "Should contain expected value")
}

// TestVisitMaxDepthZero tests Visit max depth zero scenarios.
func TestVisitMaxDepthZero(t *testing.T) {
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "base"},
			EmbeddedValue:   42,
		},
		NestedValue: true,
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	// MaxDepth 0 should visit root struct and embedded anonymous fields (depth 0)
	// but not dive into tagged fields (depth 1+)
	Visit(reflect.ValueOf(testStruct), visitor, WithMaxDepth(0))

	// VisitorTestNested embeds VisitorTestEmbedded anonymously,
	// and VisitorTestEmbedded embeds BaseVisitorTest anonymously
	// All of these are visited at depth 0 due to anonymous embedding
	assert.Contains(t, visitedStructs, "VisitorTestNested", "Should contain expected value")
	assert.Contains(t, visitedStructs, "VisitorTestEmbedded", "Should contain expected value")
	assert.Contains(t, visitedStructs, "BaseVisitorTest", "Should contain expected value")
}

// TestVisitTypeWithNilVisitors tests VisitType with nil visitors scenarios.
func TestVisitTypeWithNilVisitors(t *testing.T) {
	var visitedStructs []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
		// VisitFieldType and VisitMethodType are nil
	}

	VisitType(reflect.TypeFor[BaseVisitorTest](), visitor)

	assert.Equal(t, []string{"BaseVisitorTest"}, visitedStructs, "Should equal expected value")
}

// Tests for field index path tracking in embedded structures

// TestVisitFieldIndexPathAnonymousEmbedded tests Visit field index path anonymous embedded scenarios.
func TestVisitFieldIndexPathAnonymousEmbedded(t *testing.T) {
	// Test that anonymous embedded fields have correct index paths
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
			EmbeddedValue:   42,
		},
		NestedValue: true,
	}

	fieldIndexMap := make(map[string][]int)

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			fieldIndexMap[field.Name] = field.Index

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	// Verify nested field index paths
	// BaseValue is in BaseVisitorTest (embedded in VisitorTestEmbedded, which is embedded in VisitorTestNested)
	assert.NotNil(t, fieldIndexMap["BaseValue"], "Should not be nil")
	assert.Equal(t, []int{0, 0, 0}, fieldIndexMap["BaseValue"], "BaseValue should have path [0,0,0]")

	// EmbeddedValue is in VisitorTestEmbedded (embedded in VisitorTestNested)
	assert.NotNil(t, fieldIndexMap["EmbeddedValue"], "Should not be nil")
	assert.Equal(t, []int{0, 1}, fieldIndexMap["EmbeddedValue"], "EmbeddedValue should have path [0,1]")

	// NestedValue is a direct field of VisitorTestNested
	assert.NotNil(t, fieldIndexMap["NestedValue"], "Should not be nil")
	assert.Equal(t, []int{1}, fieldIndexMap["NestedValue"], "NestedValue should have path [1]")
}

// TestVisitTypeFieldIndexPathTaggedDive tests VisitType field index path tagged dive scenarios.
func TestVisitTypeFieldIndexPathTaggedDive(t *testing.T) {
	// Test that non-anonymous fields with dive tag have correct index paths
	fieldIndexMap := make(map[string][]int)

	visitor := TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			fieldIndexMap[field.Name] = field.Index

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestEmbedded](), visitor)

	// Services field is at [2] in VisitorTestEmbedded
	assert.NotNil(t, fieldIndexMap["Services"], "Should not be nil")
	assert.Equal(t, []int{2}, fieldIndexMap["Services"], "Services should have path [2]")

	// Logger is inside Services, which has dive tag
	assert.NotNil(t, fieldIndexMap["Logger"], "Should not be nil")
	assert.Equal(t, []int{2, 0}, fieldIndexMap["Logger"], "Logger should have path [2,0]")

	// Level is inside Logger
	assert.NotNil(t, fieldIndexMap["Level"], "Should not be nil")
	assert.Equal(t, []int{2, 0, 0}, fieldIndexMap["Level"], "Level should have path [2,0,0]")

	// Cache is inside Services
	assert.NotNil(t, fieldIndexMap["Cache"], "Should not be nil")
	assert.Equal(t, []int{2, 1}, fieldIndexMap["Cache"], "Cache should have path [2,1]")

	// Size is inside Cache
	assert.NotNil(t, fieldIndexMap["Size"], "Should not be nil")
	assert.Equal(t, []int{2, 1, 0}, fieldIndexMap["Size"], "Size should have path [2,1,0]")
}

// TestVisitFieldIndexPathCanAccessValues tests Visit field index path can access values scenarios.
func TestVisitFieldIndexPathCanAccessValues(t *testing.T) {
	// Test that index paths can be used to access actual field values
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "base_value"},
		EmbeddedValue:   42,
		Services: &VisitorTestServices{
			Logger: VisitorTestLogger{Level: "debug"},
			Cache:  &VisitorTestCache{Size: 1024},
		},
	}

	type fieldInfo struct {
		index []int
		value reflect.Value
	}

	fieldMap := make(map[string]fieldInfo)

	visitor := Visitor{
		VisitField: func(field reflect.StructField, fieldValue reflect.Value, _ int) VisitAction {
			fieldMap[field.Name] = fieldInfo{
				index: field.Index,
				value: fieldValue,
			}

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	// Verify BaseValue
	info := fieldMap["BaseValue"]
	assert.Equal(t, []int{0, 0}, info.index, "Should equal expected value")
	assert.Equal(t, "base_value", info.value.String(), "Should equal expected value")
	// Access via index path should match
	actualValue := reflect.ValueOf(testStruct).FieldByIndex(info.index)
	assert.Equal(t, "base_value", actualValue.String(), "Should equal expected value")

	// Verify EmbeddedValue
	info = fieldMap["EmbeddedValue"]
	assert.Equal(t, []int{1}, info.index, "Should equal expected value")
	assert.Equal(t, int64(42), info.value.Int(), "Should equal expected value")
	actualValue = reflect.ValueOf(testStruct).FieldByIndex(info.index)
	assert.Equal(t, int64(42), actualValue.Int(), "Should equal expected value")

	// Verify Level (deeply nested)
	info = fieldMap["Level"]
	assert.Equal(t, []int{2, 0, 0}, info.index, "Should equal expected value")
	assert.Equal(t, "debug", info.value.String(), "Should equal expected value")
	actualValue = reflect.ValueOf(testStruct).FieldByIndex(info.index)
	assert.Equal(t, "debug", actualValue.String(), "Should equal expected value")

	// Verify Size (through pointer)
	info = fieldMap["Size"]
	assert.Equal(t, []int{2, 1, 0}, info.index, "Should equal expected value")
	assert.Equal(t, int64(1024), info.value.Int(), "Should equal expected value")
	actualValue = reflect.ValueOf(testStruct).FieldByIndex(info.index)
	assert.Equal(t, int64(1024), actualValue.Int(), "Should equal expected value")
}

// TestVisitTypeFieldIndexPathAllTraversalModes tests VisitType field index path all traversal modes scenarios.
func TestVisitTypeFieldIndexPathAllTraversalModes(t *testing.T) {
	// Test that all four traversal modes produce correct index paths
	testCases := []struct {
		name      string
		mode      TraversalMode
		useValue  bool
		fieldName string
		expected  []int
	}{
		{"TypeDepthFirst - BaseValue", DepthFirst, false, "BaseValue", []int{0, 0}},
		{"TypeDepthFirst - EmbeddedValue", DepthFirst, false, "EmbeddedValue", []int{1}},
		{"TypeDepthFirst - Level", DepthFirst, false, "Level", []int{2, 0, 0}},
		{"TypeBreadthFirst - BaseValue", BreadthFirst, false, "BaseValue", []int{0, 0}},
		{"TypeBreadthFirst - EmbeddedValue", BreadthFirst, false, "EmbeddedValue", []int{1}},
		{"TypeBreadthFirst - Level", BreadthFirst, false, "Level", []int{2, 0, 0}},
		{"ValueDepthFirst - BaseValue", DepthFirst, true, "BaseValue", []int{0, 0}},
		{"ValueDepthFirst - EmbeddedValue", DepthFirst, true, "EmbeddedValue", []int{1}},
		{"ValueDepthFirst - Level", DepthFirst, true, "Level", []int{2, 0, 0}},
		{"ValueBreadthFirst - BaseValue", BreadthFirst, true, "BaseValue", []int{0, 0}},
		{"ValueBreadthFirst - EmbeddedValue", BreadthFirst, true, "EmbeddedValue", []int{1}},
		{"ValueBreadthFirst - Level", BreadthFirst, true, "Level", []int{2, 0, 0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var foundIndex []int

			if tc.useValue {
				testValue := VisitorTestEmbedded{
					BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
					EmbeddedValue:   42,
					Services: &VisitorTestServices{
						Logger: VisitorTestLogger{Level: "info"},
					},
				}

				visitor := Visitor{
					VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
						if field.Name == tc.fieldName {
							foundIndex = field.Index

							return Stop
						}

						return Continue
					},
				}

				Visit(reflect.ValueOf(testValue), visitor, WithTraversalMode(tc.mode))
			} else {
				visitor := TypeVisitor{
					VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
						if field.Name == tc.fieldName {
							foundIndex = field.Index

							return Stop
						}

						return Continue
					},
				}

				VisitType(reflect.TypeFor[VisitorTestEmbedded](), visitor, WithTraversalMode(tc.mode))
			}

			assert.Equal(t, tc.expected, foundIndex, "Field %s should have correct index path", tc.fieldName)
		})
	}
}

// TestVisitFieldIndexPathDeepNesting tests Visit field index path deep nesting scenarios.
func TestVisitFieldIndexPathDeepNesting(t *testing.T) {
	// Test deeply nested structures (4+ levels)
	type Level4 struct {
		DeepValue string
	}

	type Level3 struct {
		Level4 `visit:"dive"`

		L3Value int
	}

	type Level2 struct {
		Level3 `visit:"dive"`

		L2Value bool
	}

	type Level1 struct {
		Level2

		L1Value float64
	}

	fieldIndexMap := make(map[string][]int)

	visitor := TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			fieldIndexMap[field.Name] = field.Index

			return Continue
		},
	}

	VisitType(reflect.TypeFor[Level1](), visitor)

	// Verify deep nesting paths
	assert.Equal(t, []int{0, 0, 0, 0}, fieldIndexMap["DeepValue"], "DeepValue at level 4 should have 4-element path")
	assert.Equal(t, []int{0, 0, 1}, fieldIndexMap["L3Value"], "L3Value at level 3 should have 3-element path")
	assert.Equal(t, []int{0, 1}, fieldIndexMap["L2Value"], "L2Value at level 2 should have 2-element path")
	assert.Equal(t, []int{1}, fieldIndexMap["L1Value"], "L1Value at level 1 should have 1-element path")
}

// TestVisitFieldIndexPathMixedEmbedding tests Visit field index path mixed embedding scenarios.
func TestVisitFieldIndexPathMixedEmbedding(t *testing.T) {
	// Test mixed anonymous and tagged dive embedding
	type Inner struct {
		InnerField string
	}

	type Middle struct {
		Inner // Anonymous embedding - always recursed

		MiddleField int
		Tagged      Inner `visit:"dive"` // Non-anonymous with dive tag - only recursed with WithDiveTag
	}

	type Outer struct {
		Middle

		OuterField bool
	}

	fieldIndexMap := make(map[string][]int)

	// Test with dive tag enabled
	visitor := TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			fieldIndexMap[field.Name] = field.Index

			return Continue
		},
	}

	VisitType(reflect.TypeFor[Outer](), visitor, WithDiveTag("visit", "dive"))

	// Anonymous InnerField should be at [0, 0, 0]
	anonymousInnerIndex, found := fieldIndexMap["InnerField"]
	assert.True(t, found, "Should find InnerField from anonymous embedding")
	assert.Equal(t, []int{0, 0, 0}, anonymousInnerIndex, "Anonymous InnerField should have path [0,0,0]")

	// MiddleField should be at [0, 1]
	assert.NotNil(t, fieldIndexMap["MiddleField"], "Should not be nil")
	assert.Equal(t, []int{0, 1}, fieldIndexMap["MiddleField"], "MiddleField should have path [0,1]")

	// OuterField should be at [1]
	assert.NotNil(t, fieldIndexMap["OuterField"], "Should not be nil")
	assert.Equal(t, []int{1}, fieldIndexMap["OuterField"], "OuterField should have path [1]")
}

// TestVisitFieldIndexPathPointerFields tests Visit field index path pointer fields scenarios.
func TestVisitFieldIndexPathPointerFields(t *testing.T) {
	// Test that index paths work correctly with pointer fields
	testStruct := VisitorTestEmbedded{
		Services: &VisitorTestServices{
			Cache: &VisitorTestCache{Size: 512},
		},
	}

	var (
		cacheFieldIndex []int
		sizeFieldIndex  []int
	)

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			switch field.Name {
			case "Cache":
				cacheFieldIndex = field.Index
			case "Size":
				sizeFieldIndex = field.Index
			}

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor)

	// Verify Cache pointer field index
	assert.Equal(t, []int{2, 1}, cacheFieldIndex, "Cache pointer field should have path [2,1]")

	// Verify Size field inside pointer
	assert.Equal(t, []int{2, 1, 0}, sizeFieldIndex, "Size inside pointer should have path [2,1,0]")

	// Verify we can access the value through the index path
	cacheValue := reflect.ValueOf(testStruct).FieldByIndex(cacheFieldIndex)
	assert.Equal(t, reflect.Pointer, cacheValue.Kind(), "Should equal expected value")
	assert.False(t, cacheValue.IsNil(), "Should be false")

	sizeValue := reflect.ValueOf(testStruct).FieldByIndex(sizeFieldIndex)
	assert.Equal(t, int64(512), sizeValue.Int(), "Should equal expected value")
}

// TestVisitTypeFieldIndexPathConsistency tests VisitType field index path consistency scenarios.
func TestVisitTypeFieldIndexPathConsistency(t *testing.T) {
	// Test that Type traversal and Value traversal produce the same index paths for non-nil fields
	testValue := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
			EmbeddedValue:   42,
			Services: &VisitorTestServices{
				Logger: VisitorTestLogger{Level: "info"},
				Cache:  &VisitorTestCache{Size: 100},
			},
		},
		NestedValue: true,
	}

	typeFieldIndices := make(map[string][]int)
	valueFieldIndices := make(map[string][]int)

	// Collect indices from Type traversal
	typeVisitor := TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			typeFieldIndices[field.Name] = field.Index

			return Continue
		},
	}
	VisitType(reflect.TypeFor[VisitorTestNested](), typeVisitor)

	// Collect indices from Value traversal
	valueVisitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			valueFieldIndices[field.Name] = field.Index

			return Continue
		},
	}
	Visit(reflect.ValueOf(testValue), valueVisitor)

	// Verify that all fields have the same indices in both traversals
	for fieldName, typeIndex := range typeFieldIndices {
		valueIndex, found := valueFieldIndices[fieldName]
		assert.True(t, found, "Field %s should be found in value traversal", fieldName)
		assert.Equal(t, typeIndex, valueIndex, "Field %s should have same index in both traversals", fieldName)
	}
}

// --- BreadthFirst coverage tests ---

// TestVisitBreadthFirstStopAction tests VisitStruct returning Stop in BreadthFirst mode.
func TestVisitBreadthFirstStopAction(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		EmbeddedValue:   42,
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Stop
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst))

	assert.Equal(t, []string{"VisitorTestEmbedded"}, visitedStructs, "Should stop at first struct")
}

// TestVisitBreadthFirstSkipChildrenAction tests VisitField returning SkipChildren in BreadthFirst mode.
func TestVisitBreadthFirstSkipChildrenAction(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		Services: &VisitorTestServices{
			Logger: VisitorTestLogger{Level: "info"},
		},
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			if field.Name == "Services" {
				return SkipChildren
			}

			return Continue
		},
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst))

	assert.NotContains(t, visitedStructs, "VisitorTestServices", "Should not visit skipped children")
	assert.NotContains(t, visitedStructs, "VisitorTestLogger", "Should not visit nested skipped children")
}

// TestVisitBreadthFirstMaxDepth tests MaxDepth in BreadthFirst mode.
func TestVisitBreadthFirstMaxDepth(t *testing.T) {
	testStruct := VisitorTestNested{
		VisitorTestEmbedded: VisitorTestEmbedded{
			BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
			Services: &VisitorTestServices{
				Logger: VisitorTestLogger{Level: "info"},
			},
		},
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst), WithMaxDepth(2))

	assert.NotContains(t, visitedStructs, "VisitorTestLogger", "Should not visit deeper structures due to MaxDepth")
}

// TestVisitBreadthFirstNilPointerField tests nil pointer handling in BreadthFirst mode.
func TestVisitBreadthFirstNilPointerField(t *testing.T) {
	testStruct := VisitorTestEmbedded{
		BaseVisitorTest: BaseVisitorTest{BaseValue: "test"},
		Services:        nil,
	}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst))

	assert.Contains(t, visitedStructs, "VisitorTestEmbedded", "Should visit root struct")
	assert.NotContains(t, visitedStructs, "VisitorTestServices", "Should not visit nil pointer target")
}

// TestVisitBreadthFirstUnexportedFields tests unexported field skipping in BreadthFirst mode.
func TestVisitBreadthFirstUnexportedFields(t *testing.T) {
	type StructWithUnexported struct {
		PublicField  string
		privateField int
	}

	testStruct := StructWithUnexported{PublicField: "public", privateField: 42}

	var visitedFields []string

	visitor := Visitor{
		VisitField: func(field reflect.StructField, _ reflect.Value, _ int) VisitAction {
			visitedFields = append(visitedFields, field.Name)

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst))

	assert.Equal(t, []string{"PublicField"}, visitedFields, "Should only visit exported fields")
}

// TestVisitBreadthFirstVisitedDedup tests visited type deduplication in BreadthFirst mode.
func TestVisitBreadthFirstVisitedDedup(t *testing.T) {
	testStruct := VisitorSharedRoot{}

	var visitedStructs []string

	visitor := Visitor{
		VisitStruct: func(structType reflect.Type, _ reflect.Value, _ int) VisitAction {
			visitedStructs = append(visitedStructs, structType.Name())

			return Continue
		},
	}

	Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(BreadthFirst))

	count := 0
	for _, name := range visitedStructs {
		if name == "VisitorSharedBase" {
			count++
		}
	}

	assert.Equal(t, 1, count, "Shared base should only be visited once")
}

// TestVisitMethodStopAction tests method visitor returning Stop in both traversal modes.
func TestVisitMethodStopAction(t *testing.T) {
	testStruct := BaseVisitorTest{BaseValue: "test"}

	modes := []struct {
		name string
		mode TraversalMode
	}{
		{"DepthFirst", DepthFirst},
		{"BreadthFirst", BreadthFirst},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			var visitedMethods []string

			visitor := Visitor{
				VisitMethod: func(method reflect.Method, _ reflect.Value, _ int) VisitAction {
					visitedMethods = append(visitedMethods, method.Name)

					return Stop
				},
			}

			Visit(reflect.ValueOf(testStruct), visitor, WithTraversalMode(m.mode))

			assert.Len(t, visitedMethods, 1, "Should stop after first method")
		})
	}
}

// --- Type visitor BreadthFirst coverage tests ---

// TestVisitTypeBreadthFirstStopAction tests VisitStructType returning Stop in BreadthFirst mode.
func TestVisitTypeBreadthFirstStopAction(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Stop
		},
	}

	VisitType(reflect.TypeFor[VisitorTestNested](), visitor, WithTraversalMode(BreadthFirst))

	assert.Equal(t, []string{"VisitorTestNested"}, visitedTypes, "Should stop at first struct")
}

// TestVisitTypeBreadthFirstSkipChildrenAction tests VisitFieldType returning SkipChildren in BreadthFirst mode.
func TestVisitTypeBreadthFirstSkipChildrenAction(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
			if field.Name == "Services" {
				return SkipChildren
			}

			return Continue
		},
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestEmbedded](), visitor, WithTraversalMode(BreadthFirst))

	assert.NotContains(t, visitedTypes, "VisitorTestServices", "Should not visit skipped children")
}

// TestVisitTypeBreadthFirstMaxDepth tests MaxDepth in BreadthFirst type traversal.
func TestVisitTypeBreadthFirstMaxDepth(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorTestNested](), visitor, WithTraversalMode(BreadthFirst), WithMaxDepth(2))

	assert.NotContains(t, visitedTypes, "VisitorTestLogger", "Should not visit deeper structures due to MaxDepth")
}

// TestVisitTypeBreadthFirstVisitedDedup tests visited type deduplication in BreadthFirst type traversal.
func TestVisitTypeBreadthFirstVisitedDedup(t *testing.T) {
	var visitedTypes []string

	visitor := TypeVisitor{
		VisitStructType: func(structType reflect.Type, _ int) VisitAction {
			visitedTypes = append(visitedTypes, structType.Name())

			return Continue
		},
	}

	VisitType(reflect.TypeFor[VisitorSharedRoot](), visitor, WithTraversalMode(BreadthFirst))

	count := 0
	for _, name := range visitedTypes {
		if name == "VisitorSharedBase" {
			count++
		}
	}

	assert.Equal(t, 1, count, "Shared base should only be visited once")
}

// TestVisitTypeUnexportedFields tests unexported field skipping in both traversal modes.
func TestVisitTypeUnexportedFields(t *testing.T) {
	type LocalStructWithPrivate struct {
		Public  string
		private int //nolint:unused // intentionally unexported to test that VisitType skips it
	}

	modes := []struct {
		name string
		mode TraversalMode
	}{
		{"DepthFirst", DepthFirst},
		{"BreadthFirst", BreadthFirst},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			var visitedFields []string

			visitor := TypeVisitor{
				VisitFieldType: func(field reflect.StructField, _ int) VisitAction {
					visitedFields = append(visitedFields, field.Name)

					return Continue
				},
			}

			VisitType(reflect.TypeFor[LocalStructWithPrivate](), visitor, WithTraversalMode(m.mode))

			assert.Equal(t, []string{"Public"}, visitedFields, "Should only visit exported fields")
		})
	}
}

// TestVisitTypeMethodStopAction tests method type visitor returning Stop in both traversal modes.
func TestVisitTypeMethodStopAction(t *testing.T) {
	modes := []struct {
		name string
		mode TraversalMode
	}{
		{"DepthFirst", DepthFirst},
		{"BreadthFirst", BreadthFirst},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			var visitedMethods []string

			visitor := TypeVisitor{
				VisitMethodType: func(method reflect.Method, _ reflect.Type, _ int) VisitAction {
					visitedMethods = append(visitedMethods, method.Name)

					return Stop
				},
			}

			VisitType(reflect.TypeFor[BaseVisitorTest](), visitor, WithTraversalMode(m.mode))

			assert.Len(t, visitedMethods, 1, "Should stop after first method")
		})
	}
}
