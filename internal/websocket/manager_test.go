package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pika-monitor/pika/internal/protocol"
	"go.uber.org/zap"
)

func newTestClient(m *Manager, id, bootID string) *Client {
	c := NewClient(id, nil, m)
	m.Register(c, bootID)
	return c
}

func readAck(t *testing.T, c *Client) uint64 {
	t.Helper()
	select {
	case raw := <-c.send:
		var msg protocol.InputMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("unmarshal ack: %v", err)
		}
		if msg.Type != protocol.MessageTypeAck {
			t.Fatalf("expected ack message, got %s", msg.Type)
		}
		var ack protocol.AckData
		if err := json.Unmarshal(msg.Data, &ack); err != nil {
			t.Fatalf("unmarshal ack data: %v", err)
		}
		return ack.Seq
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ack")
		return 0
	}
}

func TestReliableSessionDedupAckAndOrder(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()

	var mu sync.Mutex
	var processed []string
	done := make(chan struct{})
	m.SetMessageHandler(func(_ context.Context, _, typ string, _ json.RawMessage) error {
		mu.Lock()
		processed = append(processed, typ)
		if len(processed) == 2 {
			close(done)
		}
		mu.Unlock()
		return nil
	})

	client := newTestClient(m, "agent-1", "boot-1")
	client.session.ackSeq.Store(1)
	client.session.reliableQueue <- agentMessage{seq: 1, typ: "replayed"}
	client.session.reliableQueue <- agentMessage{seq: 2, typ: "first"}
	client.session.reliableQueue <- agentMessage{seq: 3, typ: "second"}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for processing")
	}

	mu.Lock()
	got := append([]string(nil), processed...)
	mu.Unlock()
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("processed = %v, want [first second]", got)
	}
	if ack := m.AckSeq("agent-1"); ack != 3 {
		t.Fatalf("AckSeq = %d, want 3", ack)
	}
	for i, want := range []uint64{1, 2, 3} {
		if ack := readAck(t, client); ack != want {
			t.Fatalf("ack #%d = %d, want %d", i, ack, want)
		}
	}
}

