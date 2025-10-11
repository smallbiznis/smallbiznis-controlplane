package secretmanager

import (
	vault "github.com/hashicorp/vault-client-go"
	"go.uber.org/fx"
)

var Module = fx.Module("secretmanager", fx.Provide(ProvideVault))

func ProvideVault() (*vault.Client, error) {
	client, err := vault.New(
		vault.WithEnvironment(),
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}
