// go-parallel-pinger is a easy and thread-safe way to send ping in Go.
package pinger

import (
	"context"
	"encoding/binary"
	"errors"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// DEFAULT_PRIVILEGED is the default value of Pinger.Privileged/Pinger.SetPrivileged.
	DEFAULT_PRIVILEGED = runtime.GOOS == "windows"
)

var (
	// ErrAlreadyStarted is the error if the pinger is already started.
	ErrAlreadyStarted = errors.New("Pinger is already started")

	// ErrNotStarted is the error if call Pinger.Ping before start the pinger.
	ErrNotStarted = errors.New("Pinger is not started yet")

	errInvalidTracker = errors.New("Invalid Tracker")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// tracker is a value for tracking echo reply.
type tracker struct {
	ProbeID   uint32
	MessageID uint32
}

func newTracker(probeID uint32) tracker {
	return tracker{
		ProbeID:   probeID,
		MessageID: rand.Uint32(),
	}
}

func (t *tracker) Unmarshal(raw []byte) error {
	if len(raw) != 8 {
		return errInvalidTracker
	}
	t.ProbeID = binary.BigEndian.Uint32(raw[:4])
	t.MessageID = binary.BigEndian.Uint32(raw[4:])
	return nil
}

func (t tracker) Marshal() []byte {
	var result [8]byte
	binary.BigEndian.PutUint32(result[:4], t.ProbeID)
	binary.BigEndian.PutUint32(result[4:], t.MessageID)
	return result[:]
}

// reply is a echo reply event from target hos.
type reply struct {
	ReceivedAt int64
	Tracker    tracker
}

// handlerRegistry manage handlers set.
type handlerRegistry struct {
	sync.RWMutex

	handlers map[uint32]chan reply
}

func newHandlerRegistry() *handlerRegistry {
	return &handlerRegistry{
		handlers: make(map[uint32]chan reply),
	}
}

func (h *handlerRegistry) Register(probeID uint32, ch chan reply) {
	h.Lock()
	defer h.Unlock()

	h.handlers[probeID] = ch
}

func (h *handlerRegistry) Unregister(probeID uint32) {
	h.Lock()
	defer h.Unlock()

	delete(h.handlers, probeID)
}

func (h *handlerRegistry) Handle(r reply) {
	h.RLock()
	defer h.RUnlock()

	if handler, ok := h.handlers[r.Tracker.ProbeID]; ok {
		handler <- r
	}
}

// Pinger is the ping sender and receiver.
type Pinger struct {
	lock sync.Mutex

	id int

	protocol   byte
	privileged bool

	started bool
	conn    *icmp.PacketConn

	handler *handlerRegistry
}

func newPinger(protocol byte) *Pinger {
	return &Pinger{
		id:         rand.Intn(0xffff + 1),
		protocol:   protocol,
		privileged: DEFAULT_PRIVILEGED,
	}
}

// NewIPv4 makes new Pinger for IPv4 protocol.
func NewIPv4() *Pinger {
	return newPinger('4')
}

// NewIPv6 makes new Pinger for IPv6 protocol.
func NewIPv6() *Pinger {
	return newPinger('6')
}

// Protocol returns "v4" if it's a Pinger for IPv4, otherwise returns "v6".
func (p *Pinger) Protocol() string {
	if p.protocol == '4' {
		return "v4"
	}
	return "v6"
}

// SetPrivileged sets privileged mode.
//
// It should set as true if runs on Windows, and the default value in Windows is true.
//
// In Linux or Darwin(mac os), you have to run as root if use privileged mode. and the default is false.
//
// You can't call it after call Start method.
// It will returns ErrAlreadyStarted if call it after Pinger started.
func (p *Pinger) SetPrivileged(b bool) error {
	if p.Started() {
		return ErrAlreadyStarted
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	p.privileged = b
	return nil
}

// Privileged returns current privileged mode.
//
// Please seealso SetPrivileged.
func (p *Pinger) Privileged() bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.privileged
}

// Started returns true if this Pinger is started, otherwise returns false.
func (p *Pinger) Started() bool {
	p.lock.Lock()
	defer p.lock.Unlock()
	return p.started
}

func (p *Pinger) close() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.conn.Close()
	p.conn = nil

	p.handler = nil

	p.started = false
}

func (p *Pinger) listen() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.started {
		return ErrAlreadyStarted
	}

	var err error
	p.conn, err = icmp.ListenPacket(p.protoName(), p.listenAddr())
	if err != nil {
		p.conn = nil
		return err
	}

	p.started = true
	p.handler = newHandlerRegistry()

	return nil
}

func (p *Pinger) runHandler(ctx context.Context) {
	var buf [1500]byte

	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		p.conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))

		if n, _, err := p.conn.ReadFrom(buf[:]); err == nil {
			p.onReceiveMessage(buf[:n])
		}
	}

	p.close()
}

// Start is starts this Pinger for send and receive ping in new goroutine.
//
// It returns ErrAlreadyStarted if the Pinger already started.
func (p *Pinger) Start(ctx context.Context) error {
	if err := p.listen(); err != nil {
		return err
	}

	go p.runHandler(ctx)

	return nil
}

