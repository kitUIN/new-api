package common

import (
	"strconv"
	"strings"
)

var (
	// OpenAIResponseOnlyModels is a list of models that are only available for OpenAI responses.
	OpenAIResponseOnlyModels = []string{
		"o3-pro",
		"o3-deep-research",
		"o4-mini-deep-research",
	}
	ImageGenerationModels = []string{
		"dall-e-3",
		"dall-e-2",
		"gpt-image-1",
		"prefix:imagen-",
		"flux-",
		"flux.1-",
	}
	OpenAITextModels = []string{
		"gpt-",
		"o1",
		"o3",
		"o4",
		"chatgpt",
	}
)

func IsOpenAIResponseOnlyModel(modelName string) bool {
	for _, m := range OpenAIResponseOnlyModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

func UsesTurnScopedResponsesInstructions(modelName string) bool {
	modelName = strings.TrimPrefix(strings.ToLower(modelName), "openai/")
	version, found := strings.CutPrefix(modelName, "gpt-")
	if !found {
		return false
	}
	version, _, _ = strings.Cut(version, "-")
	majorText, minorText, hasMinor := strings.Cut(version, ".")
	major, err := strconv.ParseUint(majorText, 10, 32)
	if err != nil || !hasMinor {
		return err == nil && major > 6
	}
	minor, err := strconv.ParseUint(minorText, 10, 32)
	return err == nil && (major > 6 || major == 6 && minor >= 2)
}

func IsImageGenerationModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range ImageGenerationModels {
		if strings.Contains(modelName, m) {
			return true
		}
		if strings.HasPrefix(m, "prefix:") && strings.HasPrefix(modelName, strings.TrimPrefix(m, "prefix:")) {
			return true
		}
	}
	return false
}

func IsOpenAITextModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range OpenAITextModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}
