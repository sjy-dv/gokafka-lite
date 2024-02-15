package remote_drpc

import (
	"crypto/tls"
	"log/slog"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/actor"
)

type streamDeliver struct {
	sender *pb.PID
	target *pb.PID
	msg    any
}

type terminateStream struct {
	address string
}

type streamRouter struct {
	engine *actor.Engine
	// streams is a map of remote address to stream writer pid.
	streams   map[string]*pb.PID
	pid       *pb.PID
	tlsConfig *tls.Config
}

func newStreamRouter(e *actor.Engine, tlsConfig *tls.Config) actor.Producer {
	return func() actor.Receiver {
		return &streamRouter{
			streams:   make(map[string]*pb.PID),
			engine:    e,
			tlsConfig: tlsConfig,
		}
	}
}

func (s *streamRouter) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case actor.Started:
		s.pid = ctx.PID()
	case *streamDeliver:
		s.deliverStream(msg)
	case terminateStream:
		s.handleTerminateStream(msg)
	}
}

func (s *streamRouter) handleTerminateStream(msg terminateStream) {
	streamWriterPID := s.streams[msg.address]
	delete(s.streams, msg.address)
	slog.Debug("terminating stream",
		"remote", msg.address,
		"pid", streamWriterPID,
	)
}

func (s *streamRouter) deliverStream(msg *streamDeliver) {
	var (
		swpid   *pb.PID
		ok      bool
		address = msg.target.Address
	)

	swpid, ok = s.streams[address]
	if !ok {
		swpid = s.engine.SpawnProc(newStreamWriter(s.engine, s.pid, address, s.tlsConfig))
		s.streams[address] = swpid
	}

	s.engine.Send(swpid, msg)
}
