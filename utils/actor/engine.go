package actor

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

type Remoter interface {
	Address() string
	Send(*pb.PID, any, *pb.PID)
	Start(*Engine) error
	Stop() *sync.WaitGroup
}

type Producer func() Receiver

type Receiver interface {
	Receive(*Context)
}

type Engine struct {
	Registry    *Registry
	address     string
	remote      Remoter
	eventStream *pb.PID
}

type EngineConfig struct {
	remote Remoter
}

func NewEngineConfig() EngineConfig {
	return EngineConfig{}
}

func (config EngineConfig) WithRemote(remote Remoter) EngineConfig {
	config.remote = remote
	return config
}

func NewEngine(config EngineConfig) (*Engine, error) {
	e := &Engine{}
	e.Registry = newRegistry(e)
	e.address = LocalLookupAddr
	if config.remote != nil {
		e.remote = config.remote
		e.address = config.remote.Address()
		err := config.remote.Start(e)
		if err != nil {
			return nil, fmt.Errorf("failed to start remote: %w", err)
		}
	}
	e.eventStream = e.Spawn(newEventStream(), "eventstream")
	return e, nil
}

func (e *Engine) Spawn(p Producer, kind string, opts ...OptFunc) *pb.PID {
	options := DefaultOpts(p)
	options.Kind = kind
	for _, opt := range opts {
		opt(&options)
	}

	if len(options.ID) == 0 {
		id := strconv.Itoa(rand.Intn(math.MaxInt))
		options.ID = id
	}
	proc := newProcess(e, options)
	return e.SpawnProc(proc)
}

func (e *Engine) SpawnFunc(f func(*Context), kind string, opts ...OptFunc) *pb.PID {
	return e.Spawn(newFuncReceiver(f), kind, opts...)
}

func (e *Engine) SpawnProc(p Processer) *pb.PID {
	e.Registry.add(p)
	p.Start()
	return p.PID()
}

func (e *Engine) Address() string {
	return e.address
}

func (e *Engine) Request(pid *pb.PID, msg any, timeout time.Duration) *Response {
	resp := NewResponse(e, timeout)
	e.Registry.add(resp)

	e.SendWithSender(pid, msg, resp.PID())

	return resp
}

func (e *Engine) SendWithSender(pid *pb.PID, msg any, sender *pb.PID) {
	e.send(pid, msg, sender)
}

func (e *Engine) Send(pid *pb.PID, msg any) {
	e.send(pid, msg, nil)
}

func (e *Engine) BroadcastEvent(msg any) {
	if e.eventStream != nil {
		e.send(e.eventStream, msg, nil)
	}
}

func (e *Engine) send(pid *pb.PID, msg any, sender *pb.PID) {

	if pid == nil {
		return
	}
	if e.isLocalMessage(pid) {
		e.SendLocal(pid, msg, sender)
		return
	}
	if e.remote == nil {
		e.BroadcastEvent(EngineRemoteMissingEvent{Target: pid, Sender: sender, Message: msg})
		return
	}
	e.remote.Send(pid, msg, sender)
}

type SendRepeater struct {
	engine   *Engine
	self     *pb.PID
	target   *pb.PID
	msg      any
	interval time.Duration
	cancelch chan struct{}
}

func (sr SendRepeater) start() {
	ticker := time.NewTicker(sr.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				sr.engine.SendWithSender(sr.target, sr.msg, sr.self)
			case <-sr.cancelch:
				ticker.Stop()
				return
			}
		}
	}()
}

func (sr SendRepeater) Stop() {
	close(sr.cancelch)
}

func (e *Engine) SendRepeat(pid *pb.PID, msg any, interval time.Duration) SendRepeater {
	clonedPID := *pid.CloneVT()
	sr := SendRepeater{
		engine:   e,
		self:     nil,
		target:   &clonedPID,
		interval: interval,
		msg:      msg,
		cancelch: make(chan struct{}, 1),
	}
	sr.start()
	return sr
}

func (e *Engine) Stop(pid *pb.PID, wg ...*sync.WaitGroup) *sync.WaitGroup {
	return e.sendPoisonPill(pid, false, wg...)
}

func (e *Engine) Poison(pid *pb.PID, wg ...*sync.WaitGroup) *sync.WaitGroup {
	return e.sendPoisonPill(pid, true, wg...)
}

func (e *Engine) sendPoisonPill(pid *pb.PID, graceful bool, wg ...*sync.WaitGroup) *sync.WaitGroup {
	var _wg *sync.WaitGroup
	if len(wg) > 0 {
		_wg = wg[0]
	} else {
		_wg = &sync.WaitGroup{}
	}
	_wg.Add(1)
	proc := e.Registry.get(pid)

	if proc == nil {
		e.BroadcastEvent(DeadLetterEvent{
			Target:  pid,
			Message: poisonPill{_wg, graceful},
			Sender:  nil,
		})
		return _wg
	}
	pill := poisonPill{
		wg:       _wg,
		graceful: graceful,
	}
	if proc != nil {
		e.SendLocal(pid, pill, nil)
	}
	return _wg
}

func (e *Engine) SendLocal(pid *pb.PID, msg any, sender *pb.PID) {
	proc := e.Registry.get(pid)
	if proc == nil {

		e.BroadcastEvent(DeadLetterEvent{
			Target:  pid,
			Message: msg,
			Sender:  sender,
		})
		return
	}
	proc.Send(pid, msg, sender)
}

func (e *Engine) Subscribe(pid *pb.PID) {
	e.Send(e.eventStream, eventSub{pid: pid})
}

func (e *Engine) Unsubscribe(pid *pb.PID) {
	e.Send(e.eventStream, eventUnsub{pid: pid})
}

func (e *Engine) isLocalMessage(pid *pb.PID) bool {
	if pid == nil {
		return false
	}
	return e.address == pid.Address
}

type funcReceiver struct {
	f func(*Context)
}

func newFuncReceiver(f func(*Context)) Producer {
	return func() Receiver {
		return &funcReceiver{
			f: f,
		}
	}
}

func (r *funcReceiver) Receive(c *Context) {
	r.f(c)
}
