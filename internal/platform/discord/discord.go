// Package discord implements a minimal, best-effort Discord Rich Presence IPC
// client. It is a platform adapter: it contains no feature, Wails, or
// transport code and never stores or logs credentials.
//
// Discord availability is treated as optional. If the IPC pipe is missing or
// the handshake cannot be completed, Dial returns a nil client and every
// method on a disconnected client is a no-op, so the launcher never crashes
// or blocks because of Rich Presence.
//
// Wire format: every message is a JSON payload prefixed by an operation code
// and payload length, both little-endian uint32 values. Op 0 is the connection
// handshake, op 1 is a command/event frame, and op 2 closes the connection.
package discord

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// IPC operation codes. op codes 0-2 are carried as a 4-byte little-endian
// header word on the wire; the op header word is not part of the JSON payload.
const (
	opHandshake = 0
	opFrame     = 1
	opClose     = 2
	opPing      = 3
	opPong      = 4

	dialTimeout      = 2 * time.Second // connecting to the pipe
	handshakeTimeout = 10 * time.Second
	writeTimeout     = 2 * time.Second
	// maxRpcFrameSize mirrors the official client's on-wire frame cap.
	maxRpcFrameSize = 64 << 10
)

// discoverPipePaths is a variable so tests can point discovery at one socket.
var discoverPipePaths = pipePaths

// Activity is a Discord Rich Presence activity.
type Activity struct {
	State          string
	Details        string
	LargeImageKey  string
	LargeImageText string
	SmallImageKey  string
	SmallImageText string
	StartTimestamp *int64 // Unix time in seconds, as required by Discord
}

// Client is a best-effort Discord Rich Presence IPC client. All methods are
// safe for concurrent use.
type Client struct {
	conn  net.Conn
	mu    sync.Mutex
	appID string
}

// Dial attempts to connect to the local Discord IPC pipe and complete the
// handshake (READY). It returns a nil client on any failure, so callers can
// treat a nil client as "Discord is not available".
func Dial(appID string) *Client {
	var lastErr error
	for _, path := range discoverPipePaths() {
		conn, err := dialPipe(path, dialTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		client := &Client{conn: conn, appID: appID}
		if err := client.connect(); err != nil {
			slog.Debug("discord: handshake failed", "error", err)
			_ = conn.Close()
			continue
		}
		return client
	}
	if lastErr != nil {
		slog.Debug("discord: IPC pipe unavailable", "error", lastErr)
	}
	return nil
}

// connect performs the op 0 handshake, waits for READY, then starts the
// heartbeat reader. Rich Presence does not require authorization.
func (c *Client) connect() error {
	_ = c.conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	defer c.conn.SetReadDeadline(time.Time{})

	greeting, err := json.Marshal(handshakeRequest{V: 1, ClientID: c.appID})
	if err != nil {
		return err
	}
	if err := c.sendFrame(opHandshake, greeting); err != nil {
		return err
	}
	slog.Debug("discord: handshake sent, waiting for op 1 READY response", "client_id", c.appID)
	if err := c.readUntil(func(msg ipcMessage) bool {
		return msg.Op == opFrame && msg.Evt == "READY"
	}); err != nil {
		return err
	}

	go c.readLoop(c.conn)
	return nil
}

// readUntil reads frames until match returns true, answering any heartbeat
// requests that arrive along the way. Used only during the handshake, before
// the reader goroutine starts.
func (c *Client) readUntil(match func(ipcMessage) bool) error {
	for {
		msg, raw, err := readMessage(c.conn)
		if err != nil {
			return err
		}
		if msg.Op == opPing {
			if err := c.sendFrame(opPong, raw); err != nil {
				return err
			}
			continue
		}
		if msg.Op == opFrame && msg.Evt == "HEARTBEAT" {
			reply, err := heartbeatReply(msg)
			if err != nil {
				return err
			}
			if err := c.sendFrame(opFrame, reply); err != nil {
				return err
			}
			continue
		}
		if match(msg) {
			return nil
		}
	}
}

// readLoop reads frames for the lifetime of the connection, answering
// heartbeat requests. It marks the client disconnected when the pipe goes
// away.
func (c *Client) readLoop(conn net.Conn) {
	defer conn.Close()
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
	}()
	for {
		msg, raw, err := readMessage(conn)
		if err != nil {
			slog.Debug("discord: readLoop error", "error", err)
			return
		}
		slog.Debug("discord: readLoop frame", "op", msg.Op, "cmd", msg.Cmd, "evt", msg.Evt, "nonce", msg.Nonce)
		if msg.Op == opClose {
			return
		}
		if msg.Op == opPing {
			c.mu.Lock()
			err = c.sendFrame(opPong, raw)
			c.mu.Unlock()
			if err != nil {
				return
			}
			continue
		}
		if msg.Op == opFrame && msg.Evt == "HEARTBEAT" {
			reply, err := heartbeatReply(msg)
			if err != nil {
				return
			}
			c.mu.Lock()
			err = c.sendFrame(opFrame, reply)
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// SetActivity sends a SET_ACTIVITY frame. It is a best-effort no-op when
// Discord is not connected, and never panics.
func (c *Client) SetActivity(activity Activity) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	args, err := json.Marshal(activityRequest{Pid: os.Getpid(), Activity: buildActivity(activity)})
	if err != nil {
		return nil
	}
	payload, err := json.Marshal(ipcMessage{Op: opFrame, Cmd: "SET_ACTIVITY", Args: args, Nonce: newNonce()})
	if err != nil {
		return nil
	}
	slog.Debug("discord: SET_ACTIVITY sending")
	if err := c.sendFrame(opFrame, payload); err != nil {
		slog.Debug("discord: SET_ACTIVITY write failed", "error", err)
	} else {
		slog.Debug("discord: SET_ACTIVITY sent ok")
	}
	return nil
}

// ClearActivity sends SET_ACTIVITY with a null activity. It is a best-effort
// no-op when Discord is not connected, and never panics.
func (c *Client) ClearActivity() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	args, err := json.Marshal(activityRequest{Pid: os.Getpid()})
	if err != nil {
		return nil
	}
	payload, err := json.Marshal(ipcMessage{Op: opFrame, Cmd: "SET_ACTIVITY", Args: args, Nonce: newNonce()})
	if err != nil {
		return nil
	}
	if err := c.sendFrame(opFrame, payload); err != nil {
		slog.Debug("discord: clear activity write failed", "error", err)
	}
	return nil
}

// Close sends the op 2 close frame and closes the socket. It is idempotent.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	if shutdown, err := json.Marshal(ipcMessage{Op: opClose}); err == nil {
		_ = c.sendFrame(opClose, shutdown)
	}
	conn := c.conn
	c.conn = nil
	_ = conn.Close()
}

