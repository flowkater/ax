package core

// DefaultSkillRegistryYAML returns builtin skill chain contract for ax commands.
func DefaultSkillRegistryYAML() string {
	return `propose:
  skills: [interview, openspec]
plan:
  skills: [interview, openspec, tdd-plan]
run:
  skills: [tdd-go, tdd-go-loop]
discover:
  skills: [interview, superpowers]
review:
  skills: [superpowers]
compound:
  skills: [superpowers]
`
}
