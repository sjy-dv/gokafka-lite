package component

import "sync"

type ComponentParam struct {
	once sync.Once

	HttpProtocol  ProtocolCfg
	HttpsProtocol ProtocolCfg
	GrpcProtocol  GrpcProtocolCfg
}
