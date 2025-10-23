package rule

import (
	"fmt"
	"strconv"
	"strings"
)

func makeRuleKey(tenantID, trigger string) string {
	return fmt.Sprintf("%s:%s", tenantID, trigger)
}

func encodeCursor(priority int32, ruleID string) string {
	return fmt.Sprintf("%d:%s", priority, ruleID)
}

func decodeCursor(cursor string) (int32, string, error) {
	parts := strings.SplitN(cursor, ":", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid cursor format")
	}
	value, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return 0, "", err
	}
	return int32(value), parts[1], nil
}
