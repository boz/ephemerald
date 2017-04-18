package lifecycle_test

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/boz/ephemerald/lifecycle"
	"github.com/boz/ephemerald/params"
	"github.com/stretchr/testify/require"
)

func TestParseAction_override(t *testing.T) {

	js := []byte(`{
		"type": "exec",
		"retries": 10,
		"timeout": "5ms",
		"delay": "10s",
		"path": "make"
	}`)

	action, err := lifecycle.ParseAction(js)
	require.NoError(t, err)
	require.NotNil(t, action)

	require.Equal(t, 10, action.Config().Retries)
	require.Equal(t, 5*time.Millisecond, action.Config().Timeout)
	require.Equal(t, 10*time.Second, action.Config().Delay)
}

func TestParseAction_defaults(t *testing.T) {

	js := []byte(`{
		"type": "exec",
		"path": "echo"
	}`)

	action, err := lifecycle.ParseAction(js)
	require.NoError(t, err)
	require.NotNil(t, action)

	require.Equal(t, lifecycle.ActionDefaultRetries, action.Config().Retries)
	require.Equal(t, 5*time.Second, action.Config().Timeout)
	require.Equal(t, lifecycle.ActionDefaultDelay, action.Config().Delay)
}

func TestActionExec(t *testing.T) {
	runActionFromFile(t, "action.exec.json", "exec", params.Params{}, true, "exec")
}

func TestActionHttpPing(t *testing.T) {
	runActionFromFile(t, "action.http.get.json", "http.get", params.Params{}, true, "http.get")
}

func TestActionTCPConnect(t *testing.T) {
	runActionFromFile(t, "action.tcp.connect.json", "tcp.connect", params.Params{Hostname: "google.com", Port: "80"}, true, "tcp.connect")
}

func actionFromFile(t *testing.T, name string) lifecycle.Action {
	path := path.Join("_testdata", name)
	file, err := os.Open(path)
	require.NoError(t, err)

	buf, err := ioutil.ReadAll(file)
	require.NoError(t, err)

	action, err := lifecycle.ParseAction(buf)
	require.NoError(t, err)
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
	log := logrus.New()
	log.Level = logrus.DebugLevel
	return lifecycle.NewEnv(context.Background(), log.WithField("test", t.Name()))
}
