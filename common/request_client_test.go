package common

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectRequestClientDefaults(t *testing.T) {
	require.NoError(t, UpdateRequestClientRules(DefaultRequestClientRules))
	for _, tc := range []struct{ ua, want string }{
		{"Codex Desktop/0.169.0 (Windows; x86_64)", "codex-desktop"},
		{"codex_desktop/1.0", "codex-desktop"},
		{"codex_app/1.0", "codex-desktop"},
		{"codex-tui/0.160.0 (Ubuntu) screen", "codex-tui"},
		{"codex_vscode/0.151.0", "codex-vscode"},
		{"codex_vscode/0.151.0 (Debian) unknown (code-server; 26.825.51511)", "codex-vscode"},
		{"codex-tui/0.167.0 (Windows 10.0.26200; x86_64) vscode/1.145.0 (codex-tui; 0.166.0)", "codex-tui-vscode"},
		{"codex_cli_rs/0.160.0 (Ubuntu) code-server/1.0", "codex-tui-vscode"},
		{"Codex Desktop/0.169.0 (Windows) unknown (VS Code; 26.1111)", "codex-desktop"},
		{"codex_exec/0.164.0 Orca/1.4.237", "codex"},
		{"codex_cli_rs/0.160.0", "codex-tui"},
		{"codex_sdk_ts/0.157.1", "codex"},
		{"pi (win32 10.0.19045; x64)", "pi"},
		{"pi/1.4.0 (win32; node/v24.22.1; x64)", "pi"},
		{"opencode/latest/2.2.5/cli", "opencode"},
		{"opencode/1.18.33 ai-sdk/provider-utils/4.0.40 runtime/bun/1.3.14", "opencode"},
		{"VSCode/1.145.2 CodeBuddy/4.12.39997873", "codebuddy"},
		{"deepseek-harness/0.2.1-rc.5", "deepseek-harness"},
		{"RikkaHub-Android/2.5.3", "rikkahub"},
		{"magpie/0.1.1840", "magpie"},
		{"cc-pocket/0.154.0", "cc-pocket"},
		{"omp/18.11.0", "omp"},
		{"ohmyapi", "ohmyapi"},
		{"OhMyAPI/1.0", "ohmyapi"},
		{"ohmyapi-helper/1.0", ""},
		{"CLAUDE-CLI/2.0.30", "claude-code"},
		{"Go-http-client/1.1", ""}, {"undici", ""}, {"Bun/1.4.0", ""},
		{"OpenAI/JS 6.40.0", ""}, {"ai-sdk/openai/4.0.15", ""},
		{"orca_desktop/0.169.0", ""}, {"pvf_editor/0.160.0", ""}, {"ucp/8.10.0", ""},
		{"not-codex/1.0", ""}, {"Mozilla/5.0", ""}, {"", ""},
	} {
		t.Run(tc.ua, func(t *testing.T) {
			h := make(http.Header)
			h.Set("User-Agent", tc.ua)
			require.Equal(t, tc.want, DetectRequestClient(h))
		})
	}
	require.Equal(t, "codex-desktop", DetectRequestClient(http.Header{"Originator": {"CODEX_DESKTOP"}}))
	require.Equal(t, "codex-vscode", DetectRequestClient(http.Header{"Originator": {"codex_vscode"}}))
	require.Equal(t, "codex-tui", DetectRequestClient(http.Header{"Originator": {"codex_cli_rs"}}))
	require.Empty(t, DetectRequestClient(nil))
}

func TestRequestClientConfiguration(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, UpdateRequestClientRules(DefaultRequestClientRules)) })
	config := `[ {"id":"custom","name":"Custom App","icon":"/custom.svg","url":"https://example.com","enabled":true,"rules":[{"header":"X-App","pattern":"(?i)^custom$"}]} ]`
	require.NoError(t, UpdateRequestClientRules(config))
	require.Equal(t, "custom", DetectRequestClient(http.Header{"X-App": {"CUSTOM"}}))
	require.Empty(t, DetectRequestClient(http.Header{"User-Agent": {"codex/1.0"}}))
	require.Equal(t, "Custom App", GetRequestClientDisplay()[0].Name)
	for _, value := range []string{`null`, `{}`, `[{}]`, `[ {"id":"a","name":"A","icon":"codex","url":"javascript:alert(1)","rules":[{"header":"User-Agent","pattern":"a"}]} ]`, `[ {"id":"a","name":"A","icon":"codex","rules":[{"header":"User-Agent","pattern":"["}]} ]`} {
		require.Error(t, UpdateRequestClientRules(value))
		require.Equal(t, "custom", DetectRequestClient(http.Header{"X-App": {"custom"}}))
	}
	require.NoError(t, UpdateRequestClientRules(`[]`))
	require.Empty(t, GetRequestClientDisplay())
	require.Empty(t, DetectRequestClient(http.Header{"X-App": {"CUSTOM"}}))
}

func TestRequestClientOrderAndDisabledRules(t *testing.T) {
	t.Cleanup(func() { require.NoError(t, UpdateRequestClientRules(DefaultRequestClientRules)) })
	configs := []RequestClientConfig{
		{RequestClientDisplay: RequestClientDisplay{ID: "disabled", Name: "Disabled", Icon: "pi"}, Enabled: false, Rules: []RequestClientRule{{Header: "User-Agent", Pattern: "custom"}}},
		{RequestClientDisplay: RequestClientDisplay{ID: "first", Name: "First", Icon: "pi"}, Enabled: true, Rules: []RequestClientRule{{Header: "User-Agent", Pattern: "custom"}}},
		{RequestClientDisplay: RequestClientDisplay{ID: "second", Name: "Second", Icon: "pi"}, Enabled: true, Rules: []RequestClientRule{{Header: "User-Agent", Pattern: "custom"}}},
	}
	data, err := Marshal(configs)
	require.NoError(t, err)
	require.NoError(t, UpdateRequestClientRules(string(data)))
	require.Equal(t, "first", DetectRequestClient(http.Header{"User-Agent": {"custom/1.0"}}))
	require.Len(t, GetRequestClientDisplay(), 2)
	configs[2].ID = "first"
	data, err = Marshal(configs)
	require.NoError(t, err)
	require.Error(t, CheckRequestClientRules(string(data)))
}
