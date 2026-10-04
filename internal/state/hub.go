package state

import (
	"fmt"
	"log"
	"sync"

	"github.com/scribble-rs/scribble.rs/internal/config"
	"github.com/scribble-rs/scribble.rs/internal/game"
)

var (
	hubMutex = &sync.Mutex{}
	hubLobby *game.Lobby
)

// EnsureHubLobby creates the one permanent room behind the home page, unless
// it already exists (e.g. because EnsureHubLobby was called before).
// Settings are taken from the configured lobby setting defaults.
// The home page is fully socketless: it never opens a websocket, so socket
// pushes are not needed and the lobby's WriteObject is set to a no-op to
// guard Broadcast calls against a nil function field.
func EnsureHubLobby(cfg *config.Config) *game.Lobby {
	hubMutex.Lock()
	defer hubMutex.Unlock()

	if hubLobby != nil {
		return hubLobby
	}

	// An unknown configured language would crash on the lowercaser lookup,
	// so we fall back to English in that case.
	language := cfg.LobbySettingDefaults.Language
	if _, exists := game.WordlistData[language]; !exists {
		log.Printf("Unknown wordpack language '%s', falling back to 'english'.\n", language)
		language = "english"
	}

	lobbySettings := parseHubLobbySettings(cfg)

	_, lobby, err := game.CreateLobby("", game.GeneratePlayername(),
		language, lobbySettings, nil, parseHubScoreCalculation(cfg))
	if err != nil {
		log.Fatalf("error creating the home page room: %s", err)
	}
	lobby.IsHub = true
	lobby.WriteObject = func(player *game.Player, object any) error { return nil }

	AddLobby(lobby)
	hubLobby = lobby

	log.Printf("Home page room created with id '%s'.\n", lobby.LobbyID)
	return lobby
}

// HubLobby returns the permanent room, or nil if it hasn't been created yet.
func HubLobby() *game.Lobby {
	hubMutex.Lock()
	defer hubMutex.Unlock()

	return hubLobby
}

// parseHubLobbySettings converts the configured lobby setting defaults into
// editable settings, falling back to sane values in case the configuration
// contains illegal values.
func parseHubLobbySettings(cfg *config.Config) *game.EditableLobbySettings {
	defaults := cfg.LobbySettingDefaults
	bounds := cfg.LobbySettingBounds

	return &game.EditableLobbySettings{
		Public: defaults.Public == "true",
		MaxPlayers: parseHubInt(defaults.MaxPlayers, 24,
			bounds.MinMaxPlayers, bounds.MaxMaxPlayers),
		// The maximum amount of custom words per turn shares its bound with
		// the general maximum amount of words per turn.
		CustomWordsPerTurn: parseHubInt(defaults.CustomWordsPerTurn, 3,
			bounds.MinCustomWordsPerTurn, bounds.MaxWordsPerTurn),
		Rounds:      parseHubInt(defaults.Rounds, 4, bounds.MinRounds, bounds.MaxRounds),
		DrawingTime:  parseHubInt(defaults.DrawingTime, 120, bounds.MinDrawingTime, bounds.MaxDrawingTime),
		WordsPerTurn: parseHubInt(defaults.WordsPerTurn, 3, bounds.MinWordsPerTurn, bounds.MaxWordsPerTurn),
	}
}

// parseHubScoreCalculation resolves the configured scoring calculation,
// falling back to the chill scoring algorithm.
func parseHubScoreCalculation(cfg *config.Config) game.ScoreCalculation {
	switch cfg.LobbySettingDefaults.ScoreCalculation {
	case "competitive":
		return game.CompetitiveScoring
	default:
		return game.ChillScoring
	}
}

// parseHubInt parses an integer setting, clamping it to the given bounds. On
// any error the fallback value is returned, since all of these are deployment
// settings that shouldn't crash the server.
func parseHubInt(value string, fallback, minimum, maximum int) int {
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return clampHubValue(fallback, minimum, maximum)
	}

	return clampHubValue(parsed, minimum, maximum)
}

func clampHubValue(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
