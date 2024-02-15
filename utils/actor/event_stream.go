package actor

import (
	"context"
	"log/slog"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

type eventSub struct {
	pid *pb.PID
}

type eventUnsub struct {
	pid *pb.PID
}

type eventStream struct {
	subs map[*pb.PID]bool
}

func newEventStream() Producer {
	return func() Receiver {
		return &eventStream{
			subs: make(map[*pb.PID]bool),
		}
	}
}

func (e *eventStream) Receive(c *Context) {
	switch msg := c.Message().(type) {
	case eventSub:
		e.subs[msg.pid] = true
	case eventUnsub:
		delete(e.subs, msg.pid)
	default:
		logMsg, ok := c.Message().(EventLogger)
		if ok {
			level, msg, attr := logMsg.Log()
			slog.Log(context.Background(), level, msg, attr...)
		}
		for sub := range e.subs {
			c.Forward(sub)
		}
	}
}
