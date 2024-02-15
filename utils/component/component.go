package component

import "google.golang.org/grpc/credentials"

type ProtocolCfg bool

type ProtocolPort int16

type rpcCfg struct {
	Enable bool
	Cred   credentials.TransportCredentials
}

type GrpcProtocolCfg rpcCfg

type ConfigSchema struct {
	Schema []internalOptions `json:"schema,omitempty"`
}

type internalOptions struct {
	Protocol   string `json:"protocol"`
	TlsEnabled bool   `json:"tls_enabled"`
	Port       int32  `json:"port"`
}
