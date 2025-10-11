package license

import (
	licensev1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/license/v1"
)

type Service struct {
	licensev1.UnimplementedLicenseServiceServer
}

func NewService() *Service {
	return &Service{}
}
