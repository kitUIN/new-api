package common_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestDeepCopyResponsesPreservesValuesAndIsolatesBuffers(t *testing.T) {
	var request dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal([]byte(`{"model":"gpt-5","background":false,"stream":false,"temperature":0,"top_p":0,"top_logprobs":0,"max_output_tokens":0,"max_tool_calls":0,"reasoning":{"effort":"high"},"stream_options":{"include_usage":true}}`), &request))
	// Cover every RawMessage field, including ones added to the DTO later.
	value := reflect.ValueOf(&request).Elem()
	for i := 0; i < value.NumField(); i++ {
		if value.Field(i).Type() == reflect.TypeOf(json.RawMessage{}) {
			value.Field(i).SetBytes([]byte(`{"nested":[0,false,9007199254740993]}`))
		}
	}
	before, err := common.Marshal(request)
	require.NoError(t, err)
	cloned, err := common.DeepCopy(&request)
	require.NoError(t, err)
	after, err := common.Marshal(cloned)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.Equal(t, request, *cloned)
	cloneValue := reflect.ValueOf(cloned).Elem()
	for i := 0; i < cloneValue.NumField(); i++ {
		field := cloneValue.Field(i)
		switch field.Kind() {
		case reflect.Slice:
			if field.Len() > 0 {
				field.Index(0).SetUint('!')
			}
		case reflect.Pointer:
			require.NotEqual(t, value.Field(i).Pointer(), field.Pointer(), value.Type().Field(i).Name)
		}
	}
	cloned.Reasoning.Effort = "low"
	cloned.StreamOptions.IncludeUsage = false
	unchanged, err := common.Marshal(request)
	require.NoError(t, err)
	require.Equal(t, string(before), string(unchanged), "conversion must not contaminate a retry")
}

func TestDeepCopyNestedByteBuffers(t *testing.T) {
	type payload struct {
		Bytes []byte
		Raw   json.RawMessage
		Items []json.RawMessage
		Map   map[string]json.RawMessage
	}
	for _, raw := range []json.RawMessage{nil, {}, []byte(`false`)} {
		src := payload{Bytes: []byte{1, 2}, Raw: raw, Items: []json.RawMessage{[]byte(`0`)}, Map: map[string]json.RawMessage{"key": []byte(`false`)}}
		cloned, err := common.DeepCopy(&src)
		require.NoError(t, err)
		require.Equal(t, src, *cloned)
		cloned.Bytes[0] = 3
		cloned.Items[0][0] = '1'
		cloned.Map["key"][0] = '!'
		require.Equal(t, byte(1), src.Bytes[0])
		require.Equal(t, "0", string(src.Items[0]))
		require.Equal(t, "false", string(src.Map["key"]))
	}
	_, err := common.DeepCopy[payload](nil)
	require.Error(t, err)
}
