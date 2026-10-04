package state

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/scribble-rs/scribble.rs/internal/game"
)

// wallFileVersion is bumped whenever the on-disk format changes, so that old
// files can be detected and discarded instead of misinterpreted.
const wallFileVersion = 1

// hubPlayerIdleThreshold defines how long a home page visitor may stay
// absent (no chat or drawing activity) before being pruned from the room.
const hubPlayerIdleThreshold = 10 * time.Minute

// persistedWall is the on-disk representation of the drawing wall. Events use
// the same JSON shape as they do on the websocket.
type persistedWall struct {
	Version int  `json:"version"`
	Events  []any `json:"events"`
}

// LaunchWallStore loads the wall from disk and starts the periodic save and
// maintenance loop. If path is empty, wall persistence is disabled entirely.
func LaunchWallStore(path string, lobby *game.Lobby) {
	if path == "" || lobby == nil {
		log.Println("Wall persistence disabled.")
		return
	}

	if err := loadWall(path, lobby); err != nil {
		log.Printf("error loading the wall from '%s': %s\n", path, err)
	}

	go hubMaintenanceLoop(path, lobby)
}

// loadWall replaces the lobby's wall with the data from disk. Corrupted data
// is reported but never fatal; the wall simply starts out empty then.
func loadWall(path string, lobby *game.Lobby) error {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("No wall file at '%s', starting with an empty wall.\n", path)
			return nil
		}
		return fmt.Errorf("error reading wall file: %w", err)
	}

	var persisted persistedWall
	if err := json.Unmarshal(fileBytes, &persisted); err != nil {
		return fmt.Errorf("error parsing wall file: %w", err)
	}

	if persisted.Version != wallFileVersion {
		return fmt.Errorf(
			"wall file version %d doesn't match expected version %d",
			persisted.Version, wallFileVersion)
	}

	// Elements are unmarshalled into their concrete types, so that the
	// server keeps working with properly typed events like the rest of the
	// codebase does.
	events := make([]any, 0, len(persisted.Events))
	for eventIndex, rawEvent := range persisted.Events {
		event, err := unmarshalWallEvent(rawEvent)
		if err != nil {
			// A single broken event shouldn't take down the whole history.
			log.Printf("skipping wall event %d: %s\n", eventIndex, err)
			continue
		}
		events = append(events, event)
	}

	lobby.LoadWall(events)
	log.Printf("Wall loaded from '%s' with %d events.\n", path, len(events))
	return nil
}

func unmarshalWallEvent(rawEvent any) (any, error) {
	rawBytes, err := json.Marshal(rawEvent)
	if err != nil {
		return nil, fmt.Errorf("error re-marshalling wall event: %w", err)
	}

	var typeOnly game.EventTypeOnly
	if err := json.Unmarshal(rawBytes, &typeOnly); err != nil {
		return nil, fmt.Errorf("error determining wall event type: %w", err)
	}

	switch typeOnly.Type {
	case game.EventTypeLine:
		event := &game.LineEvent{}
		if err := json.Unmarshal(rawBytes, event); err != nil {
			return nil, fmt.Errorf("error parsing line event: %w", err)
		}
		return event, nil
	case game.EventTypeFill:
		event := &game.FillEvent{}
		if err := json.Unmarshal(rawBytes, event); err != nil {
			return nil, fmt.Errorf("error parsing fill event: %w", err)
		}
		return event, nil
	default:
		return nil, fmt.Errorf("unknown wall event type '%s'", typeOnly.Type)
	}
}

// hubMaintenanceLoop saves the wall in regular intervals, whenever it's
// dirty, and prunes home page visitors that have been gone for too long.
func hubMaintenanceLoop(path string, lobby *game.Lobby) {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		removed := lobby.PruneIdlePlayers(hubPlayerIdleThreshold)
		if removed != 0 {
			log.Printf("Pruned %d idle home page visitors.\n", removed)
		}

		if !lobby.WallDirty() {
			continue
		}

		if err := saveWall(path, lobby); err != nil {
			log.Printf("error saving the wall to '%s': %s\n", path, err)
		} else {
			lobby.MarkWallSaved()
			log.Printf("Wall saved to '%s'.\n", path)
		}
	}
}

// saveWall writes the current wall to disk in one call. Callers are
// responsible for resetting the dirty flag afterwards.
func saveWall(path string, lobby *game.Lobby) error {
	persisted := persistedWall{
		Version: wallFileVersion,
		Events:  lobby.WallSnapshot(),
	}

	fileBytes, err := json.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("error marshalling wall: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("error creating wall directory: %w", err)
	}

	// Write to a temporary file first and then replace the target, so that
	// a crash mid-write can't corrupt the previous wall.
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, fileBytes, 0o644); err != nil {
		return fmt.Errorf("error writing wall file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("error replacing wall file: %w", err)
	}

	return nil
}

// SaveWallNow persists the wall immediately, e.g. during graceful shutdown.
func SaveWallNow(path string, lobby *game.Lobby) error {
	if path == "" || lobby == nil {
		return nil
	}

	if err := saveWall(path, lobby); err != nil {
		return err
	}
	lobby.MarkWallSaved()
	return nil
}
