package cluster

import (
	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/actor"
)

func memberToProviderPID(m *pb.Member) *pb.PID {
	return actor.NewPID(m.Host, "provider/"+m.ID).PID
}

func NewCID(pid *pb.PID, kind, id, region string) *pb.CID {
	return &pb.CID{
		PID:    pid,
		Kind:   kind,
		ID:     id,
		Region: region,
	}
}

func CIDEquals(cid *pb.CID, other *pb.CID) bool {
	return cid.ID == other.ID && cid.Kind == other.Kind
}

func PID(m *pb.Member) *pb.PID {
	return actor.NewPID(m.Host, "cluster/"+m.ID).PID
}

func Equals(m, other *pb.Member) bool {
	return m.Host == other.Host && m.ID == other.ID
}

func HasKind(m *pb.Member, kind string) bool {
	for _, k := range m.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}
