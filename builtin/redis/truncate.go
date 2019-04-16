package redis

import "github.com/boz/ephemerald/lifecycle"

func init() {
	lifecycle.MakeActionPlugin("redis.truncate", actionRedisTruncateParse)
}

func actionRedisTruncateParse(buf []byte) (lifecycle.Generator, error) {
	action := &actionRedisExec{
		ActionConfig: lifecycle.DefaultActionConfig(),
		Command:      "FLUSHALL",
	}
	return action, parseRedisExec(action, buf)
}
