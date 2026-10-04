package game

import (
	"strings"
	"sync"
	"time"

	discordemojimap "github.com/Bios-Marcel/discordemojimap/v2"
	"github.com/gofrs/uuid/v5"
	"github.com/lxzan/gws"
	"golang.org/x/text/cases"
)

// slotReservationTime should give a player enough time to restart their browser
// without losing their slost.
const slotReservationTime = time.Minute * 1

type roundEndReason string

const (
	drawerDisconnected   roundEndReason = "drawer_disconnected"
	guessersDisconnected roundEndReason = "guessers_disconnected"
)

// Lobby represents a game session. It must not be sent via the API, as it
// exposes gameplay relevant information.
type Lobby struct {
	// ID uniquely identified the Lobby.
	LobbyID string

	EditableLobbySettings

	// DrawingTimeNew is the new value of the drawing time. If a round is
	// already ongoing, we can't simply change the drawing time, as it would
	// screw with the score calculation of the current turn.
	DrawingTimeNew int

	CustomWords []string
	// customWordIndex is used to keep track of the next word to be drawn from
	// the custom word stack. Only used if exclusive custom word mode is active.
	customWordIndex int
	words           []string

	// players references all participants of the Lobby.
	players []*Player

	// Whether the game has started, is ongoing or already over.
	State State
	// OwnerID references the Player that currently owns the lobby.
	// Meaning this player has rights to restart or change certain settings.
	OwnerID uuid.UUID
	// ScoreCalculation decides how scores for both guessers and drawers are
	// determined.
	ScoreCalculation ScoreCalculation
	// CurrentWord represents the word that was last selected. If no word has
	// been selected yet or the round is already over, this should be empty.
	CurrentWord string
	// wordHints for the current word.
	wordHints []*WordHint
	// wordHintsShown are the same as wordHints with characters visible.
	wordHintsShown []*WordHint
	// hintsLeft is the amount of hints still available for revelation.
	hintsLeft int
	// hintCount is the amount of hints that were initially available
	// for revelation.
	hintCount int
	// Round is the round that the Lobby is currently in. This is a number
	// between 0 and Rounds. 0 indicates that it hasn't started yet.
	Round             int
	wordChoiceEndTime time.Time
	preSelectedWord   int
	// wordChoice represents the current choice of words present to the drawer.
	wordChoice []string
	Wordpack   string
	// roundEndTime represents the time at which the current round will end.
	// This is a UTC unix-timestamp in milliseconds.
	roundEndTime   int64
	roundEndReason roundEndReason

	timeLeftTicker *time.Ticker
	// currentDrawing represents the state of the current canvas. The elements
	// consist of LineEvent and FillEvent. Please do not modify the contents
	// of this array an only move AppendLine and AppendFill on the respective
	// lobby object.
	currentDrawing []any

	// These variables are used to define the ranges of connected drawing events.
	// For example a line that has been drawn or a fill that has been executed.
	// Since we can't trust the client to tell us this, we use the time passed
	// between draw events as an indicator of which draw events make up one line.
	// An alternative approach could be using the coordinates and see if they are
	// connected, but that could technically undo a whole drawing.

	lastDrawEvent                 time.Time
	connectedDrawEventsIndexStack []int

	// IsHub marks the one persistent room behind the home page. It must
	// never be removed by the cleanup routine and has no lobby UI.
	IsHub bool

	// wallDrawing is the persistent drawing wall of the home page. In
	// contrast to currentDrawing it is never cleared and can be drawn on
	// by everyone. Elements are LineEvent and FillEvent, just like
	// currentDrawing.
	wallDrawing []any
	wallDirty   bool

	// wallStrokes records, in order, how many consecutive wall entries
	// belong to which player, so that a player can undo their own last
	// stroke even when others drew after them. Records loaded from disk
	// don't exist, making booted wall content not undoable.
	wallStrokes []wallStrokeRecord

	// wallUpdateCounter is bumped whenever the wall changes. Clients use it
	// to detect wall changes when polling over HTTP.
	wallUpdateCounter uint64

	// chatLog stores the most recent chat messages with monotonic IDs, so
	// that clients can poll them over HTTP without maintaining a socket.
	chatCounter uint64
	chatLog     []StoreMessage

	lowercaser cases.Caser

	// LastPlayerDisconnectTime is used to know since when a lobby is empty, in case
	// it is empty.
	LastPlayerDisconnectTime *time.Time

	mutex sync.Mutex

	IsWordpackRtl bool

	WriteObject          func(*Player, any) error
	WritePreparedMessage func(*Player, *gws.Broadcaster) error
}

// MaxPlayerNameLength defines how long a string can be at max when used
// as the playername.
const MaxPlayerNameLength int = 30

