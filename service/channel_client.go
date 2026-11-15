package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// IsClaudeCodeRequest recognizes client headers without inspecting user prompts.
func IsClaudeCodeRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	for _, product := range strings.Fields(strings.ToLower(c.Request.UserAgent())) {
		name, _, _ := strings.Cut(product, "/")
		switch name {
		case "claude-cli", "claude-code", "claudecode":
			return true
		}
	}
	// Some proxies replace User-Agent but retain Claude Code's application and
	// beta headers. Neither a generic CLI marker nor an SDK header alone suffices.
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("X-App")), "cli") {
		for _, value := range c.Request.Header.Values("Anthropic-Beta") {
			for _, beta := range strings.Split(value, ",") {
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(beta)), "claude-code-") {
					return true
				}
			}
		}
	}
	return false
}

// IsCodexRequest recognizes Codex products and their forwarded originator header.
func IsCodexRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	isCodexProduct := func(name string) bool {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "codex", "codex-cli", "codex_cli_rs", "codex_exec", "codex_vscode", "codex_desktop", "codex_app", "codex cli":
			return true
		}
		return false
	}
	for _, product := range strings.Fields(c.Request.UserAgent()) {
		name, _, _ := strings.Cut(product, "/")
		if isCodexProduct(name) {
			return true
		}
	}
	for _, originator := range c.Request.Header.Values("Originator") {
		if isCodexProduct(originator) {
			return true
		}
	}
	return false
}

func IsChannelBlockedForClient(c *gin.Context, channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	rules := channel.GetOtherSettings().GetClientRestrictions()
	isCodex, isClaudeCode := IsCodexRequest(c), IsClaudeCodeRequest(c)
	// A deny rule wins when a request advertises both client identities.
	if (isCodex && rules.Codex == dto.ChannelClientAccessDeny) ||
		(isClaudeCode && rules.ClaudeCode == dto.ChannelClientAccessDeny) {
		return true
	}
	onlyCodex := rules.Codex == dto.ChannelClientAccessAllowOnly
	onlyClaudeCode := rules.ClaudeCode == dto.ChannelClientAccessAllowOnly
	return (onlyCodex || onlyClaudeCode) && !(onlyCodex && isCodex || onlyClaudeCode && isClaudeCode)
}

// Client restrictions are permanent for this selection, unlike failed-channel
// exclusions, which retry logic may release after exhausting other candidates.
func getRandomChannelForRequest(c *gin.Context, group, modelName string, retry int, excludedChannelIDs []int) (*model.Channel, error) {
	excluded := append([]int(nil), excludedChannelIDs...)
	for {
		channel, err := model.GetRandomSatisfiedChannelWithExclusions(group, modelName, retry, excluded)
		if err != nil || channel == nil || !IsChannelBlockedForClient(c, channel) {
			return channel, err
		}
		excluded = append(excluded, channel.Id)
	}
}
