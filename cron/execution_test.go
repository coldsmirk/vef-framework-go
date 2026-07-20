package cron

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionBindParams(t *testing.T) {
	t.Run("AbsentParamsLeaveTargetUntouched", func(t *testing.T) {
		target := map[string]string{"keep": "me"}

		require.NoError(t, Execution{}.BindParams(&target), "Binding absent params must succeed")
		assert.Equal(t, map[string]string{"keep": "me"}, target, "The target must stay untouched")
	})

	t.Run("MalformedParamsFail", func(t *testing.T) {
		var target map[string]string

		err := Execution{Params: json.RawMessage(`{broken`)}.BindParams(&target)
		assert.Error(t, err, "Malformed params must fail loudly")
	})
}