func TestReliableFailureDoesNotAdvanceOrSkip(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()

	var allowFirst atomic.Bool
	processed := make(chan string, 4)
	m.SetMessageHandler(func(_ context.Context, _, typ string, _ json.RawMessage) error {
		if typ == "first" && !allowFirst.Load() {
			return errors.New("temporary database failure")
		}
		processed <- typ
		return nil
	})

	client := newTestClient(m, "agent-2", "boot-1")
	client.session.reliableQueue <- agentMessage{seq: 1, typ: "first"}
	client.session.reliableQueue <- agentMessage{seq: 2, typ: "second"}

	time.Sleep(100 * time.Millisecond)
	if ack := m.AckSeq("agent-2"); ack != 0 {
		t.Fatalf("AckSeq during failure = %d, want 0", ack)
	}
	select {
	case typ := <-processed:
		t.Fatalf("processed %s before seq 1 recovered", typ)
	default:
	}

	allowFirst.Store(true)
	for _, want := range []string{"first", "second"} {
		select {
		case got := <-processed:
			if got != want {
				t.Fatalf("processed %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %s", want)
		}
	}
	if ack := m.AckSeq("agent-2"); ack != 2 {
		t.Fatalf("AckSeq after recovery = %d, want 2", ack)
	}
}

func TestBootIDReplacementRejectsOldSessionCommit(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()

	oldStarted := make(chan struct{})
	releaseOld := make(chan struct{})
	newProcessed := make(chan struct{})
	m.SetMessageHandler(func(_ context.Context, _, typ string, _ json.RawMessage) error {
		switch typ {
		case "old":
			close(oldStarted)
			<-releaseOld // 模拟不遵守 context 的旧业务处理器
		case "new":
			close(newProcessed)
		}
		return nil
	})

	oldClient := newTestClient(m, "agent-3", "boot-old")
	oldSession := oldClient.session
	oldSession.reliableQueue <- agentMessage{seq: 100, typ: "old"}
	<-oldStarted

	newClient := newTestClient(m, "agent-3", "boot-new")
	if oldClient.session == newClient.session {
		t.Fatal("BootID change should replace the session")
	}
	if ack := m.AckSeq("agent-3"); ack != 0 {
		t.Fatalf("new session AckSeq = %d, want 0", ack)
	}

	close(releaseOld)
	newClient.session.reliableQueue <- agentMessage{seq: 1, typ: "new"}
	select {
	case <-newProcessed:
	case <-time.After(2 * time.Second):
		t.Fatal("new session message was not processed")
	}
	if ack := readAck(t, newClient); ack != 1 {
		t.Fatalf("new session ack = %d, want 1", ack)
	}
	time.Sleep(50 * time.Millisecond)
	if ack := m.AckSeq("agent-3"); ack != 1 {
		t.Fatalf("old session polluted AckSeq: got %d, want 1", ack)
	}
}

func TestSameBootReconnectReusesSession(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()

	first := newTestClient(m, "agent-4", "boot-1")
	first.session.ackSeq.Store(9)
	second := newTestClient(m, "agent-4", "boot-1")

	if first.session != second.session {
		t.Fatal("same BootID reconnect should reuse session")
	}
	if ack := m.AckSeq("agent-4"); ack != 9 {
		t.Fatalf("AckSeq after reconnect = %d, want 9", ack)
	}
	select {
	case <-first.done:
	default:
		t.Fatal("old transport should be closed after replacement")
	}
	if m.Unregister(first) {
		t.Fatal("old transport must not unregister the replacement")
	}
	if current, ok := m.GetClient("agent-4"); !ok || current != second {
		t.Fatal("replacement transport was removed by old unregister")
	}
}

func TestReplacedTransportCannotEnqueueOutOfOrderSequence(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()
	processed := make(chan string, 2)
	m.SetMessageHandler(func(_ context.Context, _, typ string, _ json.RawMessage) error {
		processed <- typ
		return nil
	})

	oldClient := newTestClient(m, "agent-order", "boot-1")
	newClient := newTestClient(m, "agent-order", "boot-1")
	if err := m.enqueueReliable(context.Background(), oldClient, agentMessage{seq: 10, typ: "stale"}); !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("old transport enqueue = %v, want ErrClientNotFound", err)
	}
	if err := m.enqueueReliable(context.Background(), newClient, agentMessage{seq: 1, typ: "current"}); err != nil {
		t.Fatalf("new transport enqueue: %v", err)
	}

	select {
	case typ := <-processed:
		if typ != "current" {
			t.Fatalf("processed %q, want current", typ)
		}
	case <-time.After(time.Second):
		t.Fatal("current transport message was not processed")
	}
	select {
	case typ := <-processed:
		t.Fatalf("processed stale transport message %q", typ)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBestEffortDoesNotBlockReliableEvents(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()

	blockTelemetry := make(chan struct{})
	reliableDone := make(chan struct{})
	m.SetMessageHandler(func(_ context.Context, _, typ string, _ json.RawMessage) error {
		if typ == "metrics" {
			<-blockTelemetry
		} else if typ == "event" {
			close(reliableDone)
		}
		return nil
	})

	client := newTestClient(m, "agent-5", "boot-1")
	client.session.bestEffortQueue <- agentMessage{typ: "metrics"}
	client.session.reliableQueue <- agentMessage{seq: 1, typ: "event"}
	select {
	case <-reliableDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reliable event was blocked by best-effort telemetry")
	}
	close(blockTelemetry)
}

func TestPermanentMessageAdvancesAck(t *testing.T) {
	m := NewManager(zap.NewNop())
	defer m.shutdown()
	m.SetMessageHandler(func(context.Context, string, string, json.RawMessage) error {
		return Permanent(errors.New("invalid payload"))
	})

	client := newTestClient(m, "agent-6", "boot-1")
	client.session.reliableQueue <- agentMessage{seq: 1, typ: "bad"}
	if ack := readAck(t, client); ack != 1 {
		t.Fatalf("permanent message ack = %d, want 1", ack)
	}
}

func TestClosedClientRejectsSendWithoutPanic(t *testing.T) {
	m := NewManager(zap.NewNop())
	client := newTestClient(m, "agent-7", "boot-1")
	client.close()

	if client.trySend([]byte("ack")) {
		t.Fatal("closed client accepted non-blocking send")
	}
	if err := client.sendWithTimeout([]byte("command"), time.Second); !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("send to closed client = %v, want ErrClientNotFound", err)
	}
	m.shutdown()
}

func TestClientShouldWriteStatusDebounce(t *testing.T) {
	m := NewManager(zap.NewNop())
	client := NewClient("agent-8", nil, m)
	if !client.ShouldWriteStatus(time.Minute) {
		t.Fatal("first check should pass")
	}
	client.MarkStatusWritten()
	if client.ShouldWriteStatus(time.Minute) {
		t.Fatal("second check within interval should be debounced")
	}
	if !client.ShouldWriteStatus(0) {
		t.Fatal("zero interval should always pass")
	}
}
