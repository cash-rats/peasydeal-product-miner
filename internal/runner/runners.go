package runner

import (
	"fmt"

	"go.uber.org/fx"
)

type NewRunnersParams struct {
	fx.In

	Runners []ToolRunner `group:"tool_runners"`
}

func NewRunners(p NewRunnersParams) (map[string]ToolRunner, error) {
	m := make(map[string]ToolRunner, len(p.Runners))
	for _, r := range p.Runners {
		if _, exists := m[r.Name()]; exists {
			return nil, fmt.Errorf("duplicate runner name: %s", r.Name())
		}
		m[r.Name()] = r
	}
	return m, nil
}
