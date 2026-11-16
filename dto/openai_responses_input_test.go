package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestResponsesParseInputPreservesTokenMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        []MediaInput
	}{
		{"string", `"hello\n世界"`, []MediaInput{{Type: "input_text", Text: "hello\n世界"}}},
		{"content", `[{"role":"user","content":"hello"},{"content":[{"type":"input_text","text":"world"},{"type":"input_image","image_url":"data:image/png;base64,YWJj"},{"type":"input_image","image_url":{"url":"https://example.test/a.png"}},{"type":"input_file","file_url":{"url":"https://example.test/a.pdf"}},{"type":"input_file","file_url":"https://example.test/b.pdf"}]}]`, []MediaInput{
			{Type: "input_text", Text: "hello"}, {Type: "input_text", Text: "world"},
			{Type: "input_image", ImageUrl: "data:image/png;base64,YWJj"}, {Type: "input_image", ImageUrl: "https://example.test/a.png"},
			{Type: "input_file", FileUrl: "https://example.test/a.pdf"}, {Type: "input_file", FileUrl: "https://example.test/b.pdf"},
		}},
		{"unsupported", `[null,1,{"content":[null,1,{}, {"type":"output_text","text":"skip"},{"type":"input_text","text":42},{"type":"input_image","image_url":false},{"type":"input_file","file_url":{"url":7}}]}]`, []MediaInput{{Type: "input_text"}, {Type: "input_image"}, {Type: "input_file"}}},
		{"duplicate keys", `[{"content":[{"type":"input_text","text":"first","text":"last"}]}]`, []MediaInput{{Type: "input_text", Text: "last"}}},
		{"case insensitive struct field", `[{"Content":"first","CONTENT":"last"}]`, []MediaInput{{Type: "input_text", Text: "last"}}},
		{"empty", `[]`, nil}, {"null", `null`, nil}, {"object", `{}`, nil}, {"invalid", `[{`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &OpenAIResponsesRequest{Input: []byte(tc.input)}
			require.Equal(t, tc.want, r.ParseInput())
		})
	}
	r := &OpenAIResponsesRequest{Input: []byte(`[{"content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"https://example.test/a.png"}]}]`), Tools: []byte(`[]`)}
	meta := r.GetTokenCountMeta()
	require.Equal(t, "hello\n[]", meta.CombineText)
	require.Len(t, meta.Files, 1)
	require.Equal(t, types.FileTypeImage, meta.Files[0].FileType)
}
