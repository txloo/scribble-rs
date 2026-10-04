package game

import (
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest //this test is very stateful
func TestChatLogAndUpdates(t *testing.T) {
	lobby := &Lobby{}
	lobby.WriteObject = func(player *Player, object any) error { return nil }

	lobby.Synchronized(func() {
		sender := &Player{ID: mustUUID(t), Name: "marcel"}
		lobby.AppendChatMessage(EventTypeMessage, "hello", sender)
		lobby.AppendChatMessage(EventTypeMessage, "world", sender)
	})

	// A fresh client receives the whole log and bootstraps its cursor.
	messages, latestID, wallVersion := lobby.ChatUpdates(0)
	require.Len(t, messages, 2)
	require.Equal(t, uint64(2), latestID)
	require.Equal(t, uint64(0), wallVersion)
	require.Equal(t, "hello", messages[0].Content)
	require.Equal(t, "marcel", messages[0].Author)

	// A client that already saw everything receives nothing.
	messages, _, _ = lobby.ChatUpdates(latestID)
	require.Empty(t, messages)

	// Only the most recent messages are kept, so the log never grows
	// without bounds.
	lobby.Synchronized(func() {
		sender := &Player{ID: mustUUID(t), Name: "kevin"}
		for i := 0; i < chatLogCapacity+10; i++ {
			lobby.AppendChatMessage(EventTypeMessage, "spam", sender)
		}
	})

	messages, latestID, _ = lobby.ChatUpdates(0)
	require.Len(t, messages, chatLogCapacity)
	require.Len(t, lobby.chatLog, chatLogCapacity)
	require.Equal(t, "spam", messages[len(messages)-1].Content)
}

//nolint:paralleltest //this test is very stateful
func TestWallAppendAndRateLimit(t *testing.T) {
	lobby := &Lobby{}
	lobby.IsHub = true
	lobby.WriteObject = func(player *Player, object any) error { return nil }

	linePayload := []byte(`{"type":"line","data":{"x":1,"y":2,"x2":3,"y2":4,"color":1,"width":8}}`)

	sender := &Player{ID: mustUUID(t), Name: "marcel"}
	other := &Player{ID: mustUUID(t), Name: "kevin"}

	// Sessions don't throttle each other, and back-to-back strokes are
	// accepted since the throttle was removed.
	require.True(t, lobby.AppendWallEvent(linePayload, EventTypeLine, sender))
	require.True(t, lobby.AppendWallEvent(linePayload, EventTypeLine, sender))
	require.True(t, lobby.AppendWallEvent(linePayload, EventTypeLine, other))

	// The wall width is clamped like the game canvas width.
	thickPayload := []byte(`{"type":"line","data":{"x":1,"y":2,"x2":3,"y2":4,"color":1,"width":1000}}`)
	require.True(t, lobby.AppendWallEvent(thickPayload, EventTypeLine, nil))

	snapshot := lobby.WallSnapshot()
	require.Len(t, snapshot, 3)
	line, isLine := snapshot[2].(*LineEvent)
	require.True(t, isLine)
	require.Equal(t, uint8(MaxBrushSize), line.Data.Width, "Stroke width must be clamped.")

	// The wall version reflects every accepted event.
	require.Equal(t, uint64(3), lobby.WallVersion())
}

//nolint:paralleltest //this test is very stateful
func TestWallStrokeChain(t *testing.T) {
	lobby := &Lobby{}
	lobby.IsHub = true
	lobby.WriteObject = func(player *Player, object any) error { return nil }

	sender := &Player{ID: mustUUID(t), Name: "marcel"}

	chainPayload := []byte(`[
		{"type":"line","data":{"x":1,"y":2,"x2":3,"y2":4,"color":1,"width":8}},
		{"type":"line","data":{"x":3,"y":4,"x2":5,"y2":6,"color":1,"width":8}}
	]`)
	require.True(t, lobby.AppendWallEventChain(chainPayload, sender))
	require.True(t, lobby.AppendWallEventChain(chainPayload, sender),
		"Back-to-back strokes of one session must be accepted.")

	// The chain is expanded into regular line events.
	snapshot := lobby.WallSnapshot()
	require.Len(t, snapshot, 4)
	secondSegment, isLine := snapshot[3].(*LineEvent)
	require.True(t, isLine)
	require.Equal(t, int16(5), secondSegment.Data.X2)

	// Overlong chains are capped.
	longPayload := []byte("[")
	for i := 0; i < maxWallChainPoints+50; i++ {
		if i != 0 {
			longPayload = append(longPayload, ',')
		}
		longPayload = append(longPayload, []byte(
			`{"type":"line","data":{"x":1,"y":2,"x2":3,"y2":4,"color":1,"width":8}}`)...)
	}
	longPayload = append(longPayload, ']')
	require.True(t, lobby.AppendWallEventChain(longPayload, nil))
	require.Len(t, lobby.WallSnapshot(), 4+maxWallChainPoints,
		"The chain must be capped at maxWallChainPoints segments.")

	// Each segment's width is clamped.
	thickChain := []byte(
		`[{"type":"line","data":{"x":1,"y":2,"x2":3,"y2":4,"color":1,"width":1000}}]`)
	require.True(t, lobby.AppendWallEventChain(thickChain, nil))
	lastEvent := lobby.WallSnapshot()[len(lobby.WallSnapshot())-1]
	line, isLine := lastEvent.(*LineEvent)
	require.True(t, isLine)
	require.Equal(t, uint8(MaxBrushSize), line.Data.Width, "Stroke width must be clamped.")
}

func mustUUID(t *testing.T) uuid.UUID {
	t.Helper()

	id, err := uuid.NewV4()
	require.NoError(t, err)
	return id
}

//nolint:paralleltest //this test is very stateful
func TestWallUndoAndClear(t *testing.T) {
	lobby := &Lobby{}
	lobby.IsHub = true
	lobby.WriteObject = func(player *Player, object any) error { return nil }

	marcel := &Player{ID: mustUUID(t), Name: "marcel"}
	kevin := &Player{ID: mustUUID(t), Name: "kevin"}

	marcelChain := []byte(`[
		{"type":"line","data":{"x":1,"y":2,"x2":3,"y2":4,"color":1,"width":8}},
		{"type":"line","data":{"x":3,"y":4,"x2":5,"y2":6,"color":1,"width":8}}
	]`)
	kevinChain := []byte(`[
		{"type":"line","data":{"x":7,"y":8,"x2":9,"y2":10,"color":2,"width":8}}
	]`)

	// Distinct senders keep ownership assertions honest.
	require.True(t, lobby.AppendWallEventChain(marcelChain, marcel))
	require.True(t, lobby.AppendWallEventChain(kevinChain, kevin))

	// Undoing kevin's stroke while it's the last one.
	require.True(t, lobby.UndoLastWallStroke(kevin))
	snapshot := lobby.WallSnapshot()
	require.Len(t, snapshot, 2, "Kevin's single-segment stroke should be gone.")

	// Marcel draws again, then undoes: the removal must splice Marcel's
	// last stroke out, leaving kevin's stroke (drawn after Marcel's first)
	// intact.
	require.True(t, lobby.AppendWallEventChain(kevinChain, kevin))
	require.True(t, lobby.UndoLastWallStroke(marcel))
	snapshot = lobby.WallSnapshot()
	require.Len(t, snapshot, 1)
	remaining, isLine := snapshot[0].(*LineEvent)
	require.True(t, isLine)
	require.Equal(t, int16(7), remaining.Data.X,
		"The remaining stroke must be kevin's second one.")

	// Marcel has nothing left to undo.
	require.False(t, lobby.UndoLastWallStroke(marcel))

	// Clear empties everything and bumps the version.
	versionBefore := lobby.WallVersion()
	require.True(t, lobby.ClearWall())
	require.Empty(t, lobby.WallSnapshot())
	require.Greater(t, lobby.WallVersion(), versionBefore)

	// Clearing an empty wall changes nothing.
	require.False(t, lobby.ClearWall())

	// Loaded wall content has no stroke records and can't be undone.
	lobby.LoadWall([]any{&LineEvent{Type: EventTypeLine}})
	require.False(t, lobby.UndoLastWallStroke(marcel),
		"Booted wall content must not be undoable.")
	require.Len(t, lobby.WallSnapshot(), 1)
}
