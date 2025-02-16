package bolt

import (
	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

type settingsBackend struct {
	db *storm.DB
}

func (s settingsBackend) Get() (*settings.Settings, error) {
	set := &settings.Settings{}
	return set, get(s.db, "settings", set)
}

func (s settingsBackend) Save(set *settings.Settings) error {
	return Save(s.db, "settings", set)
}

func (s settingsBackend) GetServer() (*settings.Server, error) {
	server := &settings.Server{
		Port:               80,
		NumImageProcessors: 1,
	}
	return server, get(s.db, "server", server)
}

func (s settingsBackend) SaveServer(server *settings.Server) error {
	return Save(s.db, "server", server)
}
