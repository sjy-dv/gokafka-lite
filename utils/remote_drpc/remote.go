package remote_drpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/actor"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
)

type Config struct {
	TLSConfig *tls.Config
	// Wg        *sync.WaitGroup
}

func NewConfig() Config {
	return Config{}
}

func (c Config) WithTLS(tlsconf *tls.Config) Config {
	c.TLSConfig = tlsconf
	return c
}

type Remote struct {
	addr            string
	engine          *actor.Engine
	config          Config
	streamReader    *streamReader
	streamRouterPID *pb.PID
	stopCh          chan struct{}
	stopWg          *sync.WaitGroup
	state           atomic.Uint32
}

const (
	stateInvalid uint32 = iota
	stateInitialized
	stateRunning
	stateStopped
)

func New(addr string, config Config) *Remote {
	r := &Remote{
		addr:   addr,
		config: config,
	}
	r.state.Store(stateInitialized)
	r.streamReader = newStreamReader(r)
	return r
}

func (r *Remote) Start(e *actor.Engine) error {
	if r.state.Load() != stateInitialized {
		return fmt.Errorf("remote already started")
	}
	r.state.Store(stateRunning)
	r.engine = e
	var ln net.Listener
	var err error
	switch r.config.TLSConfig {
	case nil:
		ln, err = net.Listen("tcp", r.addr)
	default:
		slog.Debug("remote using TLS for listening")
		ln, err = tls.Listen("tcp", r.addr, r.config.TLSConfig)
	}
	if err != nil {
		return fmt.Errorf("remote failed to listen: %w", err)
	}
	slog.Debug("listening", "addr", r.addr)
	mux := drpcmux.New()
	err = pb.DRPCRegisterRemote(mux, r.streamReader)
	if err != nil {
		return fmt.Errorf("failed to register remote: %w", err)
	}
	s := drpcserver.New(mux)

	r.streamRouterPID = r.engine.Spawn(
		newStreamRouter(r.engine, r.config.TLSConfig),
		"router", actor.WithInboxSize(1024*1024))
	slog.Debug("server started", "listenAddr", r.addr)
	r.stopWg = &sync.WaitGroup{}
	r.stopWg.Add(1)
	r.stopCh = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer r.stopWg.Done()
		err := s.Serve(ctx, ln)
		if err != nil {
			slog.Error("drpcserver", "err", err)
		} else {
			slog.Debug("drpcserver stopped")
		}
	}()

	go func() {
		<-r.stopCh
		cancel()
	}()
	return nil
}

func (r *Remote) Stop() *sync.WaitGroup {
	if r.state.Load() != stateRunning {
		slog.Warn("remote already stopped but stop was called", "state", r.state.Load())
		return &sync.WaitGroup{}
	}
	r.state.Store(stateStopped)
	r.stopCh <- struct{}{}
	return r.stopWg
}

func (r *Remote) Send(pid *pb.PID, msg any, sender *pb.PID) {
	r.engine.Send(r.streamRouterPID, &streamDeliver{
		target: pid,
		sender: sender,
		msg:    msg,
	})
}

func (r *Remote) Address() string {
	return r.addr
}

func init() {
	RegisterType(&pb.PID{})
}