// GetLastKnownAddress returns the last known IP-Address used for an HTTP request.
func (player *Player) GetLastKnownAddress() string {
	return player.lastKnownAddress
}

// SetLastKnownAddress sets the last known IP-Address used for an HTTP request.
// Can be retrieved via GetLastKnownAddress().
func (player *Player) SetLastKnownAddress(address string) {
	player.lastKnownAddress = address
}

// GetWebsocket simply returns the players websocket connection. This method
// exists to encapsulate the websocket field and prevent accidental sending
// the websocket data via the network.
func (player *Player) GetWebsocket() *gws.Conn {
	return player.ws
}

// SetWebsocket sets the given connection as the players websocket connection.
func (player *Player) SetWebsocket(socket *gws.Conn) {
	player.ws = socket
}

// GetUserSession returns the players current user session.
func (player *Player) GetUserSession() uuid.UUID {
	return player.userSession
}

// TouchLastSeen records that the player recently interacted with the lobby
// over HTTP (chat or wall polling), which is how socketless clients prove
// their presence.
func (player *Player) TouchLastSeen() {
	player.lastSeen = time.Now()
}

// LastSeen reports when the player last interacted with the lobby.
func (player *Player) LastSeen() time.Time {
	return player.lastSeen
}

// wallStrokeInterval is the minimum spacing between two wall strokes of one
// session. A mousemove produces around one stroke per rendered frame, so
// this must be well below a second; it only exists to survive the loss of
// the websocket's natural backpressure against flooding.
const wallStrokeInterval = 15 * time.Millisecond

// TakeWallStroke reports whether the player may add another wall stroke
// right now and records the attempt. Must be called while holding the
// lobby's mutex, matching how the rest of the codebase mutates players.
func (player *Player) TakeWallStroke() bool {
	now := time.Now()
	if now.Sub(player.lastWallStroke) < wallStrokeInterval {
		return false
	}
	player.lastWallStroke = now
	return true
}

type PlayerState string

const (
	Guessing   PlayerState = "guessing"
	Drawing    PlayerState = "drawing"
	Standby    PlayerState = "standby"
	Ready      PlayerState = "ready"
	Spectating PlayerState = "spectating"
)

func (lobby *Lobby) GetPlayerByID(id uuid.UUID) *Player {
	for _, player := range lobby.players {
		if player.ID == id {
			return player
		}
	}

	return nil
}

func (lobby *Lobby) GetPlayerBySession(userSession uuid.UUID) *Player {
	for _, player := range lobby.players {
		if player.userSession == userSession {
			return player
		}
	}

	return nil
}

func (lobby *Lobby) GetOwner() *Player {
	return lobby.GetPlayerByID(lobby.OwnerID)
}

func (lobby *Lobby) ClearDrawing() {
	lobby.currentDrawing = make([]any, 0)
	lobby.connectedDrawEventsIndexStack = nil
}

// AppendLine adds a line direction to the current drawing. This exists in order
// to prevent adding arbitrary elements to the drawing, as the backing array is
// an empty interface type.
func (lobby *Lobby) AppendLine(line *LineEvent) {
	lobby.currentDrawing = append(lobby.currentDrawing, line)
}

// AppendFill adds a fill direction to the current drawing. This exists in order
// to prevent adding arbitrary elements to the drawing, as the backing array is
// an empty interface type.
func (lobby *Lobby) AppendFill(fill *FillEvent) {
	lobby.currentDrawing = append(lobby.currentDrawing, fill)
}

// WallSnapshot returns a copy of the wall's drawing events, e.g. for
// persisting them to disk or sending them to newly connected players.
func (lobby *Lobby) WallSnapshot() []any {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	snapshot := make([]any, len(lobby.wallDrawing))
	copy(snapshot, lobby.wallDrawing)
	return snapshot
}

// WallDirty reports whether the wall contains changes that haven't been
// saved to disk yet.
func (lobby *Lobby) WallDirty() bool {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	return lobby.wallDirty
}

// MarkWallSaved resets the wall's dirty flag. Called by the wall store after
// a successful save.
func (lobby *Lobby) MarkWallSaved() {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	lobby.wallDirty = false
}

// LoadWall replaces the wall's drawing events, e.g. with data loaded from
// disk at boot time.
func (lobby *Lobby) LoadWall(events []any) {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	lobby.wallDrawing = events
	// Loaded wall content has no stroke records, so it can't be undone.
	lobby.wallStrokes = nil
	lobby.wallDirty = false
	// The counter only needs to differ from the previous value; a fresh
	// boot starting at 1 is fine, as clients compare within one uptime.
	lobby.wallUpdateCounter++
}

// WallVersion returns a number that changes whenever the wall's contents
// changed. HTTP clients compare it to detect updates without transferring
// the whole wall.
func (lobby *Lobby) WallVersion() uint64 {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	return lobby.wallUpdateCounter
}

