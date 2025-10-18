package rediskey

import "fmt"

// Tenant keys (global convention across services)
const (
	TenantPrefix       = "tenant"
	TenantCodePrefix   = "tenant:code"
	TenantDomainPrefix = "tenant:domain"
)

func NamespaceKey(namespace, key string) string {
	return fmt.Sprintf("%s:%s", namespace, key)
}

// BuildTenantIDKey returns "tenant:{tenantID}"
func BuildTenantIDKey(tenantID string) string {
	return NamespaceKey(TenantPrefix, tenantID)
}

// BuildTenantCodeKey returns "tenant:code:{tenantCode}"
func BuildTenantCodeKey(code string) string {
	return NamespaceKey(TenantCodePrefix, code)
}

// BuildTenantDomainKey returns "tenant:domain:{domain}"
func BuildTenantDomainKey(domain string) string {
	return NamespaceKey(TenantDomainPrefix, domain)
}
