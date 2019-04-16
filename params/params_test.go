package params_test

// func TestParseConfig(t *testing.T) {

// 	bufs := map[string][]byte{
// 		"json": testutil.ReadJSON(t, "config.params.json"),
// 		"yaml": testutil.ReadJSON(t, "config.params.yaml"),
// 	}

// 	for ext, buf := range bufs {
// 		cfg, err := params.ParseConfig(buf)
// 		require.NoError(t, err, ext)
// 		assert.Equal(t, "postgres", cfg.Username, ext)
// 		assert.Equal(t, "", cfg.Password, ext)
// 		assert.Equal(t, "postgres", cfg.Database, ext)
// 	}
// }
