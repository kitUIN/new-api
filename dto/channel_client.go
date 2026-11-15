package dto

import "fmt"

type ChannelClientAccess string

const (
	ChannelClientAccessUnrestricted ChannelClientAccess = "unrestricted"
	ChannelClientAccessDeny         ChannelClientAccess = "deny"
	ChannelClientAccessAllowOnly    ChannelClientAccess = "allow_only"
)

type ChannelClientRestrictions struct {
	Codex      ChannelClientAccess `json:"codex,omitempty"`
	ClaudeCode ChannelClientAccess `json:"claude_code,omitempty"`
}

func (r ChannelClientRestrictions) Validate() error {
	for _, rule := range []struct {
		client string
		access ChannelClientAccess
	}{{"codex", r.Codex}, {"claude_code", r.ClaudeCode}} {
		switch rule.access {
		case "", ChannelClientAccessUnrestricted, ChannelClientAccessDeny, ChannelClientAccessAllowOnly:
		default:
			return fmt.Errorf("invalid client_restrictions.%s: %q", rule.client, rule.access)
		}
	}
	return nil
}

// Explicit rules replace the legacy switch, including an empty rules object.
func (s ChannelOtherSettings) GetClientRestrictions() ChannelClientRestrictions {
	if s.ClientRestrictions != nil {
		return *s.ClientRestrictions
	}
	if s.DisableClaudeCode {
		return ChannelClientRestrictions{ClaudeCode: ChannelClientAccessDeny}
	}
	return ChannelClientRestrictions{}
}
