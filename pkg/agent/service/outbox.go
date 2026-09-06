package service

import (
	"log/slog"
	"sync"
	"time"

	"github.com/pika-monitor/pika/internal/protocol"
)

const (
	// outboxMaxEntries 待确认队列上限。溢出时丢弃最旧消息：新事件（尤其是
	// 安全事件）比旧事件更有价值，且队列只增不减通常意味着探针已离线过久。
	outboxMaxEntries = 1024

	// outboxDropLogInterval 溢出告警的日志节流，避免事件风暴时刷屏
	outboxDropLogInterval = 30 * time.Second
)

type outboxEntry struct {
	seq uint64
	msg protocol.OutboundMessage
}

// outbox 事件类出站消息的有界待确认队列，实现 at-least-once 投递：
//
//   - 每条消息入队时分配单调递增 seq；
//   - 网络写成功不代表删除，只有收到服务端累计 ack（或重连时注册响应
//     携带的 AckSeq）才修剪队列；
//   - 每次新连接从队头整体重放未确认消息，由服务端按 seq 去重。
//
// 纯内存实现：进程重启后清空，seq 从 1 重新分配。服务端通过注册请求
// 携带的 BootID 识别探针重启并重置确认位点，避免新序号被旧位点误判
// 为重复消息。
type outbox struct {
	mu       sync.Mutex
	entries  []outboxEntry
	nextSeq  uint64
	ackedSeq uint64
	dropped  uint64
	lastDrop time.Time
	notify   chan struct{}
}

func newOutbox() *outbox {
	return &outbox{
		notify: make(chan struct{}, 1),
	}
}

// enqueue 分配序号并入队，返回分配的 seq。队列满时丢弃最旧消息。
func (o *outbox) enqueue(msg protocol.OutboundMessage) uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.nextSeq++
	msg.Seq = o.nextSeq
	o.entries = append(o.entries, outboxEntry{seq: msg.Seq, msg: msg})

	if len(o.entries) > outboxMaxEntries {
		drop := len(o.entries) - outboxMaxEntries
		o.entries = o.entries[drop:]
		o.dropped += uint64(drop)
		if time.Since(o.lastDrop) >= outboxDropLogInterval {
			slog.Warn("事件待确认队列溢出，已丢弃最旧消息",
				"droppedTotal", o.dropped, "queueSize", len(o.entries))
			o.lastDrop = time.Now()
		}
	}

	o.signal()
	return msg.Seq
}

// after 返回 seq 大于 lastSent 的消息快照（按序），供发送协程增量发送或
// 连接建立后整体重放。ack 修剪只从队头移除，不影响该判定。
func (o *outbox) after(lastSent uint64) []protocol.OutboundMessage {
	o.mu.Lock()
	defer o.mu.Unlock()

	msgs := make([]protocol.OutboundMessage, 0, len(o.entries))
	for _, e := range o.entries {
		if e.seq > lastSent {
			msgs = append(msgs, e.msg)
		}
	}
	return msgs
}

// ack 记录服务端累计确认并修剪队列，返回本次修剪的消息数。
func (o *outbox) ack(seq uint64) int {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 不信任超过本进程已分配范围的 ACK。异常或过期服务端位点不能
	// 把未来序号也标成已确认，否则后续消息会长期无法正常修剪。
	if seq > o.nextSeq {
		seq = o.nextSeq
	}
	if seq <= o.ackedSeq {
		return 0
	}
	o.ackedSeq = seq

	kept := o.entries[:0]
	for _, e := range o.entries {
		if e.seq > seq {
			kept = append(kept, e)
		}
	}
	trimmed := len(o.entries) - len(kept)
	o.entries = kept
	return trimmed
}

// stats 返回队列统计（用于日志与观测）。
func (o *outbox) stats() (pending int, acked uint64, dropped uint64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.entries), o.ackedSeq, o.dropped
}

// signal 唤醒发送协程（非阻塞，协程自带快照逻辑不会漏消息）。
func (o *outbox) signal() {
	select {
	case o.notify <- struct{}{}:
	default:
	}
}

// wait 阻塞等待新消息或退出信号。
func (o *outbox) wait(done <-chan struct{}) bool {
	select {
	case <-o.notify:
		return true
	case <-done:
		return false
	}
}
