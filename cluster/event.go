package cluster

import pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"

type MemberJoinEvent struct {
	Member *pb.Member
}

type MemberLeaveEvent struct {
	Member *pb.Member
}

type ActivationEvent struct {
	PID *pb.PID
}

type DeactivationEvent struct {
	PID *pb.PID
}
