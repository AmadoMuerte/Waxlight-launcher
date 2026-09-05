package discord

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// TestDialReturnsNilWhenPipeMissing verifies that Dial against a
// non-existent IPC pipe returns a nil client instead of crashing.
func TestDialReturnsNilWhenPipeMissing(t *testing.T) {
	old := discoverPipePaths
	discoverPipePaths = func() []string {
		return []string{filepath.Join(t.TempDir(), "missing-discord-ipc-0")}
	}
	t.Cleanup(func() { discoverPipePaths = old })

	if client := Dial("test-app-id"); client != nil {
		client.Close()
		t.Fatalf("Dial() with a missing pipe = %v, want nil", client)
	}
}

// TestActivityNoopWhenDisconnected verifies that SetActivity and
// ClearActivity on a disconnected client are no-ops that return nil.
func TestActivityNoopWhenDisconnected(t *testing.T) {
	client := &Client{}
	if client.Connected() {
		t.Fatal("Connected() on a disconnected client = true")
	}
	if err := client.SetActivity(Activity{State: "In a world"}); err != nil {
		t.Fatalf("SetActivity() on disconnected client = %v, want nil", err)
	}
	if err := client.ClearActivity(); err != nil {
		t.Fatalf("ClearActivity() on disconnected client = %v, want nil", err)
	}
	client.Close() // must not panic
}

func TestIPCMessageOmitsHeaderOpcodeFromJSON(t *testing.T) {
	payload, err := json.Marshal(ipcMessage{Op: opFrame, Cmd: "SET_ACTIVITY"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte(`"op"`)) {
		t.Fatalf("ipcMessage JSON includes header opcode: %s", payload)
	}
}

// TestProtocolRoundTrip runs a fake Discord IPC server over a unix socket and
// verifies the whole exchange: handshake, READY, SET_ACTIVITY, heartbeat
// reply, activity clearing, and the op 2 close frame.
func TestProtocolRoundTrip(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "discord-ipc-0")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	old := discoverPipePaths
	discoverPipePaths = func() []string { return []string{socketPath} }
	t.Cleanup(func() { discoverPipePaths = old })

	// heartbeatReplied is closed once the fake server has seen the client's
	// heartbeat reply, so the test only closes the client afterwards.
	heartbeatReplied := make(chan struct{})
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		serverErr <- serveFakeDiscord(conn, heartbeatReplied)
	}()

	client := Dial("test-app-id")
	if client == nil {
		t.Fatal("Dial() failed against the fake Discord server")
	}
	if !client.Connected() {
		t.Fatal("Connected() after READY = false")
	}
	defer client.Close()

	if err := client.SetActivity(Activity{
		State:          "In a world",
		Details:        "Waxlight server",
		LargeImageKey:  "vintage_story",
		SmallImageText: "artisan",
	}); err != nil {
		t.Fatalf("SetActivity() = %v, want nil", err)
	}

	select {
	case <-heartbeatReplied:
	case err := <-serverErr:
		t.Fatalf("fake server failed before heartbeat reply: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the fake server to see the heartbeat reply")
	}

	if err := client.ClearActivity(); err != nil {
		t.Fatalf("ClearActivity() = %v, want nil", err)
	}
	client.Close()

	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("fake server: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fake server timed out waiting for client frames")
	}
}

// serveFakeDiscord speaks the fake server side of the protocol: it accepts
// one connection and validates every frame the client under test sends. It
// closes heartbeatReplied after the client's heartbeat reply arrives; the
// clearing SET_ACTIVITY frame is read after that.
func serveFakeDiscord(conn net.Conn, heartbeatReplied chan<- struct{}) error {
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	fail := func(format string, args ...any) error { return fmt.Errorf(format, args...) }

	// 1. Handshake, op 0 with v=1 and the app id.
	op, raw, err := readRawFrame(conn)
	if err != nil {
		return fail("handshake read: %w", err)
	}
	if op != opHandshake {
		return fail("handshake op = %d, want %d", op, opHandshake)
	}
	var greeting handshakeRequest
	if err := json.Unmarshal(raw, &greeting); err != nil {
		return fail("handshake decode: %w", err)
	}
	if greeting.V != 1 || greeting.ClientID != "test-app-id" {
		return fail("handshake = %+v", greeting)
	}
	var greetingFields map[string]any
	if err := json.Unmarshal(raw, &greetingFields); err != nil {
		return fail("handshake fields decode: %w", err)
	}
	if len(greetingFields) != 2 {
		return fail("handshake fields = %v, want only v and client_id", greetingFields)
	}

	// 2. DISPATCH/READY, op 1 with evt READY.
	if err := writeRawFrame(conn, ipcMessage{Op: opFrame, Cmd: "DISPATCH", Evt: "READY"}); err != nil {
		return err
	}

	// 3. SET_ACTIVITY immediately follows READY; no AUTHORIZE command is sent.
	msg, _, err := readMessage(conn)
	if err != nil {
		return fail("set_activity read: %w", err)
	}
	if msg.Op != opFrame || msg.Cmd != "SET_ACTIVITY" {
		return fail("set_activity = op %d cmd %q", msg.Op, msg.Cmd)
	}
	var command activityRequest
	if err := json.Unmarshal(msg.Args, &command); err != nil {
		return fail("set_activity decode: %w", err)
	}
	if command.Pid <= 0 || command.Activity == nil || command.Activity.State != "In a world" || command.Activity.Details != "Waxlight server" {
		return fail("set_activity args = %+v", command)
	}

	// 4. Heartbeat request; the client's readLoop must reply with the same
	// args (op 1, no cmd/evt).
	if err := writeRawFrame(conn, ipcMessage{Op: opFrame, Evt: "HEARTBEAT", Args: json.RawMessage(`{"seq":42}`)}); err != nil {
		return err
	}
	msg, _, err = readMessage(conn)
	if err != nil {
		return fail("heartbeat reply read: %w", err)
	}
	if msg.Op != opFrame || msg.Cmd != "" || msg.Evt != "" {
		return fail("heartbeat reply = op %d cmd %q evt %q", msg.Op, msg.Cmd, msg.Evt)
	}
	var returned map[string]int
	if err := json.Unmarshal(msg.Args, &returned); err != nil {
		return fail("heartbeat reply decode: %w", err)
	}
	if returned["seq"] != 42 {
		return fail("heartbeat reply args = %v, want seq 42", returned)
	}
	close(heartbeatReplied)

	// 5. Discord clears activity with SET_ACTIVITY and an explicit JSON null.
	msg, _, err = readMessage(conn)
	if err != nil {
		return fail("clear_activity read: %w", err)
	}
	if msg.Op != opFrame || msg.Cmd != "SET_ACTIVITY" {
		return fail("clear_activity = op %d cmd %q", msg.Op, msg.Cmd)
	}
	var cleared activityRequest
	if err := json.Unmarshal(msg.Args, &cleared); err != nil {
		return fail("clear_activity decode: %w", err)
	}
	if cleared.Pid <= 0 || cleared.Activity != nil {
		return fail("clear_activity args = %+v", cleared)
	}
	var clearedFields map[string]any
	if err := json.Unmarshal(msg.Args, &clearedFields); err != nil {
		return fail("clear_activity fields decode: %w", err)
	}
	activity, exists := clearedFields["activity"]
	if !exists || activity != nil {
		return fail("clear_activity activity = %#v, exists = %v, want explicit null", activity, exists)
	}

	// 6. Close, op 2.
	msg, _, err = readMessage(conn)
	if err != nil {
		return fail("close read: %w", err)
	}
	if msg.Op != opClose {
		return fail("close op = %d, want %d", msg.Op, opClose)
	}
	return nil
}