func (p *Pinger) onReceiveMessage(raw []byte) {
	reply := reply{ReceivedAt: time.Now().UnixNano()}

	msg, err := icmp.ParseMessage(p.protoNum(), raw)
	if err != nil {
		return
	}

	if msg.Type != p.replyType() {
		return
	}

	if body, ok := msg.Body.(*icmp.Echo); !ok {
		return
	} else if err = reply.Tracker.Unmarshal(body.Data); err == nil {
		p.handler.Handle(reply)
	}
}

func (p *Pinger) listenAddr() string {
	if p.protocol == '4' {
		return "0.0.0.0"
	} else {
		return "::"
	}
}

func (p *Pinger) protoName() string {
	if p.privileged {
		if p.protocol == '4' {
			return "ip4:icmp"
		} else {
			return "ip6:ipv6-icmp"
		}
	} else {
		if p.protocol == '4' {
			return "udp4"
		} else {
			return "udp6"
		}
	}
}

func (p *Pinger) protoNum() int {
	if p.protocol == '4' {
		return 1
	} else {
		return 58
	}
}

func (p *Pinger) requestType() icmp.Type {
	if p.protocol == '4' {
		return ipv4.ICMPTypeEcho
	} else {
		return ipv6.ICMPTypeEchoRequest
	}
}

func (p *Pinger) replyType() icmp.Type {
	if p.protocol == '4' {
		return ipv4.ICMPTypeEchoReply
	} else {
		return ipv6.ICMPTypeEchoReply
	}
}

func (p *Pinger) send(dst net.Addr, seq int, t tracker) error {
	msg := icmp.Message{
		Type: p.requestType(),
		Body: &icmp.Echo{
			ID:   p.id,
			Seq:  seq,
			Data: t.Marshal(),
		},
	}
	if b, err := msg.Marshal(nil); err != nil {
		return err
	} else if _, err = p.conn.WriteTo(b, dst); err != nil {
		return err
	}

	return nil
}

func (p *Pinger) convertTarget(target *net.IPAddr) net.Addr {
	if p.Privileged() {
		return target
	} else {
		return &net.UDPAddr{IP: target.IP, Zone: target.Zone}
	}
}

// Ping sends ping to target, and wait for reply.
//
// It returns ErrNotStarted if call this before call Start.
func (p *Pinger) Ping(ctx context.Context, target *net.IPAddr, count int, interval time.Duration) (Result, error) {
	result := newResult(target, count)

	if !p.Started() {
		return result, ErrNotStarted
	}

	if count <= 0 {
		return result, nil
	}

	targetAddr := p.convertTarget(target)

	probeID := rand.Uint32()

	recv := make(chan reply, count+1)
	defer close(recv)

	p.handler.Register(probeID, recv)
	defer p.handler.Unregister(probeID)

	tick := time.NewTicker(interval)
	defer tick.Stop()

	result.Sent++
	t := newTracker(probeID)
	sent := map[uint32]int64{
		t.MessageID: time.Now().UnixNano(),
	}
	if err := p.send(targetAddr, result.Sent, t); err != nil {
		return result, err
	}

	for {
		select {
		case <-ctx.Done():
			return result.calculate(), nil
		case <-tick.C:
			if result.Sent < count {
				result.Sent++
				t = newTracker(probeID)
				sent[t.MessageID] = time.Now().UnixNano()
				if err := p.send(targetAddr, result.Sent, t); err != nil {
					return result.calculate(), err
				}
			}
		case r := <-recv:
			if sentAt, ok := sent[r.Tracker.MessageID]; ok {
				delete(sent, r.Tracker.MessageID)

				(&result).onRecv(time.Duration(r.ReceivedAt-sentAt) * time.Nanosecond)

				if result.Recv >= count {
					return result.calculate(), nil
				}
			}
		}
	}
}

// Result is a result of Ping.
type Result struct {
	Target *net.IPAddr
	Sent   int
	Recv   int
	Loss   int
	RTTs   []time.Duration
	MinRTT time.Duration
	MaxRTT time.Duration
	AvgRTT time.Duration
}

func newResult(target *net.IPAddr, total int) Result {
	return Result{
		Target: target,
		Loss:   total,
		RTTs:   make([]time.Duration, 0, total),
	}
}

func (r *Result) onRecv(rtt time.Duration) {
	r.Recv++
	r.Loss--

	r.RTTs = append(r.RTTs, rtt)
}

func (r Result) calculate() Result {
	if len(r.RTTs) == 0 {
		return r
	}

	r.MinRTT = r.RTTs[0]
	r.MaxRTT = r.RTTs[0]
	r.AvgRTT = r.RTTs[0]

	for _, x := range r.RTTs[1:] {
		if x < r.MinRTT {
			r.MinRTT = x
		}
		if x > r.MaxRTT {
			r.MaxRTT = x
		}
		r.AvgRTT += x
	}

	r.AvgRTT /= time.Duration(len(r.RTTs))

	return r
}
