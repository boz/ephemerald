package lifecycle_test

import (
	"testing"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/testutil"
	"github.com/stretchr/testify/require"
)

// func TestParseManager_full(t *testing.T) {
// 	ms := map[string]lifecycle.Manager{
// 		"json": managerFromFile(t, "manager.full.json"),
// 		"yaml": managerFromFile(t, "manager.full.yaml"),
// 	}

// 	// for ext, m := range ms {
// 	// 	cm := m.ForContainer(testutil.ContainerEmitter(), testutil.CID())

// 	// 	assert.True(t, cm.HasInitialize(), ext)
// 	// 	assert.True(t, cm.HasHealthcheck(), ext)
// 	// 	assert.True(t, cm.HasReset(), ext)
// 	// }
// }

// func TestParseManager_partial(t *testing.T) {
// 	ms := map[string]lifecycle.Manager{
// 		"json": managerFromFile(t, "manager.partial.json"),
// 		"yaml": managerFromFile(t, "manager.partial.yaml"),
// 	}

// 	// for ext, m := range ms {
// 	// 	cm := m.ForContainer(testutil.ContainerEmitter(), testutil.CID())
// 	// 	assert.True(t, cm.HasInitialize(), ext)
// 	// 	assert.False(t, cm.HasHealthcheck(), ext)
// 	// 	assert.False(t, cm.HasReset(), ext)
// 	// }
// }

func managerFromFile(t *testing.T, fpath string) lifecycle.Manager {
	buf := testutil.ReadJSON(t, fpath)
	log := testutil.Log()

	m := lifecycle.NewManager(log)
	require.NoError(t, m.ParseConfig(buf))
	return m
}
