package actor

import (
	"context"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

type Response struct {
	engine  *Engine
	pid     *pb.PID
	result  chan any
	timeout time.Duration
}

func NewResponse(e *Engine, timeout time.Duration) *Response {
	return &Response{
		engine:  e,
		result:  make(chan any, 1),
		timeout: timeout,
		pid:     NewPID(e.address, "response"+pidSeparator+strconv.Itoa(rand.Intn(math.MaxInt32))).PID,
	}
}

func (r *Response) Result() (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer func() {
		cancel()
		r.engine.Registry.Remove(r.pid)
	}()

	select {
	case resp := <-r.result:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *Response) Send(_ *pb.PID, msg any, _ *pb.PID) {
	r.result <- msg
}

func (r *Response) PID() *pb.PID               { return r.pid }
func (r *Response) Shutdown(_ *sync.WaitGroup) {}
func (r *Response) Start()                     {}
func (r *Response) Invoke([]Envelope)          {}
