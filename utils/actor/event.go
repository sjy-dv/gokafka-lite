package actor

import (
	"log/slog"
	"time"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

type EventLogger interface {
	Log() (slog.Level, string, []any)
}

type ActorStartedEvent struct {
	PID       *pb.PID
	Timestamp time.Time
}

func (e ActorStartedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelDebug, "Actor started", []any{"pid", e.PID}
}

type ActorInitializedEvent struct {
	PID       *pb.PID
	Timestamp time.Time
}

func (e ActorInitializedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelDebug, "Actor initialized", []any{"pid", e.PID}
}

type ActorStoppedEvent struct {
	PID       *pb.PID
	Timestamp time.Time
}

func (e ActorStoppedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelDebug, "Actor stopped", []any{"pid", e.PID}
}

type ActorRestartedEvent struct {
	PID        *pb.PID
	Timestamp  time.Time
	Stacktrace []byte
	Reason     any
	Restarts   int32
}

func (e ActorRestartedEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Actor crashed and restarted",
		[]any{"pid", e.PID.GetID(), "stack", string(e.Stacktrace),
			"reason", e.Reason, "restarts", e.Restarts}
}

type ActorMaxRestartsExceededEvent struct {
	PID       *pb.PID
	Timestamp time.Time
}

func (e ActorMaxRestartsExceededEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Actor crashed too many times", []any{"pid", e.PID.GetID()}
}

type ActorDuplicateIdEvent struct {
	PID *pb.PID
}

func (e ActorDuplicateIdEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Actor name already claimed", []any{"pid", e.PID.GetID()}
}

type EngineRemoteMissingEvent struct {
	Target  *pb.PID
	Sender  *pb.PID
	Message any
}

func (e EngineRemoteMissingEvent) Log() (slog.Level, string, []any) {
	return slog.LevelError, "Engine has no remote", []any{"sender", e.Target.GetID()}
}

type RemoteUnreachableEvent struct {
	ListenAddr string
}

type DeadLetterEvent struct {
	Target  *pb.PID
	Message any
	Sender  *pb.PID
}
