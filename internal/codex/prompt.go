package codex

import (
	"fmt"
	"strings"

	"github.com/flowkater/ax/internal/core"
)

// BuildStepPrompt creates a deterministic prompt for one run step.
func BuildStepPrompt(step string, planBody string, ctx core.BuiltContextLayers) string {
	var b strings.Builder
	b.WriteString("AX RUN STEP\n")
	b.WriteString(fmt.Sprintf("step: %s\n", strings.TrimSpace(step)))
	b.WriteString("plan:\n")
	b.WriteString(strings.TrimSpace(planBody))
	b.WriteString("\n\ncontext.protocol: ")
	b.WriteString(strings.Join(ctx.Protocol, ","))
	b.WriteString("\ncontext.task: ")
	b.WriteString(strings.Join(ctx.TaskScoped, ","))
	b.WriteString("\ncontext.session: ")
	b.WriteString(strings.Join(ctx.SessionMem, ","))
	return b.String()
}

// BuildTDDStepPrompt creates a deterministic TDD-oriented prompt.
func BuildTDDStepPrompt(tier string, phase string, planBody string) string {
	tier = strings.TrimSpace(tier)
	if tier == "" {
		tier = "T0"
	}
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = "red"
	}
	return fmt.Sprintf("AX TDD STEP\ntier: %s\nphase: %s\nplan:\n%s", tier, phase, strings.TrimSpace(planBody))
}
