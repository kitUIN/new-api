package gemini

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const maxGeminiPartialArgArrayIndex = 4095

type geminiPartialFunctionCallState struct {
	byCandidate map[int64]*geminiPartialToolCall
}

type geminiPartialToolCall struct {
	id        string
	name      string
	arguments map[string]interface{}
}

type geminiPartialArgPathSegment struct {
	member  string
	index   int
	isIndex bool
}

func newGeminiPartialFunctionCallState() *geminiPartialFunctionCallState {
	return &geminiPartialFunctionCallState{byCandidate: make(map[int64]*geminiPartialToolCall)}
}

func (s *geminiPartialFunctionCallState) prepare(response *dto.GeminiChatResponse) (*dto.GeminiChatResponse, error) {
	if s == nil || response == nil {
		return response, nil
	}
	prepared := *response
	prepared.Candidates = append([]dto.GeminiChatCandidate(nil), response.Candidates...)
	for candidateIndex := range prepared.Candidates {
		candidate := &prepared.Candidates[candidateIndex]
		parts := make([]dto.GeminiPart, 0, len(candidate.Content.Parts))
		for _, part := range candidate.Content.Parts {
			call := part.FunctionCall
			if call == nil || (s.byCandidate[candidate.Index] == nil && call.WillContinue == nil && len(call.PartialArgs) == 0) {
				parts = append(parts, part)
				continue
			}
			completed, ready, err := s.append(candidate.Index, call)
			if err != nil {
				return nil, fmt.Errorf("reconstruct Gemini streamed function arguments: %w", err)
			}
			if ready {
				part.FunctionCall = completed
				parts = append(parts, part)
			}
		}
		candidate.Content.Parts = parts
	}
	return &prepared, nil
}

func (s *geminiPartialFunctionCallState) append(candidateIndex int64, call *dto.FunctionCall) (*dto.FunctionCall, bool, error) {
	current := s.byCandidate[candidateIndex]
	if current == nil {
		current = &geminiPartialToolCall{arguments: make(map[string]interface{})}
		s.byCandidate[candidateIndex] = current
	}
	if id := strings.TrimSpace(call.ID); id != "" {
		if current.id != "" && current.id != id {
			return nil, false, fmt.Errorf("candidate %d function call changed id from %q to %q", candidateIndex, current.id, id)
		}
		current.id = id
	}
	if name := strings.TrimSpace(call.FunctionName); name != "" {
		if current.name != "" && current.name != name {
			return nil, false, fmt.Errorf("candidate %d function call changed name from %q to %q", candidateIndex, current.name, name)
		}
		current.name = name
	}
	for _, partial := range call.PartialArgs {
		path, err := parseGeminiPartialArgPath(partial.JSONPath)
		if err != nil {
			return nil, false, err
		}
		value, present := geminiPartialArgValue(partial)
		if !present {
			continue
		}
		updated, err := setGeminiPartialArgValue(current.arguments, path, value, partial.StringValue != nil)
		if err != nil {
			return nil, false, fmt.Errorf("set partial argument %q: %w", partial.JSONPath, err)
		}
		arguments, ok := updated.(map[string]interface{})
		if !ok {
			return nil, false, fmt.Errorf("partial argument path %q replaced the arguments object", partial.JSONPath)
		}
		current.arguments = arguments
	}
	if call.WillContinue != nil && *call.WillContinue {
		return nil, false, nil
	}
	if current.name == "" {
		return nil, false, fmt.Errorf("candidate %d completed a partial function call without a name", candidateIndex)
	}
	completed := &dto.FunctionCall{ID: current.id, FunctionName: current.name, Arguments: current.arguments}
	delete(s.byCandidate, candidateIndex)
	return completed, true, nil
}

