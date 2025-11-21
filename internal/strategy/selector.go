package strategy

import "log"

type Selector struct {
	registry *Registry
}

func NewSelector(registry *Registry) *Selector {
	return &Selector{
		registry: registry,
	}
}

func (s *Selector) Select(name string, mode string) *Strategy {
	strategy, ok := s.registry.GetStrategy(name)
	if !ok {
		panic("当前选择的地图尚未支持!")
	}
	log.Printf("[选择器] 当前选择的地图: %s 模式: %s\n", name, mode)
	return strategy
}
