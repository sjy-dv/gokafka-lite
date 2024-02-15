package cluster

import pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"

type MemberSet struct {
	members map[string]*pb.Member
}

func NewMemberSet(members ...*pb.Member) *MemberSet {
	m := make(map[string]*pb.Member)
	for _, member := range members {
		m[member.ID] = member
	}
	return &MemberSet{
		members: m,
	}
}

func (s *MemberSet) Len() int {
	return len(s.members)
}

func (s *MemberSet) GetByHost(host string) *pb.Member {
	var theMember *pb.Member
	for _, member := range s.members {
		if member.Host == host {
			theMember = member
		}
	}
	return theMember
}

func (s *MemberSet) Add(member *pb.Member) {
	s.members[member.ID] = member
}

func (s *MemberSet) Contains(member *pb.Member) bool {
	_, ok := s.members[member.ID]
	return ok
}

func (s *MemberSet) Remove(member *pb.Member) {
	delete(s.members, member.ID)
}

func (s *MemberSet) RemoveByHost(host string) {
	member := s.GetByHost(host)
	if member != nil {
		s.Remove(member)
	}
}

func (s *MemberSet) Slice() []*pb.Member {
	members := make([]*pb.Member, len(s.members))
	i := 0
	for _, member := range s.members {
		members[i] = member
		i++
	}
	return members
}

func (s *MemberSet) ForEach(fun func(m *pb.Member) bool) {
	for _, s := range s.members {
		if !fun(s) {
			break
		}
	}
}

func (s *MemberSet) Except(members []*pb.Member) []*pb.Member {
	var (
		except = []*pb.Member{}
		m      = make(map[string]*pb.Member)
	)
	for _, member := range members {
		m[member.ID] = member
	}
	for _, member := range s.members {
		if _, ok := m[member.ID]; !ok {
			except = append(except, member)
		}
	}
	return except
}

func (s *MemberSet) FilterByKind(kind string) []*pb.Member {
	members := []*pb.Member{}
	for _, member := range s.members {
		if HasKind(member, kind) {
			members = append(members, member)
		}
	}
	return members
}
