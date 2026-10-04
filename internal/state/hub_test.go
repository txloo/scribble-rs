package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/scribble-rs/scribble.rs/internal/config"
	"github.com/scribble-rs/scribble.rs/internal/game"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest //this test is very stateful
func TestEnsureHubLobby(t *testing.T) {
	ResetHubState()
	t.Cleanup(ResetHubState)

	hub := EnsureHubLobby(&config.Default)
	require.True(t, hub.IsHub, "The hub lobby must be marked as hub.")

	hubAgain := EnsureHubLobby(&config.Default)
	require.Same(t, hub, hubAgain, "EnsureHubLobby should be idempotent.")

	// Break the lobby on purpose, so we can test the re-creation.
	hubMutex.Lock()
	RemoveLobby(hub.LobbyID)
	hubLobby = nil
	hubMutex.Unlock()

	recreatedHub := EnsureHubLobby(&config.Default)
	require.NotNil(t, recreatedHub)
	require.NotSame(t, hub, recreatedHub,
		"A missing hub must be re-created by EnsureHubLobby.")
	require.True(t, recreatedHub.IsHub)
}

// ResetHubState resets all hub related state. This is used by tests to
// create a clean slate.
func ResetHubState() {
	hubMutex.Lock()
	defer hubMutex.Unlock()

	if hubLobby != nil {
		RemoveLobby(hubLobby.LobbyID)
	}
	hubLobby = nil
}

//nolint:paralleltest //this test is very stateful
func TestWallPersistAndLoad(t *testing.T) {
	ResetHubState()
	t.Cleanup(ResetHubState)

	lobby := EnsureHubLobby(&config.Default)

	wallFile := filepath.Join(t.TempDir(), "subdir", "wall.json")

	lineEvent := &game.LineEvent{
		Type: game.EventTypeLine,
		Data: game.LineEventData{X: 10, Y: 20, X2: 30, Y2: 40, Color: 1, Width: 8},
	}
	fillEvent := &game.FillEvent{
		Type: game.EventTypeFill,
		Data: game.FillEventData{X: 50, Y: 60, Color: 2},
	}

	// LoadWall stands for the boot-time load; it must leave the dirty flag
	// cleared, since nothing was changed yet.
	lobby.LoadWall([]any{lineEvent, fillEvent})
	require.False(t, lobby.WallDirty(), "LoadWall should reset the dirty flag.")
	require.Len(t, lobby.WallSnapshot(), 2)

	// Simulate a wall change through the regular draw-event path.
	lineJSON, err := json.Marshal(lineEvent)
	require.NoError(t, err)
	require.True(t, lobby.AppendWallEvent(lineJSON, game.EventTypeLine, nil))
	require.True(t, lobby.WallDirty(), "The wall should be dirty after a change.")

	require.NoError(t, SaveWallNow(wallFile, lobby))
	require.False(t, lobby.WallDirty(), "Saving should reset the dirty flag.")

	// Simulate a reboot: a fresh lobby loads the persisted wall.
	_, rebootedLobby, err := game.CreateLobby("", "player", "english",
		&game.EditableLobbySettings{
			Public: false, DrawingTime: 80, Rounds: 4, MaxPlayers: 8,
			CustomWordsPerTurn: 3, ClientsPerIPLimit: 2, WordsPerTurn: 3,
		}, nil, game.ChillScoring)
	require.NoError(t, err)
	rebootedLobby.WriteObject = func(player *game.Player, object any) error { return nil }

	reloadedLobby := rebootedLobby
	require.NoError(t, loadWall(wallFile, reloadedLobby))

	snapshot := reloadedLobby.WallSnapshot()
	require.Len(t, snapshot, 3, "Two seeded events plus one appended event should be persisted.")

	reloadedLine, isLine := snapshot[0].(*game.LineEvent)
	require.True(t, isLine, "Line events should survive the round trip as typed events.")
	require.Equal(t, lineEvent.Data, reloadedLine.Data)

	reloadedFill, isFill := snapshot[1].(*game.FillEvent)
	require.True(t, isFill, "Fill events should survive the round trip as typed events.")
	require.Equal(t, fillEvent.Data, reloadedFill.Data)

	require.NoError(t, os.Remove(wallFile))
}

//nolint:paralleltest //this test is very stateful
func TestHubPlayerPruning(t *testing.T) {
	ResetHubState()
	t.Cleanup(ResetHubState)

	hub := EnsureHubLobby(&config.Default)

	hub.Synchronized(func() {
		player := hub.JoinPlayer("prune-me")
		// The player never interacts (socketless), so their lastSeen stays
		// in the past relative to a zero-duration threshold.
		_ = player
	})

	require.Equal(t, 2, hub.PruneIdlePlayers(0),
		"Both the auto-generated hub player and the joined player should be pruned with a zero threshold.")

	// The hub itself must survive pruning attempts of the cleanup routine.
	hub.Synchronized(func() {
		require.Empty(t, hub.GetPlayers())
	})
}
