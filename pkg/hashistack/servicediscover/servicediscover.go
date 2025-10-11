package servicediscover

import (
	"context"
	"fmt"

	"smallbiznis-controlplane/pkg/config"

	"github.com/hashicorp/consul/api"
	"go.uber.org/fx"
)

func registerConsul(lc fx.Lifecycle) {

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {},
		OnStop:  func(ctx context.Context) error {},
	})

}

type ServiceRegistry interface {
	Register(ctx context.Context) error
	Deregister(ctx context.Context) error
}

type serviceRegistry struct {
	client *api.Client
}

func NewConfig(cfg *config.Config) *api.Config {
	config := api.DefaultConfig()
	config.Address = cfg.Consul.Addr

	return config
}

func NewClient(config *api.Config) (*api.Client, error) {
	return api.NewClient(config)
}

func NewRegistry(client *api.Client) ServiceRegistry {
	return &serviceRegistry{
		client: client,
	}
}

type ConsulRegistry struct {
	client    *api.Client
	serviceID string
	service   *api.AgentServiceRegistration
}

func NewConsulRegistry(address, serviceName, serviceID, host string, port int) (*ConsulRegistry, error) {
	config := api.DefaultConfig()
	config.Address = address

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	service := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host,
		Port:    port,
		Check: &api.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%d/health/readiness", host, port),
			Interval: "10s",
			Timeout:  "5s",
		},
	}

	return &ConsulRegistry{
		client:    client,
		serviceID: serviceID,
		service:   service,
	}, nil
}

func (r *ConsulRegistry) Register(ctx context.Context) error {
	return r.client.Agent().ServiceRegister(r.service)
}

func (r *ConsulRegistry) Deregister(ctx context.Context) error {
	return r.client.Agent().ServiceDeregister(r.serviceID)
}
