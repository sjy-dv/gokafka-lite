package remote_drpc

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/actor"
	"storj.io/drpc/drpcconn"
)

const (
	connIdleTimeout       = time.Minute * 10
	streamWriterBatchSize = 1024
)

type streamWriter struct {
	writeToAddr string
	rawconn     net.Conn
	conn        *drpcconn.Conn
	stream      pb.DRPCRemote_ReceiveStream
	engine      *actor.Engine
	routerPID   *pb.PID
	pid         *pb.PID
	inbox       actor.Inboxer
	serializer  Serializer
	tlsConfig   *tls.Config
}

func newStreamWriter(e *actor.Engine, rpid *pb.PID, address string, tlsConfig *tls.Config) actor.Processer {
	return &streamWriter{
		writeToAddr: address,
		engine:      e,
		routerPID:   rpid,
		inbox:       actor.NewInbox(streamWriterBatchSize),
		pid:         actor.NewPID(e.Address(), "stream"+"/"+address).PID,
		serializer:  ProtoSerializer{},
		tlsConfig:   tlsConfig,
	}
}

func (s *streamWriter) PID() *pb.PID { return s.pid }
func (s *streamWriter) Send(_ *pb.PID, msg any, sender *pb.PID) {
	s.inbox.Send(actor.Envelope{Msg: msg, Sender: sender})
}

func (s *streamWriter) Invoke(msgs []actor.Envelope) {
	var (
		typeLookup   = make(map[string]int32)
		typeNames    = make([]string, 0)
		senderLookup = make(map[uint64]int32)
		senders      = make([]*pb.PID, 0)
		targetLookup = make(map[uint64]int32)
		targets      = make([]*pb.PID, 0)
		messages     = make([]*pb.Message, len(msgs))
	)

	for i := 0; i < len(msgs); i++ {
		var (
			stream   = msgs[i].Msg.(*streamDeliver)
			typeID   int32
			senderID int32
			targetID int32
		)
		typeID, typeNames = lookupTypeName(typeLookup, s.serializer.TypeName(stream.msg), typeNames)
		senderID, senders = lookupPIDs(senderLookup, stream.sender, senders)
		targetID, targets = lookupPIDs(targetLookup, stream.target, targets)

		b, err := s.serializer.Serialize(stream.msg)
		if err != nil {
			slog.Error("serialize", "err", err)
			continue
		}

		messages[i] = &pb.Message{
			Data:          b,
			TypeNameIndex: typeID,
			SenderIndex:   senderID,
			TargetIndex:   targetID,
		}
	}

	env := &pb.Envelope{
		Senders:   senders,
		Targets:   targets,
		TypeNames: typeNames,
		Messages:  messages,
	}

	if err := s.stream.Send(env); err != nil {
		if errors.Is(err, io.EOF) {
			_ = s.conn.Close()
			return
		}
		slog.Error("stream writer failed sending message",
			"err", err,
		)
	}
	// refresh the connection deadline.
	err := s.rawconn.SetDeadline(time.Now().Add(connIdleTimeout))
	if err != nil {
		slog.Error("failed to set context deadline", "err", err)
	}
}

func (s *streamWriter) init() {
	var (
		rawconn    net.Conn
		err        error
		delay      time.Duration = time.Millisecond * 500
		maxRetries               = 3
	)
	for i := 0; i < maxRetries; i++ {
		// Here we try to connect to the remote address.
		// Todo: can we make an Event here in case of failure?
		switch s.tlsConfig {
		case nil:
			rawconn, err = net.Dial("tcp", s.writeToAddr)
			if err != nil {
				d := time.Duration(delay * time.Duration(i*2))
				slog.Error("net.Dial", "err", err, "remote", s.writeToAddr, "retry", i, "max", maxRetries, "delay", d)
				time.Sleep(d)
				continue
			}
		default:
			slog.Debug("remote using TLS for writing")
			rawconn, err = tls.Dial("tcp", s.writeToAddr, s.tlsConfig)
			if err != nil {
				d := time.Duration(delay * time.Duration(i*2))
				slog.Error("tls.Dial", "err", err, "remote", s.writeToAddr, "retry", i, "max", maxRetries, "delay", d)
				time.Sleep(d)
				continue
			}
		}
		break
	}
	// We could not reach the remote after retrying N times. Hence, shutdown the stream writer.
	// and notify RemoteUnreachableEvent.
	if rawconn == nil {
		evt := actor.RemoteUnreachableEvent{
			ListenAddr: s.writeToAddr,
		}
		s.engine.BroadcastEvent(evt)
		s.Shutdown(nil)
		return
	}

	s.rawconn = rawconn
	err = rawconn.SetDeadline(time.Now().Add(connIdleTimeout))
	if err != nil {
		slog.Error("failed to set deadline on raw connection", "err", err)
		return
	}

	conn := drpcconn.New(rawconn)
	client := pb.NewDRPCRemoteClient(conn)

	stream, err := client.Receive(context.Background())
	if err != nil {
		slog.Error("receive", "err", err, "remote", s.writeToAddr)
		s.Shutdown(nil)
		return
	}

	s.stream = stream
	s.conn = conn

	slog.Debug("connected",
		"remote", s.writeToAddr,
	)

	go func() {
		<-s.conn.Closed()
		slog.Debug("lost connection",
			"remote", s.writeToAddr,
		)
		s.Shutdown(nil)
	}()
}

func (s *streamWriter) Shutdown(wg *sync.WaitGroup) {
	s.engine.Send(s.routerPID, terminateStream{address: s.writeToAddr})
	if s.stream != nil {
		s.stream.Close()
	}
	s.inbox.Stop()
	s.engine.Registry.Remove(s.PID())
	if wg != nil {
		wg.Done()
	}
}

func (s *streamWriter) Start() {
	s.inbox.Start(s)
	s.init()
}

func lookupPIDs(m map[uint64]int32, pid *pb.PID, pids []*pb.PID) (int32, []*pb.PID) {
	if pid == nil {
		return 0, pids
	}
	max := int32(len(m))
	key := actor.LookupKey(pid)
	id, ok := m[key]
	if !ok {
		m[key] = max
		id = max
		pids = append(pids, pid)
	}
	return id, pids

}

func lookupTypeName(m map[string]int32, name string, types []string) (int32, []string) {
	max := int32(len(m))
	id, ok := m[name]
	if !ok {
		m[name] = max
		id = max
		types = append(types, name)
	}
	return id, types
}
