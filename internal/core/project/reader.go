package project

import "github.com/sorokin-vladimir/tele/internal/domain"

// Reader is the read-only view of domain state a builder needs. store.Store
// satisfies it; a test double is a struct with four methods.
type Reader interface {
	Chats() []domain.Chat
	GetChat(id int64) (domain.Chat, bool)
	Messages(chatID int64) []domain.Message
	FolderFilters() []domain.FolderFilter
}
