package actor

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"strconv"
	"time"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/dmap"
)

type Context struct {
	pid      *pb.PID
	sender   *pb.PID
	engine   *Engine
	receiver Receiver
	message  any

	parentCtx *Context
	children  *dmap.DMap[string, *pb.PID]
	context   context.Context
}

func newContext(ctx context.Context, e *Engine, pid *pb.PID) *Context {
	return &Context{
		context:  ctx,
		engine:   e,
		pid:      pid,
		children: dmap.New[string, *pb.PID](),
	}
}

func (c *Context) Context() context.Context {
	return c.context
}

func (c *Context) Receiver() Receiver {
	return c.receiver
}

func (c *Context) Request(pid *pb.PID, msg any, timeout time.Duration) *Response {
	return c.engine.Request(pid, msg, timeout)
}

func (c *Context) Respond(msg any) {
	if c.sender == nil {
		slog.Warn("context got no sender", "func", "Respond", "pid", c.PID())
		return
	}
	c.engine.Send(c.sender, msg)
}

func (c *Context) SpawnChild(p Producer, name string, opts ...OptFunc) *pb.PID {
	options := DefaultOpts(p)
	options.Kind = c.PID().ID + pidSeparator + name
	for _, opt := range opts {
		opt(&options)
	}

	if len(options.ID) == 0 {
		id := strconv.Itoa(rand.Intn(math.MaxInt))
		options.ID = id
	}
	proc := newProcess(c.engine, options)
	proc.context.parentCtx = c
	pid := c.engine.SpawnProc(proc)
	c.children.Set(pid.ID, pid)

	return proc.PID()
}

func (c *Context) SpawnChildFunc(f func(*Context), name string, opts ...OptFunc) *pb.PID {
	return c.SpawnChild(newFuncReceiver(f), name, opts...)
}

func (c *Context) Send(pid *pb.PID, msg any) {
	c.engine.SendWithSender(pid, msg, c.pid)
}

func (c *Context) SendRepeat(pid *pb.PID, msg any, interval time.Duration) SendRepeater {
	sr := SendRepeater{
		engine:   c.engine,
		self:     c.pid,
		target:   pid.CloneVT(),
		interval: interval,
		msg:      msg,
		cancelch: make(chan struct{}, 1),
	}
	sr.start()
	return sr
}

func (c *Context) Forward(pid *pb.PID) {
	c.engine.SendWithSender(pid, c.message, c.pid)
}

func (c *Context) GetPID(id string) *pb.PID {
	proc := c.engine.Registry.getByID(id)
	if proc != nil {
		return proc.PID()
	}
	return nil
}

func (c *Context) Parent() *pb.PID {
	if c.parentCtx != nil {
		return c.parentCtx.pid
	}
	return nil
}

func (c *Context) Child(id string) *pb.PID {
	pid, _ := c.children.Get(id)
	return pid
}

func (c *Context) Children() []*pb.PID {
	pids := make([]*pb.PID, c.children.Len())
	i := 0
	c.children.ForEach(func(_ string, child *pb.PID) {
		pids[i] = child
		i++
	})
	return pids
}

func (c *Context) PID() *pb.PID {
	return c.pid
}

func (c *Context) Sender() *pb.PID {
	return c.sender
}

func (c *Context) Engine() *Engine {
	return c.engine
}

func (c *Context) Message() any {
	return c.message
}
