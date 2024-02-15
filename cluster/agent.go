package cluster

import (
	"log/slog"
	"reflect"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/actor"
	"github.com/sjy-dv/gokafka-lite/utils/maps"
)

type (
	activate struct {
		kind   string
		config ActivationConfig
	}
	getMembers struct{}
	getKinds   struct{}
	deactivate struct{ pid *pb.PID }
	getActive  struct{ id string }
)

// Agent is an pb/receiver that is responsible for managing the state
// of the cluster.
type Agent struct {
	members    *MemberSet
	cluster    *Cluster
	kinds      map[string]bool
	localKinds map[string]kind
	// All the pbs that are available cluster wide.
	activated map[string]*pb.PID
}

func NewAgent(c *Cluster) actor.Producer {
	kinds := make(map[string]bool)
	localKinds := make(map[string]kind)
	for _, kind := range c.kinds {
		kinds[kind.name] = true
		localKinds[kind.name] = kind
	}
	return func() actor.Receiver {
		return &Agent{
			members:    NewMemberSet(),
			cluster:    c,
			kinds:      kinds,
			localKinds: localKinds,
			activated:  make(map[string]*pb.PID),
		}
	}
}

func (a *Agent) Receive(c *actor.Context) {
	switch msg := c.Message().(type) {
	case actor.Started:
	case actor.Stopped:
	case *pb.ActorTopology:
		a.handleActorTopology(msg)
	case *pb.Members:
		a.handleMembers(msg.Members)
	case *pb.Activation:
		a.handleActivation(msg)
	case activate:
		pid := a.activate(msg.kind, msg.config)
		c.Respond(pid)
	case deactivate:
		a.bcast(&pb.Deactivation{PID: msg.pid})
	case *pb.Deactivation:
		a.handleDeactivation(msg)
	case *pb.ActivationRequest:
		resp := a.handleActivationRequest(msg)
		c.Respond(resp)
	case getMembers:
		c.Respond(a.members.Slice())
	case getKinds:
		kinds := make([]string, len(a.kinds))
		i := 0
		for kind := range a.kinds {
			kinds[i] = kind
			i++
		}
		c.Respond(kinds)
	case getActive:
		pid := a.activated[msg.id]
		c.Respond(pid)
	}
}

func (a *Agent) handleActorTopology(msg *pb.ActorTopology) {
	for _, actorInfo := range msg.Actors {
		a.addActivated(actorInfo.PID)
	}
}

func (a *Agent) handleDeactivation(msg *pb.Deactivation) {
	a.removeActivated(msg.PID)
	a.cluster.engine.Poison(msg.PID)
	a.cluster.engine.BroadcastEvent(DeactivationEvent{PID: msg.PID})
}

// A new kind is activated on this cluster.
func (a *Agent) handleActivation(msg *pb.Activation) {
	a.addActivated(msg.PID)
	a.cluster.engine.BroadcastEvent(ActivationEvent{PID: msg.PID})
}

func (a *Agent) handleActivationRequest(msg *pb.ActivationRequest) *pb.ActivationResponse {
	if !a.hasKindLocal(msg.Kind) {
		slog.Error("received activation request but kind not registered locally on this node", "kind", msg.Kind)
		return &pb.ActivationResponse{Success: false}
	}
	kind := a.localKinds[msg.Kind]
	pid := a.cluster.engine.Spawn(kind.producer, msg.Kind, actor.WithID(msg.ID))
	resp := &pb.ActivationResponse{
		PID:     pid,
		Success: true,
	}
	return resp
}

