package relay

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

// Synthetic inline images exercise the same byte-heavy operations as a large
// Responses request. Each sample populates then reuses an asset cache. This
// measures local preparation, not network latency or actual token counting.
func BenchmarkResponsesPreparation(b *testing.B) {
	for _, size := range []int{2150000, 6450000} {
		b.Run(fmt.Sprintf("%dMB", size/1000000), func(b *testing.B) {
			rng := rand.New(rand.NewSource(1))
			parts := make([]map[string]string, 18)
			for i := range parts {
				data := make([]byte, size/18*3/4)
				_, _ = rng.Read(data)
				copy(data, "\x89PNG\r\n\x1a\n")
				parts[i] = map[string]string{"type": "input_image", "image_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)}
			}
			input, err := common.Marshal([]any{map[string]any{"role": "user", "content": parts}})
			require.NoError(b, err)
			request := &dto.OpenAIResponsesRequest{Model: "gpt-5", Input: input,
				Tools:     []byte(`[{"type":"function","name":"example"}]`),
				Reasoning: &dto.Reasoning{Effort: "high"}, Store: []byte(`false`)}
			body, err := common.Marshal(request)
			require.NoError(b, err)
			b.Run("copy", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_, err := common.DeepCopy(request)
					require.NoError(b, err)
				}
			})
			b.Run("disabled_fields", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_, err := relaycommon.RemoveDisabledFields(body, dto.ChannelOtherSettings{}, false)
					require.NoError(b, err)
				}
			})
			b.Run("tools", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_, err := removeResponsesImageGenerationToolsFromBody(body)
					require.NoError(b, err)
				}
			})
			b.Run("token_meta", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = request.GetTokenCountMeta()
				}
			})
			b.Run("pipeline", func(b *testing.B) {
				cache, err := service.NewRelayImageCache(b.TempDir(), 16<<20, time.Minute)
				require.NoError(b, err)
				defer cache.Close()
				b.ReportAllocs()
				b.SetBytes(int64(len(body)))
				for b.Loop() {
					_ = request.GetTokenCountMeta()
					cloned, err := common.DeepCopy(request)
					require.NoError(b, err)
					require.NoError(b, cloned.RemoveImageGenerationTools())
					data, err := common.Marshal(cloned)
					require.NoError(b, err)
					data, err = relaycommon.RemoveDisabledFields(data, dto.ChannelOtherSettings{}, false)
					require.NoError(b, err)
					data, err = removeResponsesImageGenerationToolsFromBody(data)
					require.NoError(b, err)
					data, release, count := service.RewriteRelayAssetURLs(data, cache, service.RelayAssetRewriteOptions{Protocol: "responses", UserID: 1, BaseURL: "https://example.test"})
					require.Equal(b, 18, count)
					var compressed bytes.Buffer
					writer, err := zstd.NewWriter(&compressed, zstd.WithEncoderConcurrency(1), zstd.WithEncoderLevel(zstd.SpeedFastest))
					require.NoError(b, err)
					_, err = writer.Write(data)
					require.NoError(b, err)
					require.NoError(b, writer.Close())
					release()
				}
			})
		})
	}
}
