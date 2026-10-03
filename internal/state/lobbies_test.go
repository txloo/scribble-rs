package state

import (
	"testing"

	"github.com/lxzan/gws"
	"github.com/scribble-rs/scribble.rs/internal/config"
	"github.com/scribble-rs/scribble.rs/internal/game"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest //this test is very stateful
func TestAddAndRemove(t *testing.T) {
	require.Empty(t, lobbies, "Lobbies should be empty when test starts")

	createLobby := func() *game.Lobby {
		player, lobby, err := game.CreateLobby("", "player", "dutch", &game.EditableLobbySettings{
			Public:             true,
			DrawingTime:        100,
			Rounds:             10,
			MaxPlayers:         10,
			CustomWordsPerTurn: 3,
			ClientsPerIPLimit:  1,
			WordsPerTurn:       3,
		}, nil, game.ChillScoring)
		require.NoError(t, err)
		lobby.OnPlayerDisconnect(player)
		return lobby
	}
	lobbyA := createLobby()
	lobbyB := createLobby()
	lobbyC := createLobby()

	AddLobby(lobbyA)
	AddLobby(lobbyB)
	AddLobby(lobbyC)

	require.NotNil(t, GetLobby(lobbyA.LobbyID))
	require.NotNil(t, GetLobby(lobbyB.LobbyID))
	require.NotNil(t, GetLobby(lobbyC.LobbyID))

	RemoveLobby(lobbyB.LobbyID)
	require.Nil(t, GetLobby(lobbyB.LobbyID), "Lobby B should have been deleted.")

	require.NotNil(t, GetLobby(lobbyA.LobbyID), "Lobby A shouldn't have been deleted.")
	require.NotNil(t, GetLobby(lobbyC.LobbyID), "Lobby C shouldn't have been deleted.")
	require.Len(t, lobbies, 2)

	cleanupRoutineLogic(&config.LobbyCleanup{})
	require.Empty(t, lobbies)
}

//nolint:paralleltest //this test is very stateful
func TestCloseLobby(t *testing.T) {
	require.Empty(t, lobbies, "Lobbies should be empty when test starts")

	createLobby := func() *game.Lobby {
		player, lobby, err := game.CreateLobby("", "player", "dutch", &game.EditableLobbySettings{
			Public:             true,
			DrawingTime:        100,
			Rounds:             10,
			MaxPlayers:         10,
			CustomWordsPerTurn: 3,
			ClientsPerIPLimit:  1,
			WordsPerTurn:       3,
		}, nil, game.ChillScoring)
		require.NoError(t, err)
		lobby.OnPlayerDisconnect(player)
		return lobby
	}

	var broadcastCount int
	lobbyA := createLobby()
	lobbyA.WritePreparedMessage = func(_ *game.Player, _ *gws.Broadcaster) error {
		broadcastCount++
		return nil
	}
	lobbyB := createLobby()

	AddLobby(lobbyA)
	AddLobby(lobbyB)

	require.False(t, CloseLobby("unknown-lobby-id"), "Closing an unknown lobby should fail.")

	require.True(t, CloseLobby(lobbyA.LobbyID), "Closing an existing lobby should succeed.")
	require.Nil(t, GetLobby(lobbyA.LobbyID), "Lobby A should have been deleted.")
	require.Positive(t, broadcastCount, "Players should have been notified about the closure.")

	// Closing the same lobby again can't notify anyone, but must not panic.
	require.False(t, CloseLobby(lobbyA.LobbyID), "The lobby is already gone.")

	require.NotNil(t, GetLobby(lobbyB.LobbyID), "Lobby B shouldn't have been deleted.")
	require.Len(t, lobbies, 1)

	RemoveLobby(lobbyB.LobbyID)
	require.Empty(t, lobbies)
}
