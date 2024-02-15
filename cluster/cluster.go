package cluster

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"reflect"
	"sync"
	"time"

	pb "github.com/sjy-dv/gokafka-lite/rpc/generated/drpc/protocol/v1"
	"github.com/sjy-dv/gokafka-lite/utils/actor"
	"github.com/sjy-dv/gokafka-lite/utils/remote_drpc"
)

var defaultRequestTimeout = time.Second

type Producer func(c *Cluster) actor.Producer

type Config struct {
	listenAddr     string
	id             string
	region         string
	engine         *actor.Engine
	provider       Producer
	requestTimeout time.Duration
}

func NewConfig() Config {
	return Config{
		listenAddr:     getRandomListenAddr(),
		id:             fmt.Sprintf("%d", rand.Intn(math.MaxInt)),
		region:         "default",
		provider:       NewSelfManagedProvider(NewSelfManagedConfig()),
		requestTimeout: defaultRequestTimeout,
	}
}

func (config Config) WithRequestTimeout(d time.Duration) Config {
	config.requestTimeout = d
	return config
}

func (config Config) WithProvider(p Producer) Config {
	config.provider = p
	return config
}

func (config Config) WithEngine(e *actor.Engine) Config {
	config.engine = e
	return config
}

func (config Config) WithListenAddr(addr string) Config {
	config.listenAddr = addr
	return config
}

func (config Config) WithID(id string) Config {
	config.id = id
	return config
}

func (config Config) WithRegion(region string) Config {
	config.region = region
	return config
}

type Cluster struct {
	config      Config
	engine      *actor.Engine
	agentPID    *pb.PID
	providerPID *pb.PID
	isStarted   bool
	kinds       []kind
}

func New(config Config) (*Cluster, error) {
	if config.engine == nil {
		remote := remote_drpc.New(config.listenAddr, remote_drpc.NewConfig())
		e, err := actor.NewEngine(actor.NewEngineConfig().WithRemote(remote))
		if err != nil {
			return nil, err
		}
		config.engine = e
	}
	c := &Cluster{
		config: config,
		engine: config.engine,
		kinds:  make([]kind, 0),
	}
	return c, nil
}

func (c *Cluster) Start() {
	c.agentPID = c.engine.Spawn(NewAgent(c), "cluster", actor.WithID(c.config.id))
	c.providerPID = c.engine.Spawn(c.config.provider(c), "provider", actor.WithID(c.config.id))
	c.isStarted = true
}

func (c *Cluster) Stop() *sync.WaitGroup {
	wg := sync.WaitGroup{}
	c.engine.Poison(c.agentPID, &wg)
	c.engine.Poison(c.providerPID, &wg)
	return &wg
}

func (c *Cluster) Spawn(p actor.Producer, id string, opts ...actor.OptFunc) *pb.PID {
	pid := c.engine.Spawn(p, id, opts...)
	members := c.Members()
	for _, member := range members {
		c.engine.Send(PID(member), &pb.Activation{
			PID: pid,
		})
	}
	return pid
}

func (c *Cluster) Activate(kind string, config ActivationConfig) *pb.PID {
	msg := activate{
		kind:   kind,
		config: config,
	}
	resp, err := c.engine.Request(c.agentPID, msg, c.config.requestTimeout).Result()
	if err != nil {
		slog.Error("activation failed", "err", err)
		return nil
	}
	pid, ok := resp.(*pb.PID)
	if !ok {
		slog.Warn("activation expected response of *actor.PID", "got", reflect.TypeOf(resp))
		return nil
	}
	return pid
}

func (c *Cluster) Deactivate(pid *pb.PID) {
	c.engine.Send(c.agentPID, deactivate{pid: pid})
}

func (c *Cluster) RegisterKind(kind string, producer actor.Producer, config KindConfig) {
	if c.isStarted {
		slog.Warn("failed to register kind", "reason", "cluster already started", "kind", kind)
		return
	}
	c.kinds = append(c.kinds, newKind(kind, producer, config))
}

func (c *Cluster) HasKindLocal(name string) bool {
	for _, kind := range c.kinds {
		if kind.name == name {
			return true
		}
	}
	return false
}

func (c *Cluster) Members() []*pb.Member {
	resp, err := c.engine.Request(c.agentPID, getMembers{}, c.config.requestTimeout).Result()
	if err != nil {
		return []*pb.Member{}
	}
	if res, ok := resp.([]*pb.Member); ok {
		return res
	}
	return nil
}

func (c *Cluster) HasKind(name string) bool {
	resp, err := c.engine.Request(c.agentPID, getKinds{}, c.config.requestTimeout).Result()
	if err != nil {
		return false
	}
	if kinds, ok := resp.([]string); ok {
		for _, kind := range kinds {
			if kind == name {
				return true
			}
		}
	}
	return false
}

func (c *Cluster) GetActivated(id string) *pb.PID {
	resp, err := c.engine.Request(c.agentPID, getActive{id: id}, c.config.requestTimeout).Result()
	if err != nil {
		return nil
	}
	if res, ok := resp.(*pb.PID); ok {
		return res
	}
	return nil
}

func (c *Cluster) Member() *pb.Member {
	kinds := make([]string, len(c.kinds))
	for i := 0; i < len(c.kinds); i++ {
		kinds[i] = c.kinds[i].name
	}
	m := &pb.Member{
		ID:     c.config.id,
		Host:   c.engine.Address(),
		Kinds:  kinds,
		Region: c.config.region,
	}
	return m
}

func (c *Cluster) Engine() *actor.Engine {
	return c.engine
}

func (c *Cluster) Region() string {
	return c.config.region
}

func (c *Cluster) ID() string {
	return c.config.id
}

func (c *Cluster) Address() string {
	return c.agentPID.Address
}

func (c *Cluster) PID() *pb.PID {
	return c.agentPID
}

func getRandomListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", rand.Intn(50000)+10000)
}
