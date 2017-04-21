package lifecycle_test

import (
	"io/ioutil"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManager_full(t *testing.T) {

	buf, err := ioutil.ReadFile("_testdata/manager.full.json")
	require.NoError(t, err)

	log := logrus.New()
	m := lifecycle.NewManager(log)

	require.NoError(t, m.ParseConfig(buf))

	cm := m.ForContainer(testutil.ContainerEmitter(), testutil.CID())

	assert.True(t, cm.HasInitialize())
	assert.True(t, cm.HasHealthcheck())
	assert.True(t, cm.HasReset())
}

func TestParseManager_partial(t *testing.T) {
	buf, err := ioutil.ReadFile("_testdata/manager.partial.json")
	require.NoError(t, err)

	log := logrus.New()

	m := lifecycle.NewManager(log)

	require.NoError(t, m.ParseConfig(buf))

	cm := m.ForContainer(testutil.ContainerEmitter(), testutil.CID())

	assert.True(t, cm.HasInitialize())
	assert.False(t, cm.HasHealthcheck())
	assert.False(t, cm.HasReset())
}
