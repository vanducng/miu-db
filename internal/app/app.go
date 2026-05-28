package app

import (
	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

type App struct {
	Store     *config.Store
	Registry  *adapter.Registry
	PageStore *result.PageStore
}

func New(store *config.Store, registry *adapter.Registry, pageStore *result.PageStore) *App {
	return &App{Store: store, Registry: registry, PageStore: pageStore}
}
