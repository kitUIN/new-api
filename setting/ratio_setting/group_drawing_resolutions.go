package ratio_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// Missing groups retain support for all resolutions; an explicit empty list disables drawing.
type GroupDrawingResolutionsConfig struct {
	mu     sync.RWMutex
	groups map[string][]string
}

func (c *GroupDrawingResolutionsConfig) UnmarshalJSON(data []byte) error {
	var groups map[string][]string
	if err := common.Unmarshal(data, &groups); err != nil {
		return err
	}
	if groups == nil {
		return fmt.Errorf("绘图分辨率必须为 JSON 对象")
	}
	for group, resolutions := range groups {
		if strings.TrimSpace(group) == "" || group != strings.TrimSpace(group) {
			return fmt.Errorf("绘图分组名不能为空或包含首尾空格")
		}
		if resolutions == nil {
			return fmt.Errorf("分组 %s 的绘图分辨率必须为数组", group)
		}
		seen := map[string]bool{}
		for _, resolution := range resolutions {
			if (resolution != "1K" && resolution != "2K" && resolution != "4K") || seen[resolution] {
				return fmt.Errorf("分组 %s 的绘图分辨率无效或重复: %s", group, resolution)
			}
			seen[resolution] = true
		}
	}
	c.mu.Lock()
	c.groups = groups
	c.mu.Unlock()
	return nil
}

func (c *GroupDrawingResolutionsConfig) MarshalJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.groups == nil {
		return []byte("{}"), nil
	}
	return common.Marshal(c.groups)
}

func CheckGroupDrawingResolutions(value string) error {
	return common.Unmarshal([]byte(value), &GroupDrawingResolutionsConfig{})
}

func GroupSupportsDrawingResolution(group, resolution string) bool {
	c := groupRatioSetting.GroupDrawingResolutions
	if c == nil {
		return true
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	resolutions, configured := c.groups[group]
	return !configured || common.StringsContains(resolutions, resolution)
}

// Drawing sizes use the short edge, matching the drawing page's aspect-ratio presets.
func DrawingResolutionForSize(size string) (string, error) {
	if size == "" || size == "auto" {
		return "1K", nil
	}
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return "", fmt.Errorf("图片尺寸必须为 宽x高")
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return "", fmt.Errorf("图片尺寸必须为正整数")
	}
	if w%16 != 0 || h%16 != 0 {
		return "", fmt.Errorf("图片宽度和高度必须为 16 的倍数")
	}
	shortEdge := min(w, h)
	switch {
	case shortEdge <= 1024:
		return "1K", nil
	case shortEdge <= 2048:
		return "2K", nil
	case shortEdge <= 4096:
		return "4K", nil
	default:
		return "", fmt.Errorf("图片短边不能超过 4096 像素")
	}
}
