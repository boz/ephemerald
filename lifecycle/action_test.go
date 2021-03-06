package lifecycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	"github.com/boz/ephemerald/testutil"
	"github.com/stretchr/testify/require"
)

func TestParseAction_override(t *testing.T) {
	actions := map[string]lifecycle.Action{
		"json": actionFromFile(t, "base.override.json"),
		"yaml": actionFromFile(t, "base.override.yaml"),
	}

	for ext, action := range actions {
		require.NotNil(t, action, ext)
		require.Equal(t, 10, action.Config().Retries, ext)
		require.Equal(t, 5*time.Millisecond, action.Config().Timeout, ext)
		require.Equal(t, 10*time.Second, action.Config().Delay, ext)
	}
}

func TestParseAction_defaults(t *testing.T) {
	actions := map[string]lifecycle.Action{
		"json": actionFromFile(t, "base.defaults.json"),
		"yaml": actionFromFile(t, "base.defaults.yaml"),
	}

	for ext, action := range actions {
		require.Equal(t, lifecycle.ActionDefaultRetries, action.Config().Retries, ext)
		require.Equal(t, 5*time.Second, action.Config().Timeout, ext)
		require.Equal(t, lifecycle.ActionDefaultDelay, action.Config().Delay, ext)
	}
}

func TestActionExec(t *testing.T) {
	runActionFromFile(t, "action.exec.json", "exec", params.Params{}, true, "exec")
	runActionFromFile(t, "action.exec.yaml", "exec", params.Params{}, true, "exec")
}

func TestActionHttpPing(t *testing.T) {
	runActionFromFile(t, "action.http.get.json", "http.get", params.Params{}, true, "http.get")
	runActionFromFile(t, "action.http.get.yaml", "http.get", params.Params{}, true, "http.get")
}

func TestActionTCPConnect(t *testing.T) {
	runActionFromFile(t, "action.tcp.connect.json", "tcp.connect", params.Params{Hostname: "google.com", Port: "80"}, true, "tcp.connect")
	runActionFromFile(t, "action.tcp.connect.yaml", "tcp.connect", params.Params{Hostname: "google.com", Port: "80"}, true, "tcp.connect")
}

func actionFromFile(t *testing.T, name string) lifecycle.Action {
	buf := testutil.ReadJSON(t, name)

	action, err := lifecycle.ParseAction(buf)
	require.NoError(t, err, name)
	return action
}

func runActionFromFile(t *testing.T, name string, at string, p params.Params, ok bool, msg string) {
	action := actionFromFile(t, name)
	require.Equal(t, action.Config().Type, at, msg)
	env := actionEnv(t)

	err := action.Do(env, p)

	if ok {
		require.NoError(t, err, msg)
	} else {
		require.NotNil(t, err, msg)
	}
}

func actionEnv(t *testing.T) lifecycle.Env {
	log := testutil.Log()
	return lifecycle.NewEnv(context.Background(), log.WithField("test", t.Name()))
}
