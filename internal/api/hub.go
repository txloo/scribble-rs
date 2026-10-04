// This file contains HTTP endpoints for the home page's room: chat, drawing
// wall and presence all run over plain request/response, so that visiting
// the home page never requires a websocket. They all address the hub
// implicitly, hence no lobby id.
package api

import (
	json "encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/scribble-rs/scribble.rs/internal/game"
	"github.com/scribble-rs/scribble.rs/internal/state"
)

// ErrPlayerMissing signals that the requesting session doesn't belong to the
// home page's room.
var ErrPlayerMissing = errors.New("you are not part of this chat, please reload the page")

// getHubPlayer resolves the requesting session to a player of the hub and
// touches their presence marker.
func getHubPlayer(request *http.Request) (*game.Lobby, *game.Player, error) {
	hub := state.HubLobby()
	if hub == nil {
		return nil, nil, ErrLobbyNotExistent
	}

	player := GetPlayer(hub, request)
	if player == nil {
		return nil, nil, ErrPlayerMissing
	}

	return hub, player, nil
}

type chatUpdate struct {
	Messages []game.StoreMessage `json:"messages"`
	// LatestID is the cursor to pass to the next poll.
	LatestID uint64 `json:"latestId"`
	// WallVersion allows piggybacking wall-change detection onto chat
	// polls, saving an extra request.
	WallVersion uint64 `json:"wallVersion"`
}

// getHubChat returns all messages newer than the since cursor. New visitors
// pass no cursor and bootstrap with the recent log. Since clients poll this
// endpoint continuously, it doubles as their presence heartbeat.
func (handler *V1Handler) getHubChat(writer http.ResponseWriter, request *http.Request) {
	hub, player, err := getHubPlayer(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	player.TouchLastSeen()

	rawSince := request.URL.Query().Get("since")
	var since uint64
	if rawSince != "" {
		var parseErr error
		since, parseErr = strconv.ParseUint(rawSince, 10, 64)
		if parseErr != nil {
			http.Error(writer, "invalid since parameter", http.StatusBadRequest)
			return
		}
	}

	messages, latestID, wallVersion := hub.ChatUpdates(since)
	if started, err := marshalToHTTPWriter(&chatUpdate{
		Messages:    messages,
		LatestID:    latestID,
		WallVersion: wallVersion,
	}, writer); err != nil {
		if !started {
			http.Error(writer, "error marshalling chat updates", http.StatusInternalServerError)
		}
		log.Printf("error marshalling chat updates: %s\n", err)
	}
}

// postHubChat sends a chat message without a websocket. Rate limiting and
// guess-checking apply exactly like on the websocket path, since both
// funnel through the same handler.
func (handler *V1Handler) postHubChat(writer http.ResponseWriter, request *http.Request) {
	hub, player, err := getHubPlayer(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "error parsing chat message", http.StatusBadRequest)
		return
	}

	player.TouchLastSeen()

	// SubmitChatMessage acquires the lobby mutex itself.
	if !hub.SubmitChatMessage(payload.Content, player) {
		http.Error(writer, "rate limited", http.StatusTooManyRequests)
		return
	}
}

// getHubWall serves the wall as a JSON event list. Clients fetch it when
// the wall version reported by chat polls changes.
func (handler *V1Handler) getHubWall(writer http.ResponseWriter, request *http.Request) {
	hub, _, err := getHubPlayer(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	snapshot := hub.WallSnapshot()
	if started, err := marshalToHTTPWriter(&wallData{
		Version: hub.WallVersion(),
		Events:  snapshot,
	}, writer); err != nil {
		if !started {
			http.Error(writer, "error marshalling wall data", http.StatusInternalServerError)
		}
		log.Printf("error marshalling wall data: %s\n", err)
	}
}

type wallData struct {
	Version uint64 `json:"version"`
	Events  []any  `json:"events"`
}

// postHubWall adds drawing to the wall over plain HTTP, no websocket
// required. Two payload shapes are accepted: `type: "line"` or `"fill"`
// with a single event, and `type: "line-chain"` where the event is an array
// of line segments forming one stroke (collected client-side between
// pointer-down and release). A too-fast session receives 429 and drops the
// stroke; the client simply continues with its next stroke.
func (handler *V1Handler) postHubWall(writer http.ResponseWriter, request *http.Request) {
	hub, player, err := getHubPlayer(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	var payload struct {
		Type  string          `json:"type"`
		Event json.RawMessage `json:"event"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "error parsing wall event", http.StatusBadRequest)
		return
	}

	player.TouchLastSeen()

	var accepted bool
	if payload.Type == "line-chain" {
		accepted = hub.AppendWallEventChain(payload.Event, player)
	} else {
		accepted = hub.AppendWallEvent(payload.Event, payload.Type, player)
	}

	if !accepted {
		http.Error(writer, "rate limited", http.StatusTooManyRequests)
		return
	}
}

// undoHubWall removes the requesting player's most recent stroke from the
// wall. A stroke is the whole batched pointer-down-to-release chain, or a
// single fill.
func (handler *V1Handler) undoHubWall(writer http.ResponseWriter, request *http.Request) {
	hub, player, err := getHubPlayer(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	player.TouchLastSeen()

	if !hub.UndoLastWallStroke(player) {
		http.Error(writer, "you have nothing to undo", http.StatusNotFound)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// clearHubWall removes everything from the wall, including the undo
// history.
func (handler *V1Handler) clearHubWall(writer http.ResponseWriter, request *http.Request) {
	hub, player, err := getHubPlayer(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	player.TouchLastSeen()

	if !hub.ClearWall() {
		http.Error(writer, "the wall is already empty", http.StatusNotFound)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}
