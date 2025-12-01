package strategy

import "fmt"

type Registry struct {
	strategies map[string]Strategy
}

func NewRegistry() *Registry {
	return &Registry{
		strategies: make(map[string]Strategy),
	}
}

func (r *Registry) Register(strategy Strategy) {
	key := fmt.Sprintf("%s-%s", strategy.GetName(), strategy.GetMode())
	r.strategies[key] = strategy
}

func (r *Registry) GetStrategy(name string, mode string) (*Strategy, bool) {
	key := fmt.Sprintf("%s-%s", name, mode)
	strategy, ok := r.strategies[key]
	return &strategy, ok
}

func (r *Registry) GetStrategyList() []Strategy {
	strategies := make([]Strategy, 0, len(r.strategies))
	for _, strategy := range r.strategies {
		strategies = append(strategies, strategy)
	}
	return strategies
}