// Connected reports whether the client still has a live IPC connection.
func (c *Client) Connected() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}

// sendFrame writes one framed payload with the given operation code. The wire
// format is an 8-byte header (opcode then length, both little-endian uint32)
// followed by the JSON payload. Callers must hold c.mu unless the connection
// is still in its single-goroutine handshake phase.
func (c *Client) sendFrame(op int, payload []byte) error {
	if c.conn == nil {
		return nil
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	defer c.conn.SetWriteDeadline(time.Time{})
	frame := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(frame, uint32(op))
	binary.LittleEndian.PutUint32(frame[4:], uint32(len(payload)))
	copy(frame[8:], payload)
	for len(frame) > 0 {
		written, err := c.conn.Write(frame)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		frame = frame[written:]
	}
	return nil
}

// readMessage reads one framed message: an 8-byte little-endian header
// (opcode, then length) followed by that many bytes of JSON. The header
// opcode is mirrored onto the returned message so callers can dispatch on it
// without relying on a JSON "op" field (which real Discord does not send).
// The raw payload is returned alongside so Ping frames can be echoed verbatim.
func readMessage(r io.Reader) (ipcMessage, []byte, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return ipcMessage{}, nil, err
	}
	opcode := binary.LittleEndian.Uint32(header[:4])
	length := binary.LittleEndian.Uint32(header[4:])
	if length > maxRpcFrameSize {
		return ipcMessage{}, nil, fmt.Errorf("discord: invalid frame length %d", length)
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return ipcMessage{}, nil, err
		}
	}
	msg := ipcMessage{Op: int(opcode)}
	if length > 0 {
		if err := json.Unmarshal(payload, &msg); err != nil {
			return ipcMessage{}, nil, err
		}
	}
	return msg, payload, nil
}

// heartbeatReply echoes a heartbeat request with op 1 and the same args.
func heartbeatReply(msg ipcMessage) ([]byte, error) {
	reply, err := json.Marshal(ipcMessage{Op: opFrame, Args: msg.Args})
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// newNonce returns a short random nonce used to correlate commands.
func newNonce() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(raw[:])
}

// ipcMessage is a command/event frame carried inside an op 1 payload.
type ipcMessage struct {
	Op    int             `json:"-"`
	Cmd   string          `json:"cmd,omitempty"`
	Evt   string          `json:"evt,omitempty"`
	Args  json.RawMessage `json:"args,omitempty"`
	Nonce string          `json:"nonce,omitempty"`
}

// handshakeRequest is the op 0 connection greeting.
type handshakeRequest struct {
	V        int    `json:"v"`
	ClientID string `json:"client_id"`
}

// activityRequest is the args of SET_ACTIVITY. A nil Activity must be encoded
// as JSON null because that is Discord's clear-activity command.
type activityRequest struct {
	Pid      int              `json:"pid"`
	Activity *activityPayload `json:"activity"`
}

// activityPayload is the Discord wire representation of an Activity.
type activityPayload struct {
	State      string          `json:"state,omitempty"`
	Details    string          `json:"details,omitempty"`
	Timestamps *activityTime   `json:"timestamps,omitempty"`
	Assets     *activityAssets `json:"assets,omitempty"`
}

type activityTime struct {
	Start int64 `json:"start"`
}

type activityAssets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

// buildActivity converts an Activity into its Discord wire representation.
// It returns nil when the activity is empty.
func buildActivity(a Activity) *activityPayload {
	payload := &activityPayload{State: a.State, Details: a.Details}
	if a.StartTimestamp != nil {
		payload.Timestamps = &activityTime{Start: *a.StartTimestamp}
	}
	if a.LargeImageKey != "" || a.LargeImageText != "" || a.SmallImageKey != "" || a.SmallImageText != "" {
		payload.Assets = &activityAssets{
			LargeImage: a.LargeImageKey,
			LargeText:  a.LargeImageText,
			SmallImage: a.SmallImageKey,
			SmallText:  a.SmallImageText,
		}
	}
	if *payload == (activityPayload{}) {
		return nil
	}
	return payload
}
