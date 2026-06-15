package resource

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneofValues extracts the space-separated values of an `oneof=...` token from
// a validate struct tag (e.g. "required,oneof=approve reject" -> [approve reject]).
func oneofValues(validateTag string) []string {
	for part := range strings.SplitSeq(validateTag, ",") {
		if after, ok := strings.CutPrefix(part, "oneof="); ok {
			return strings.Fields(after)
		}
	}

	return nil
}

// TestProcessTaskActionsCoverOneofTag is the compile-time-equivalent drift
// guard for ProcessTask dispatch: the set of actions the oneof validator
// accepts must exactly match the keys of processTaskDispatch. This replaces
// the former dead runtime default arm — an action added to the tag without a
// dispatch entry (or vice versa) fails here instead of silently succeeding
// (or being unreachable defensive plumbing).
func TestProcessTaskActionsCoverOneofTag(t *testing.T) {
	field, ok := reflect.TypeFor[ProcessTaskParams]().FieldByName("Action")
	require.True(t, ok, "ProcessTaskParams must have an Action field")

	allowed := oneofValues(field.Tag.Get("validate"))
	require.NotEmpty(t, allowed, "Action field must carry an oneof validate tag")

	t.Run("EveryOneofValueHasADispatchEntry", func(t *testing.T) {
		for _, action := range allowed {
			_, dispatched := processTaskDispatch[processTaskAction(action)]
			assert.True(t, dispatched,
				"oneof action %q must have a processTaskDispatch entry", action)
		}
	})

	t.Run("EveryDispatchEntryIsAnOneofValue", func(t *testing.T) {
		allowedSet := make(map[string]struct{}, len(allowed))
		for _, a := range allowed {
			allowedSet[a] = struct{}{}
		}

		for action := range processTaskDispatch {
			_, valid := allowedSet[string(action)]
			assert.True(t, valid,
				"processTaskDispatch key %q must be a value in the oneof validate tag", action)
		}
	})

	t.Run("CountsMatch", func(t *testing.T) {
		assert.Len(t, processTaskDispatch, len(allowed),
			"dispatch table and oneof tag must enumerate the same number of actions")
	})
}
