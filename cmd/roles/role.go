package roles

import (
	"encoding/json"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/sjy-dv/gokafka-lite/utils/component"
)

func SetRole() {
	config, err := os.Open("gokafka.config.json")
	if err != nil {
		logrus.WithField("action", "startup").WithError(err).
			Fatal("invalid config")
	}
	bytes, err := io.ReadAll(config)
	if err != nil {
		logrus.WithField("action", "startup").WithError(err).
			Fatal("open config file failed")
	}
	var cfg component.ConfigSchema
	err = json.Unmarshal(bytes, &cfg)
	if err != nil {
		logrus.WithField("action", "startup").WithError(err).
			Fatal("config schema unmatched expected json")
	}
	errCh := make(chan error, 1)
	defer close(errCh)
	go func() {
		errCh <- ensureRole(&cfg)
	}()
	if err := <-errCh; err != nil {
		logrus.WithField("action", "failed to start role").WithError(err).
			Fatal("role is unavailable for cluster")
	}
}

func ensureRole(loadCfg *component.ConfigSchema) error {
	baseTable := &component.BaseTable{}
	baseTable = baseTable.Get(loadCfg.Schema)
	for _, schema := range loadCfg.Schema {
		if schema.Protocol == "http" {
			if schema.TlsEnabled {
				logrus.WithField("action", "tls-configure").WithError(
					errors.New("unsupported tls configuration version"),
				).Fatal("this version is not supported tls configuration")
			}
			go func() {
				// if err := socket.InitSocketInterface(baseTable.SocketBase); err != nil {
				// 	runtime.Goexit()
				// }
			}()
		}
		if schema.Protocol == "grpc" {
			if schema.TlsEnabled {
				logrus.WithField("action", "tls-configure").WithError(
					errors.New("unsupported tls configuration version"),
				).Fatal("this version is not supported tls configuration")
			}
			//init grpc
		}
	}
	return nil
}
