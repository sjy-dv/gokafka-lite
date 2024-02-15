package actor

import (
	"sync"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

const LocalLookupAddr = "local"

type Registry struct {
	mu     sync.RWMutex
	lookup map[string]Processer
	engine *Engine
}

func newRegistry(e *Engine) *Registry {
	return &Registry{
		lookup: make(map[string]Processer, 1024),
		engine: e,
	}
}

func (r *Registry) GetPID(kind, id string) *pb.PID {
	proc := r.getByID(kind + pidSeparator + id)
	if proc != nil {
		return proc.PID()
	}
	return nil
}

func (r *Registry) Remove(pid *pb.PID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.lookup, pid.ID)
}

func (r *Registry) get(pid *pb.PID) Processer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if proc, ok := r.lookup[pid.ID]; ok {
		return proc
	}
	return nil // didn't find the processer
}

func (r *Registry) getByID(id string) Processer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lookup[id]
}

func (r *Registry) add(proc Processer) {
	r.mu.Lock()
	id := proc.PID().ID
	if _, ok := r.lookup[id]; ok {
		r.mu.Unlock()
		r.engine.BroadcastEvent(ActorDuplicateIdEvent{PID: proc.PID()})
		return
	}
	r.lookup[id] = proc
	r.mu.Unlock()
}
