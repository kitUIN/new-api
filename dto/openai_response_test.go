package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestResponsesOutputArgumentsAcceptsObject(t *testing.T) {
	var resp OpenAIResponsesResponse
	data := []byte(`{
		"id": "resp_123",
		"output": [
			{
				"type": "function_call",
				"id": "fc_123",
				"call_id": "call_123",
				"name": "lookup",
				"arguments": {"query": "weather", "limit": 0}
			}
		]
	}`)

	if err := common.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal responses response: %v", err)
	}
	if got, want := resp.Output[0].Arguments, `{"query":"weather","limit":0}`; got != want {
		t.Fatalf("arguments = %s, want %s", got, want)
	}
}

func TestResponsesStreamItemArgumentsAcceptsObject(t *testing.T) {
	var resp ResponsesStreamResponse
	data := []byte(`{
		"type": "response.output_item.done",
		"item": {
			"type": "function_call",
			"id": "fc_123",
			"call_id": "call_123",
			"name": "lookup",
			"arguments": {"query": "weather", "limit": 0}
		}
	}`)

	if err := common.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal responses stream response: %v", err)
	}
	if resp.Item == nil {
		t.Fatal("item is nil")
	}
	if got, want := resp.Item.Arguments, `{"query":"weather","limit":0}`; got != want {
		t.Fatalf("arguments = %s, want %s", got, want)
	}
}

func TestResponsesOutputArgumentsKeepsString(t *testing.T) {
	var resp ResponsesStreamResponse
	data := []byte(`{
		"type": "response.output_item.done",
		"item": {
			"type": "function_call",
			"id": "fc_123",
			"arguments": "{\"query\":\"weather\",\"limit\":0}"
		}
	}`)

	if err := common.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal responses stream response: %v", err)
	}
	if resp.Item == nil {
		t.Fatal("item is nil")
	}
	if got, want := resp.Item.Arguments, `{"query":"weather","limit":0}`; got != want {
		t.Fatalf("arguments = %s, want %s", got, want)
	}
}

func TestResponsesOutputContentAcceptsArray(t *testing.T) {
	var resp OpenAIResponsesResponse
	data := []byte(`{
		"output": [{
			"type": "message",
			"role": "assistant",
			"content": [{"type": "output_text", "text": "128"}]
		}]
	}`)

	if err := common.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal responses response: %v", err)
	}
	if got, want := resp.Output[0].Content[0].Text, "128"; got != want {
		t.Fatalf("content text = %q, want %q", got, want)
	}
}

func TestResponsesOutputContentAcceptsObject(t *testing.T) {
	var resp OpenAIResponsesResponse
	data := []byte(`{
		"output": [{
			"type": "message",
			"role": "assistant",
			"content": {"type": "output_text", "text": "256"}
		}]
	}`)

	if err := common.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal responses response: %v", err)
	}
	if got, want := resp.Output[0].Content[0].Text, "256"; got != want {
		t.Fatalf("content text = %q, want %q", got, want)
	}
}
