package app

import (
	"aurora/internal/core"
	"aurora/internal/gateway"
	"aurora/internal/model_aliases"
	"aurora/internal/model_combinations"
	"aurora/internal/server"
)

func requestModelResolver(aliasService *aliases.Service, comboService *combos.Service) gateway.ModelResolver {
	if comboService != nil {
		return comboService
	}
	return aliasService
}

func exposedModelLister(aliasService *aliases.Service, comboService *combos.Service) server.ExposedModelLister {
	listers := make([]server.ExposedModelLister, 0, 2)
	if aliasService != nil {
		listers = append(listers, aliasService)
	}
	if comboService != nil {
		listers = append(listers, comboService)
	}
	return compositeExposedModelLister{listers: listers}
}

type compositeExposedModelLister struct {
	listers []server.ExposedModelLister
}

func (l compositeExposedModelLister) ExposedModels() []core.Model {
	out := make([]core.Model, 0)
	for _, lister := range l.listers {
		if lister != nil {
			out = append(out, lister.ExposedModels()...)
		}
	}
	return out
}
