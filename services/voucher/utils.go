package voucher

// =========================================================
// Helpers
// =========================================================
func maskCode(enc string) string {
	if len(enc) < 6 {
		return "***"
	}
	return enc[:3] + "****" + enc[len(enc)-3:]
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
