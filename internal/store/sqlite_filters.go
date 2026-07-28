package store

import (
	"encoding/json"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"go.uber.org/zap"
)

func (s *SQLiteStore) FolderFilters() []domain.FolderFilter {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM folder_filters WHERE key = 'v1'`).Scan(&data)
	if err != nil {
		return nil
	}
	var filters []domain.FolderFilter
	if err := json.Unmarshal(data, &filters); err != nil {
		return nil
	}
	return filters
}

func (s *SQLiteStore) SetFolderFilters(filters []domain.FolderFilter) {
	data, err := json.Marshal(filters)
	if err != nil {
		s.log.Error("marshal folder filters failed", zap.Error(err))
		return
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO folder_filters (key, data) VALUES ('v1', ?)`, data)
	if err != nil {
		s.log.Error("persist folder filters failed", zap.Error(err))
	}
}
