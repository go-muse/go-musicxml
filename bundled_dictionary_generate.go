package musicxml

// Generation only verifies and copies committed pinned inputs. Upstream refresh
// is a separate reviewed maintenance action, never part of this command.
//go:generate go run ./internal/dictionarygen/cmd/dictionarygen