// chatLogCapacity defines how many of the most recent chat messages are
// kept for HTTP polling.
const chatLogCapacity = 200

// wallStrokeRecord documents which player appended how many consecutive
// entries to wallDrawing in one action (one stroke chain or one fill).
type wallStrokeRecord struct {
	ownerID uuid.UUID
	count   int
}

// ChatUpdates returns all stored messages newer than the given ID. Pass 0
// to receive the whole recent log. It also reports the latest available ID,
// which clients pass back as the cursor on their next poll, and the current
// wall version, so a single request can drive both polling loops. Acquires
// the lobby mutex itself.
func (lobby *Lobby) ChatUpdates(sinceID uint64) ([]StoreMessage, uint64, uint64) {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	// Find the first message newer than the cursor. The log is small, so a
	// linear search is fine and avoids keeping a separate index structure.
	start := len(lobby.chatLog)
	for index, storedMessage := range lobby.chatLog {
		if storedMessage.ID > sinceID {
			start = index
			break
		}
	}

	updates := make([]StoreMessage, len(lobby.chatLog)-start)
	copy(updates, lobby.chatLog[start:])

	return updates, lobby.chatCounter, lobby.wallUpdateCounter
}

// SanitizeName removes invalid characters from the players name, resolves
// emoji codes, limits the name length and generates a new name if necessary.
func SanitizeName(name string) string {
	// We trim and handle emojis beforehand to avoid taking this into account
	// when checking the name length, so we don't cut off too much of the name.
	newName := discordemojimap.Replace(strings.TrimSpace(name))

	// We don't want super-long names
	if len(newName) > MaxPlayerNameLength {
		return newName[:MaxPlayerNameLength+1]
	}

	if newName != "" {
		return newName
	}

	return generatePlayerName()
}

// GetConnectedPlayerCount returns the amount of player that have currently
// established a socket connection.
func (lobby *Lobby) GetConnectedPlayerCount() int {
	var count int
	for _, player := range lobby.players {
		if player.Connected {
			count++
		}
	}

	return count
}

func (lobby *Lobby) HasConnectedPlayers() bool {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	for _, otherPlayer := range lobby.players {
		if otherPlayer.Connected {
			return true
		}
	}

	return false
}

// CanIPConnect checks whether the IP is still allowed regarding the lobbies
// clients per IP address limit. This function should only be called for
// players that aren't already in the lobby.
func (lobby *Lobby) CanIPConnect(address string) bool {
	var clientsWithSameIP int
	for _, player := range lobby.GetPlayers() {
		if player.GetLastKnownAddress() == address {
			clientsWithSameIP++
			if clientsWithSameIP >= lobby.ClientsPerIPLimit {
				return false
			}
		}
	}

	return true
}

func (lobby *Lobby) IsPublic() bool {
	return lobby.Public
}

func (lobby *Lobby) GetPlayers() []*Player {
	return lobby.players
}

// GetOccupiedPlayerSlots counts the available slots which can be taken by new
// players. Whether a slot is available is determined by the player count and
// whether a player is disconnect or furthermore how long they have been
// disconnected for. Therefore the result of this function will differ from
// Lobby.GetConnectedPlayerCount.
func (lobby *Lobby) GetOccupiedPlayerSlots() int {
	var occupiedPlayerSlots int
	now := time.Now()
	for _, player := range lobby.players {
		if player.Connected {
			occupiedPlayerSlots++
		} else {
			disconnectTime := player.disconnectTime

			// If a player hasn't been disconnected for a certain
			// timeframe, we will reserve the slot. This avoids frustration
			// in situations where a player has to restart their PC or so.
			if disconnectTime == nil || now.Sub(*disconnectTime) < slotReservationTime {
				occupiedPlayerSlots++
			}
		}
	}

	return occupiedPlayerSlots
}

// HasFreePlayerSlot determines whether the lobby still has a slot for at
// least one more player. If a player has disconnected recently, the slot
// will be preserved for 5 minutes. This function should be used over
// Lobby.GetOccupiedPlayerSlots, as it is potentially faster.
func (lobby *Lobby) HasFreePlayerSlot() bool {
	if len(lobby.players) < lobby.MaxPlayers {
		return true
	}

	return lobby.GetOccupiedPlayerSlots() < lobby.MaxPlayers
}

// Synchronized allows running a function while keeping the lobby locked via
// it's own mutex. This is useful in order to avoid having to relock a lobby
// multiple times, which might cause unexpected inconsistencies.
func (lobby *Lobby) Synchronized(logic func()) {
	lobby.mutex.Lock()
	defer lobby.mutex.Unlock()

	logic()
}
