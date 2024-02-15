package cluster

import (
	"fmt"
	"math"
	"math/rand"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
)

type ActivationConfig struct {
	id           string
	region       string
	selectMember SelectMemberFunc
}

func NewActivationConfig() ActivationConfig {
	return ActivationConfig{
		id:           fmt.Sprintf("%d", rand.Intn(math.MaxInt)),
		region:       "default",
		selectMember: SelectRandomMember,
	}
}

func (config ActivationConfig) WithSelectMemberFunc(fun SelectMemberFunc) ActivationConfig {
	config.selectMember = fun
	return config
}

func (config ActivationConfig) WithID(id string) ActivationConfig {
	config.id = id
	return config
}

func (config ActivationConfig) WithRegion(region string) ActivationConfig {
	config.region = region
	return config
}

type SelectMemberFunc func(ActivationDetails) *pb.Member

type ActivationDetails struct {
	Region  string
	Members []*pb.Member
	Kind    string
}

func SelectRandomMember(details ActivationDetails) *pb.Member {
	return details.Members[rand.Intn(len(details.Members))]
}
