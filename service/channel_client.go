package service

import (
	"strings"

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

func IsChannelBlockedForClient(c *gin.Context, channel *model.Channel) bool {
	return channel != nil && IsClaudeCodeRequest(c) && channel.GetOtherSettings().DisableClaudeCode
}

// Client restrictions are permanent for this selection, unlike failed-channel
// exclusions, which retry logic may release after exhausting other candidates.
func getRandomChannelForRequest(c *gin.Context, group, modelName string, retry int, excludedChannelIDs []int) (*model.Channel, error) {
	if !IsClaudeCodeRequest(c) {
		return model.GetRandomSatisfiedChannelWithExclusions(group, modelName, retry, excludedChannelIDs)
	}
	excluded := append([]int(nil), excludedChannelIDs...)
	for {
		channel, err := model.GetRandomSatisfiedChannelWithExclusions(group, modelName, retry, excluded)
		if err != nil || channel == nil || !channel.GetOtherSettings().DisableClaudeCode {
			return channel, err
		}
		excluded = append(excluded, channel.Id)
	}
}
