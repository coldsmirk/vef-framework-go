package param

import (
	"reflect"
	"slices"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/reflectx"
)

var (
	apiParamsType = reflect.TypeFor[api.P]()
	apiMetaType   = reflect.TypeFor[api.M]()

	builtinMetaTypes = []reflect.Type{
		reflect.TypeFor[page.Pageable](),
	}
)

func isBuiltinMetaType(t reflect.Type) bool {
	return slices.Contains(builtinMetaTypes, t)
}

// findFieldInStruct uses a multi-pass search strategy to balance explicitness with flexibility.
func findFieldInStruct(target reflect.Value, paramType reflect.Type) reflect.Value {
	if found := searchDirectFields(target, paramType); found.IsValid() {
		return found
	}

	if found := searchTaggedFields(target, paramType); found.IsValid() {
		return found
	}

	return searchEmbeddedFields(target, paramType)
}

func searchDirectFields(target reflect.Value, paramType reflect.Type) reflect.Value {
	var foundField reflect.Value

	visitor := reflectx.Visitor{
		VisitField: func(field reflect.StructField, fieldValue reflect.Value, _ int) reflectx.VisitAction {
			if !field.Anonymous && reflectx.IsTypeCompatible(fieldValue.Type(), paramType) {
				foundField = fieldValue

				return reflectx.Stop
			}

			return reflectx.Continue
		},
	}

	reflectx.Visit(target, visitor, reflectx.WithDisableRecursive())

	return foundField
}

func searchTaggedFields(target reflect.Value, paramType reflect.Type) reflect.Value {
	var foundField reflect.Value

	visitor := reflectx.Visitor{
		VisitField: func(field reflect.StructField, fieldValue reflect.Value, _ int) reflectx.VisitAction {
			if field.Anonymous {
				return reflectx.SkipChildren
			}

			if reflectx.IsTypeCompatible(fieldValue.Type(), paramType) {
				foundField = fieldValue

				return reflectx.Stop
			}

			return reflectx.Continue
		},
	}

	reflectx.Visit(target, visitor, reflectx.WithDiveTag("api", "dive"))

	return foundField
}

func searchEmbeddedFields(target reflect.Value, paramType reflect.Type) reflect.Value {
	var foundField reflect.Value

	visitor := reflectx.Visitor{
		VisitField: func(field reflect.StructField, fieldValue reflect.Value, _ int) reflectx.VisitAction {
			if !field.Anonymous {
				return reflectx.SkipChildren
			}

			if reflectx.IsTypeCompatible(fieldValue.Type(), paramType) {
				foundField = fieldValue

				return reflectx.Stop
			}

			return reflectx.Continue
		},
	}

	reflectx.Visit(target, visitor)

	return foundField
}

func embedsAPIParams(targetType reflect.Type) bool {
	return embedsSentinelType(targetType, apiParamsType)
}

func embedsAPIMeta(targetType reflect.Type) bool {
	return embedsSentinelType(targetType, apiMetaType)
}

// embedsSentinelType uses breadth-first search to handle deeply nested embeddings correctly.
func embedsSentinelType(targetType, sentinelType reflect.Type) bool {
	targetType = reflectx.Indirect(targetType)
	if targetType.Kind() != reflect.Struct {
		return false
	}

	queue := []reflect.Type{targetType}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.Kind() != reflect.Struct {
			continue
		}

		if current == sentinelType {
			return true
		}

		for field := range current.Fields() {
			if field.Anonymous {
				queue = append(queue, field.Type)
			}
		}
	}

	return false
}
