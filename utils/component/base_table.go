package component

type BaseTable struct {
	SocketBase int32
	GrpcBase   int32
}

func (b *BaseTable) Get(schemes []internalOptions) *BaseTable {
	for _, scheme := range schemes {
		if scheme.Protocol == "websocket" {
			if scheme.Port == 0 {
				b.SocketBase = b.initsock()
			} else {
				b.SocketBase = scheme.Port
			}
		}
		if scheme.Protocol == "grpc" {
			if scheme.Port == 0 {
				b.GrpcBase = b.initgrpc()
			} else {
				b.GrpcBase = scheme.Port
			}
		}
	}
	return b
}

func (b *BaseTable) initsock() int32 {
	return 6666
}

func (b *BaseTable) initgrpc() int32 {
	return 50051
}
