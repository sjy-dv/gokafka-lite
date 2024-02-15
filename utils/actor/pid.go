package actor

import (
	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/zeebo/xxh3"
)

const pidSeparator = "/"

type RawPID struct {
	*pb.PID
}

// NewPID returns a new Process ID given an address and an id.
func NewPID(address, id string) *RawPID {
	p := &pb.PID{
		Address: address,
		ID:      id,
	}
	return &RawPID{p}
}

func (pid *RawPID) String() string {
	return pid.Address + pidSeparator + pid.ID
}

func (pid *RawPID) Equals(other *RawPID) bool {
	return pid.Address == other.Address && pid.ID == other.ID
}

func (pid *RawPID) Child(id string) *RawPID {
	childID := pid.ID + pidSeparator + id
	return NewPID(pid.Address, childID)
}

func LookupKey(pid *pb.PID) uint64 {
	key := []byte(pid.Address)
	key = append(key, pid.ID...)
	return xxh3.Hash(key)
}
