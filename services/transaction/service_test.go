package transaction

import (
	"testing"

	"github.com/bwmarrin/snowflake"
	"github.com/stretchr/testify/require"

	"smallbiznis-controlplane/services/testutil"
)

func TestNewService(t *testing.T) {
	db := testutil.NewTestDB(t)
	node, err := snowflake.NewNode(4)
	require.NoError(t, err)

	svc := NewService(Params{DB: db, Node: node})

	require.NotNil(t, svc)
	require.Equal(t, db, svc.db)
	require.Equal(t, node, svc.node)
}
