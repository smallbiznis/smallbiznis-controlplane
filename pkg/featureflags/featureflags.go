package featureflags

import (
	"context"

	"smallbiznis-controlplane/pkg/config"

	"github.com/Flagsmith/flagsmith-go-client/v2"
	"go.uber.org/fx"
)

var Module = fx.Module("featureflags", fx.Provide(ProvideFeatureFlag))

type FeatureFlag interface {
	Features(ctx context.Context, identifier string) ([]flagsmith.Flag, error)
	Flags(ctx context.Context, identifier string, traits ...*flagsmith.Trait) (flagsmith.Flags, error)
}

type featureflag struct {
	client *flagsmith.Client
}

type FeatureParams struct {
	fx.In
	Config *config.Config
}

func ProvideFeatureFlag(p FeatureParams) FeatureFlag {
	if p.Config.Flagsmith.ApiKey == "" {
		return &featureflag{}
	}

	opts := []flagsmith.Option{
		flagsmith.WithBaseURL(p.Config.Flagsmith.Addr),
		flagsmith.WithAnalytics(),
	}

	return &featureflag{
		client: flagsmith.NewClient(p.Config.Flagsmith.ApiKey, opts...),
	}
}

func (s *featureflag) Features(ctx context.Context, identifier string) ([]flagsmith.Flag, error) {
	flags, err := s.client.GetEnvironmentFlags()
	if err != nil {
		return nil, err
	}

	return flags.AllFlags(), nil
}

func (s *featureflag) Flags(ctx context.Context, identifier string, traits ...*flagsmith.Trait) (flagsmith.Flags, error) {
	var traitSlice []*flagsmith.Trait
	if len(traits) > 0 {
		traitSlice = traits
	}

	return s.client.GetIdentityFlags(identifier, traitSlice)
}
