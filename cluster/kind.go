package cluster

import "github.com/sjy-dv/gokafka-lite/utils/actor"

type KindConfig struct{}

func NewKindConfig() KindConfig {
	return KindConfig{}
}

type kind struct {
	config   KindConfig
	name     string
	producer actor.Producer
}

func newKind(name string, p actor.Producer, config KindConfig) kind {
	return kind{
		name:     name,
		config:   config,
		producer: p,
	}
}