func (s *geminiPartialFunctionCallState) validateComplete() error {
	if s == nil || len(s.byCandidate) == 0 {
		return nil
	}
	indexes := make([]int64, 0, len(s.byCandidate))
	for index := range s.byCandidate {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
	index := indexes[0]
	partial := s.byCandidate[index]
	return fmt.Errorf("Gemini stream ended with an incomplete function call for candidate %d (id %q, name %q)", index, partial.id, partial.name)
}

func parseGeminiPartialArgPath(jsonPath string) ([]geminiPartialArgPathSegment, error) {
	path := strings.TrimSpace(jsonPath)
	if path == "" || path[0] != '$' {
		return nil, fmt.Errorf("unsupported Gemini partial argument path %q", jsonPath)
	}
	segments := make([]geminiPartialArgPathSegment, 0)
	for offset := 1; offset < len(path); {
		switch path[offset] {
		case '.':
			offset++
			start := offset
			for offset < len(path) && path[offset] != '.' && path[offset] != '[' {
				offset++
			}
			if start == offset {
				return nil, fmt.Errorf("empty member in Gemini partial argument path %q", jsonPath)
			}
			member := path[start:offset]
			if strings.ContainsAny(member, "]*?") {
				return nil, fmt.Errorf("unsupported member %q in Gemini partial argument path", member)
			}
			segments = append(segments, geminiPartialArgPathSegment{member: member})
		case '[':
			offset++
			if offset >= len(path) {
				return nil, fmt.Errorf("unterminated selector in Gemini partial argument path %q", jsonPath)
			}
			if path[offset] == '\'' || path[offset] == '"' {
				member, next, err := parseGeminiPartialArgMember(path, offset)
				if err != nil {
					return nil, fmt.Errorf("invalid Gemini partial argument path %q: %w", jsonPath, err)
				}
				offset = next
				if offset >= len(path) || path[offset] != ']' {
					return nil, fmt.Errorf("unterminated member selector in Gemini partial argument path %q", jsonPath)
				}
				offset++
				segments = append(segments, geminiPartialArgPathSegment{member: member})
				continue
			}
			start := offset
			for offset < len(path) && path[offset] >= '0' && path[offset] <= '9' {
				offset++
			}
			if start == offset || offset >= len(path) || path[offset] != ']' {
				return nil, fmt.Errorf("unsupported selector in Gemini partial argument path %q", jsonPath)
			}
			index, err := strconv.Atoi(path[start:offset])
			if err != nil {
				return nil, fmt.Errorf("invalid array index in Gemini partial argument path %q: %w", jsonPath, err)
			}
			if index > maxGeminiPartialArgArrayIndex {
				return nil, fmt.Errorf("array index %d exceeds Gemini partial argument materialization limit %d", index, maxGeminiPartialArgArrayIndex)
			}
			offset++
			segments = append(segments, geminiPartialArgPathSegment{index: index, isIndex: true})
		default:
			return nil, fmt.Errorf("unsupported selector at offset %d in Gemini partial argument path %q", offset, jsonPath)
		}
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("Gemini partial argument path %q targets the arguments root", jsonPath)
	}
	return segments, nil
}

func geminiPartialArgValue(partial dto.GeminiPartialArg) (any, bool) {
	switch {
	case partial.StringValue != nil:
		return *partial.StringValue, true
	case partial.NumberValue != nil:
		return *partial.NumberValue, true
	case partial.BoolValue != nil:
		return *partial.BoolValue, true
	case partial.NullValue != nil:
		return nil, true
	default:
		return nil, false
	}
}

func parseGeminiPartialArgMember(path string, offset int) (string, int, error) {
	quote := path[offset]
	start := offset
	offset++
	for offset < len(path) {
		if path[offset] == '\\' {
			offset += 2
			continue
		}
		if path[offset] == quote {
			raw := path[start : offset+1]
			if quote == '\'' {
				raw = `"` + strings.ReplaceAll(strings.ReplaceAll(raw[1:len(raw)-1], `"`, `\"`), `\'`, `'`) + `"`
			}
			var member string
			if err := common.Unmarshal([]byte(raw), &member); err != nil {
				return "", 0, err
			}
			return member, offset + 1, nil
		}
		offset++
	}
	return "", 0, fmt.Errorf("unterminated quoted member")
}

func setGeminiPartialArgValue(current any, path []geminiPartialArgPathSegment, value any, appendString bool) (any, error) {
	if len(path) == 0 {
		if appendString {
			if existing, ok := current.(string); ok {
				return existing + value.(string), nil
			}
		}
		return value, nil
	}
	segment := path[0]
	if segment.isIndex {
		var array []interface{}
		switch typed := current.(type) {
		case nil:
			array = make([]interface{}, segment.index+1)
		case []interface{}:
			array = typed
			if len(array) <= segment.index {
				array = append(array, make([]interface{}, segment.index-len(array)+1)...)
			}
		default:
			return nil, fmt.Errorf("array index %d traverses %T", segment.index, current)
		}
		updated, err := setGeminiPartialArgValue(array[segment.index], path[1:], value, appendString)
		if err != nil {
			return nil, err
		}
		array[segment.index] = updated
		return array, nil
	}

	var object map[string]interface{}
	switch typed := current.(type) {
	case nil:
		object = make(map[string]interface{})
	case map[string]interface{}:
		object = typed
	default:
		return nil, fmt.Errorf("member %q traverses %T", segment.member, current)
	}
	updated, err := setGeminiPartialArgValue(object[segment.member], path[1:], value, appendString)
	if err != nil {
		return nil, err
	}
	object[segment.member] = updated
	return object, nil
}