func (a *Agent) activate(kind string, config ActivationConfig) *pb.PID {
	members := a.members.FilterByKind(kind)
	if len(members) == 0 {
		slog.Warn("could not find any members with kind", "kind", kind)
		return nil
	}
	if config.selectMember == nil {
		config.selectMember = SelectRandomMember
	}
	memberPID := config.selectMember(ActivationDetails{
		Members: members,
		Region:  config.region,
		Kind:    kind,
	})
	if memberPID == nil {
		slog.Warn("activator did not found a member to activate on")
		return nil
	}
	req := &pb.ActivationRequest{Kind: kind, ID: config.id}
	activatorPID := actor.NewPID(memberPID.Host, "cluster/"+memberPID.ID)

	var activationResp *pb.ActivationResponse
	// Local activation
	if memberPID.Host == a.cluster.engine.Address() {
		activationResp = a.handleActivationRequest(req)
	} else {
		// Remote activation
		//
		// TODO: topology hash
		resp, err := a.cluster.engine.Request(activatorPID.PID, req, a.cluster.config.requestTimeout).Result()
		if err != nil {
			slog.Error("failed activation request", "err", err)
			return nil
		}
		r, ok := resp.(*pb.ActivationResponse)
		if !ok {
			slog.Error("expected *ActivationResponse", "msg", reflect.TypeOf(resp))
			return nil
		}
		if !r.Success {
			slog.Error("activation unsuccessful", "msg", r)
			return nil
		}
		activationResp = r
	}

	a.bcast(&pb.Activation{
		PID: activationResp.PID,
	})

	return activationResp.PID
}

func (a *Agent) handleMembers(members []*pb.Member) {
	joined := NewMemberSet(members...).Except(a.members.Slice())
	left := a.members.Except(members)

	for _, member := range joined {
		a.memberJoin(member)
	}
	for _, member := range left {
		a.memberLeave(member)
	}
}

func (a *Agent) memberJoin(member *pb.Member) {
	a.members.Add(member)

	// track cluster wide available kinds
	for _, kind := range member.Kinds {
		if _, ok := a.kinds[kind]; !ok {
			a.kinds[kind] = true
		}
	}

	actorInfos := make([]*pb.ActorInfo, 0)
	for _, pid := range a.activated {
		actorInfo := &pb.ActorInfo{
			PID: pid,
		}
		actorInfos = append(actorInfos, actorInfo)
	}

	// Send our ActorTopology to this member
	if len(actorInfos) > 0 {
		a.cluster.engine.Send(PID(member), &pb.ActorTopology{Actors: actorInfos})
	}

	// Broadcast MemberJoinEvent
	a.cluster.engine.BroadcastEvent(MemberJoinEvent{
		Member: member,
	})

	slog.Debug("[CLUSTER] member joined", "id", member.ID, "host", member.Host, "kinds", member.Kinds, "region", member.Region)
}

func (a *Agent) memberLeave(member *pb.Member) {
	a.members.Remove(member)
	a.rebuildKinds()

	// Remove all the activeKinds that where running on the member that left the cluster.
	for _, pid := range a.activated {
		if pid.Address == member.Host {
			a.removeActivated(pid)
		}
	}

	a.cluster.engine.BroadcastEvent(MemberLeaveEvent{Member: member})

	slog.Debug("[CLUSTER] member left", "id", member.ID, "host", member.Host, "kinds", member.Kinds)
}

func (a *Agent) bcast(msg any) {
	a.members.ForEach(func(member *pb.Member) bool {
		a.cluster.engine.Send(PID(member), msg)
		return true
	})
}

func (a *Agent) addActivated(pid *pb.PID) {
	if _, ok := a.activated[pid.ID]; !ok {
		a.activated[pid.ID] = pid
		slog.Debug("new actor available on cluster", "pid", pid)
	}
}

func (a *Agent) removeActivated(pid *pb.PID) {
	delete(a.activated, pid.ID)
	slog.Debug("actor removed from cluster", "pid", pid)
}

func (a *Agent) hasKindLocal(name string) bool {
	_, ok := a.localKinds[name]
	return ok
}

func (a *Agent) rebuildKinds() {
	maps.Clear(a.kinds)
	a.members.ForEach(func(m *pb.Member) bool {
		for _, kind := range m.Kinds {
			if _, ok := a.kinds[kind]; !ok {
				a.kinds[kind] = true
			}
		}
		return true
	})
}
