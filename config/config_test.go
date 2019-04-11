package config_test

// func TestRead(t *testing.T) {
// 	doReadTest(t, "_testdata/config.json", "json")
// 	doReadTest(t, "_testdata/config.yaml", "yaml")
// 	doReadTest(t, "_testdata/config.yml", "yml")
// }

// func doReadTest(t *testing.T, path string, msg string) {
// 	log := testutil.Log()
// 	uie := testutil.Emitter()

// 	configs, err := config.ReadFile(log, uie, "_testdata/config.json")
// 	require.NoError(t, err, msg)

// 	require.Equal(t, 1, len(configs), msg)

// 	cfg := configs[0]

// 	assert.Equal(t, "redis", cfg.Name, msg)
// 	assert.Equal(t, "redis", cfg.Image, msg)
// 	assert.Equal(t, 6379, cfg.Port, msg)
// 	assert.Equal(t, 10, cfg.Size, msg)

// 	m := cfg.Lifecycle.ForContainer(testutil.CID())

// 	assert.False(t, m.HasInitialize(), msg)
// 	assert.True(t, m.HasHealthcheck(), msg)
// 	assert.True(t, m.HasReset(), msg)
// }
