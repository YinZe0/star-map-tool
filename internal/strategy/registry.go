package strategy

type Registry struct {
	strategies map[string]Strategy
}

func NewRegistry() *Registry {
	return &Registry{
		strategies: make(map[string]Strategy),
	}
}

func (r *Registry) Register(strategy Strategy) {
	r.strategies[strategy.GetName()] = strategy
}

func (r *Registry) GetStrategy(name string) (*Strategy, bool) {
	strategy, ok := r.strategies[name]
	return &strategy, ok
}

func (r *Registry) GetStrategyList() []Strategy {
	strategies := make([]Strategy, 0, len(r.strategies))
	for _, strategy := range r.strategies {
		strategies = append(strategies, strategy)
	}
	return strategies
}
