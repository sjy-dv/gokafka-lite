package actor

import (
	"bytes"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/DataDog/gostackparse"
	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

type Envelope struct {
	Msg    any
	Sender *pb.PID
}

type Processer interface {
	Start()
	PID() *pb.PID
	Send(*pb.PID, any, *pb.PID)
	Invoke([]Envelope)
	Shutdown(*sync.WaitGroup)
}

type process struct {
	Opts

	inbox    Inboxer
	context  *Context
	pid      *pb.PID
	restarts int32
	mbuffer  []Envelope
}

func newProcess(e *Engine, opts Opts) *process {
	pid := NewPID(e.address, opts.Kind+pidSeparator+opts.ID)
	ctx := newContext(opts.Context, e, pid.PID)
	p := &process{
		pid:     pid.PID,
		inbox:   NewInbox(opts.InboxSize),
		Opts:    opts,
		context: ctx,
		mbuffer: nil,
	}
	p.inbox.Start(p)
	return p
}

func applyMiddleware(rcv ReceiveFunc, middleware ...MiddlewareFunc) ReceiveFunc {
	for i := len(middleware) - 1; i >= 0; i-- {
		rcv = middleware[i](rcv)
	}
	return rcv
}

func (p *process) Invoke(msgs []Envelope) {
	var (
		nmsg = len(msgs)

		nproc = 0

		processed = 0
	)
	defer func() {
		if v := recover(); v != nil {
			p.context.message = Stopped{}
			p.context.receiver.Receive(p.context)

			p.mbuffer = make([]Envelope, nmsg-nproc)
			for i := 0; i < nmsg-nproc; i++ {
				p.mbuffer[i] = msgs[i+nproc]
			}
			p.tryRestart(v)
		}
	}()
	for i := 0; i < len(msgs); i++ {
		nproc++
		msg := msgs[i]
		if pill, ok := msg.Msg.(poisonPill); ok {
			if pill.graceful {
				msgsToProcess := msgs[processed:]
				for _, m := range msgsToProcess {
					p.invokeMsg(m)
				}
			}
			p.cleanup(pill.wg)
			return
		}
		p.invokeMsg(msg)
		processed++
	}
}

func (p *process) invokeMsg(msg Envelope) {
	if _, ok := msg.Msg.(poisonPill); ok {
		return
	}
	p.context.message = msg.Msg
	p.context.sender = msg.Sender
	recv := p.context.receiver
	if len(p.Opts.Middleware) > 0 {
		applyMiddleware(recv.Receive, p.Opts.Middleware...)(p.context)
	} else {
		recv.Receive(p.context)
	}
}

func (p *process) Start() {
	recv := p.Producer()
	p.context.receiver = recv
	defer func() {
		if v := recover(); v != nil {
			p.context.message = Stopped{}
			p.context.receiver.Receive(p.context)
			p.tryRestart(v)
		}
	}()
	p.context.message = Initialized{}
	applyMiddleware(recv.Receive, p.Opts.Middleware...)(p.context)
	p.context.engine.BroadcastEvent(ActorInitializedEvent{PID: p.pid, Timestamp: time.Now()})

	p.context.message = Started{}
	applyMiddleware(recv.Receive, p.Opts.Middleware...)(p.context)
	p.context.engine.BroadcastEvent(ActorStartedEvent{PID: p.pid, Timestamp: time.Now()})
	if len(p.mbuffer) > 0 {
		p.Invoke(p.mbuffer)
		p.mbuffer = nil
	}
}

func (p *process) tryRestart(v any) {
	if msg, ok := v.(*InternalError); ok {
		slog.Error(msg.From, "err", msg.Err)
		time.Sleep(p.Opts.RestartDelay)
		p.Start()
		return
	}
	stackTrace := cleanTrace(debug.Stack())
	if p.restarts == p.MaxRestarts {
		p.context.engine.BroadcastEvent(ActorMaxRestartsExceededEvent{
			PID:       p.pid,
			Timestamp: time.Now(),
		})
		p.cleanup(nil)
		return
	}

	p.restarts++
	p.context.engine.BroadcastEvent(ActorRestartedEvent{
		PID:        p.pid,
		Timestamp:  time.Now(),
		Stacktrace: stackTrace,
		Reason:     v,
		Restarts:   p.restarts,
	})
	time.Sleep(p.Opts.RestartDelay)
	p.Start()
}

func (p *process) cleanup(wg *sync.WaitGroup) {
	if p.context.parentCtx != nil {
		p.context.parentCtx.children.Delete(p.Kind)
	}

	if p.context.children.Len() > 0 {
		children := p.context.Children()
		for _, pid := range children {
			p.context.engine.Poison(pid).Wait()
		}
	}

	p.inbox.Stop()
	p.context.engine.Registry.Remove(p.pid)
	p.context.message = Stopped{}
	applyMiddleware(p.context.receiver.Receive, p.Opts.Middleware...)(p.context)

	p.context.engine.BroadcastEvent(ActorStoppedEvent{PID: p.pid, Timestamp: time.Now()})
	if wg != nil {
		wg.Done()
	}
}

func (p *process) PID() *pb.PID { return p.pid }
func (p *process) Send(_ *pb.PID, msg any, sender *pb.PID) {
	p.inbox.Send(Envelope{Msg: msg, Sender: sender})
}
func (p *process) Shutdown(wg *sync.WaitGroup) { p.cleanup(wg) }

func cleanTrace(stack []byte) []byte {
	goros, err := gostackparse.Parse(bytes.NewReader(stack))
	if err != nil {
		slog.Error("failed to parse stacktrace", "err", err)
		return stack
	}
	if len(goros) != 1 {
		slog.Error("expected only one goroutine", "goroutines", len(goros))
		return stack
	}
	// skip the first frames:
	goros[0].Stack = goros[0].Stack[4:]
	buf := bytes.NewBuffer(nil)
	_, _ = fmt.Fprintf(buf, "goroutine %d [%s]\n", goros[0].ID, goros[0].State)
	for _, frame := range goros[0].Stack {
		_, _ = fmt.Fprintf(buf, "%s\n", frame.Func)
		_, _ = fmt.Fprint(buf, "\t", frame.File, ":", frame.Line, "\n")
	}
	return buf.Bytes()
}