func TestReadLoopClosesOnCloseFrame(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := &Client{conn: clientConn}
	go client.readLoop(clientConn)

	if err := writeRawFrame(serverConn, ipcMessage{Op: opClose}); err != nil {
		t.Fatal(err)
	}
	_ = serverConn.SetReadDeadline(time.Now().Add(time.Second))
	var one [1]byte
	if _, err := serverConn.Read(one[:]); err == nil {
		t.Fatal("server Read() after op close succeeded, want closed connection")
	}
	for deadline := time.Now().Add(time.Second); client.Connected() && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	if client.Connected() {
		t.Fatal("Connected() after op close = true")
	}
}

func TestSendFrameHandlesShortWrites(t *testing.T) {
	conn := &shortWriteConn{limit: 3}
	client := &Client{conn: conn}
	payload := []byte(`{"cmd":"SET_ACTIVITY"}`)
	if err := client.sendFrame(opFrame, payload); err != nil {
		t.Fatal(err)
	}
	op, raw, err := readRawFrame(bytes.NewReader(conn.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if op != opFrame || !bytes.Equal(raw, payload) {
		t.Fatalf("frame = op %d payload %q", op, raw)
	}
}

func TestCloseAlwaysClosesConnection(t *testing.T) {
	conn := &shortWriteConn{limit: 3}
	client := &Client{conn: conn}

	client.Close()

	if !conn.closed {
		t.Fatal("Close() did not close the IPC connection")
	}
	if client.Connected() {
		t.Fatal("Connected() after Close() = true")
	}
}

type shortWriteConn struct {
	bytes.Buffer
	limit  int
	closed bool
}

func (conn *shortWriteConn) Write(value []byte) (int, error) {
	if len(value) > conn.limit {
		value = value[:conn.limit]
	}
	return conn.Buffer.Write(value)
}

func (*shortWriteConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (conn *shortWriteConn) Close() error                { conn.closed = true; return nil }
func (*shortWriteConn) LocalAddr() net.Addr              { return testAddr("local") }
func (*shortWriteConn) RemoteAddr() net.Addr             { return testAddr("remote") }
func (*shortWriteConn) SetDeadline(time.Time) error      { return nil }
func (*shortWriteConn) SetReadDeadline(time.Time) error  { return nil }
func (*shortWriteConn) SetWriteDeadline(time.Time) error { return nil }

type testAddr string

func (address testAddr) Network() string { return string(address) }
func (address testAddr) String() string  { return string(address) }

// readRawFrame reads one framed payload (8-byte little-endian opcode+length
// header followed by the JSON bytes) from conn. Used by the fake server to
// validate each op 1 frame the client sends.
func readRawFrame(r io.Reader) (int, []byte, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	length := binary.LittleEndian.Uint32(header[4:])
	if length == 0 || length > maxRpcFrameSize {
		return 0, nil, fmt.Errorf("invalid frame length %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return int(binary.LittleEndian.Uint32(header[:4])), payload, nil
}

// writeRawFrame writes one framed JSON message to conn using the same 8-byte
// opcode+length header the real client expects. When msg has Op 0 it is the
// handshake opcode; otherwise the frame opcode defaults to opFrame.
func writeRawFrame(conn net.Conn, msg ipcMessage) error {
	op := msg.Op
	if op == 0 {
		op = opFrame
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	frame := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(frame, uint32(op))
	binary.LittleEndian.PutUint32(frame[4:], uint32(len(payload)))
	copy(frame[8:], payload)
	_, err = conn.Write(frame)
	return err
}
