package pipeline

import (
	"fmt"
	"strings"
)

type ProgressStage string

const (
	ProgressStageSource    ProgressStage = "source"
	ProgressStageChunks    ProgressStage = "chunks"
	ProgressStageCache     ProgressStage = "cache"
	ProgressStageTranslate ProgressStage = "translate"
	ProgressStageBuild     ProgressStage = "build"
)

type ProgressReporter func(ProgressEvent)

type ProgressEvent struct {
	Stage   ProgressStage
	Message string
	Current int
	Total   int
}

func (e ProgressEvent) Line() string {
	if e.Total <= 0 {
		return e.Message
	}
	current := clamp(e.Current, 0, e.Total)
	percent := current * 100 / e.Total
	return fmt.Sprintf("%s %3d%% %s", progressBar(current, e.Total, 20), percent, e.Message)
}

func progressBar(current, total, width int) string {
	if total <= 0 || width <= 0 {
		return "[]"
	}
	current = clamp(current, 0, total)
	filled := current * width / total
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
