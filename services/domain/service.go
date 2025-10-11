package domain

import (
	domainv1 "github.com/smallbiznis/go-genproto/smallbiznis/controlplane/domain/v1"
)

type Service struct {
	domainv1.UnimplementedDomainServiceServer
}

func NewService() *Service {
	return &Service{}
}
