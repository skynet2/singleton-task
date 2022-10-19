package singleton_task

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestMultipleInstances(t *testing.T) {
	locker := redislock.New(getRedisClient())
	lockKey := fmt.Sprint(time.Now().Nanosecond())

	instance1Executed := false
	instance1Stopped := false

	instance2Executed := false
	instance2Stopped := true

	instance1 := NewSingletonRedLock(locker, lockKey, func(ctx context.Context) {
		assert.Equal(t, true, instance2Stopped)
		instance1Stopped = false
		for ctx.Err() == nil {
			instance1Executed = true
			time.Sleep(1 * time.Second)
		}
		instance1Stopped = true
	}, context.Background(), 3*time.Second)

	instance2 := NewSingletonRedLock(locker, lockKey, func(ctx context.Context) {
		instance2Stopped = false
		assert.Equal(t, true, instance1Stopped)
		for ctx.Err() == nil {
			instance2Executed = true
			time.Sleep(1 * time.Second)
		}
		instance2Stopped = true
	}, context.Background(), 3*time.Second)

	assert.NoError(t, instance1.StartAsync())
	time.Sleep(1 * time.Second)
	assert.NoError(t, instance2.StartAsync())

	time.Sleep(2 * time.Second)
	assert.NoError(t, instance1.Close())
	time.Sleep(10 * time.Second)

	assert.Equal(t, true, instance1Executed)
	assert.Equal(t, true, instance1Stopped)
	assert.Equal(t, true, instance1.(*singletonRedLock).isClosed)

	assert.Equal(t, true, instance2Executed)
	assert.Equal(t, false, instance2Stopped)
}

func TestSwitchAfterPanic(t *testing.T) {
	redisClient1 := getRedisClient()
	locker1 := redislock.New(redisClient1)
	locker2 := redislock.New(getRedisClient())
	lockKey := fmt.Sprint(time.Now().Nanosecond())

	instance1Executed := false
	instance1Stopped := false

	instance2Executed := false
	instance2Stopped := true

	instance1 := NewSingletonRedLock(locker1, lockKey, func(ctx context.Context) {
		assert.Equal(t, true, instance2Stopped)
		instance1Stopped = false
		for ctx.Err() == nil {
			instance1Executed = true
			time.Sleep(1 * time.Second)
		}
		instance1Stopped = true
	}, context.Background(), 3*time.Second).(*singletonRedLock)

	time.Sleep(1 * time.Second)
	instance2 := NewSingletonRedLock(locker2, lockKey, func(ctx context.Context) {
		instance2Stopped = false
		assert.Equal(t, true, instance1Stopped)
		for ctx.Err() == nil {
			instance2Executed = true
			time.Sleep(1 * time.Second)
		}
		instance2Stopped = true
	}, context.Background(), 3*time.Second)

	assert.NoError(t, instance1.StartAsync())
	time.Sleep(2 * time.Second)
	assert.NoError(t, instance2.StartAsync())

	time.Sleep(2 * time.Second)
	assert.NoError(t, redisClient1.Close())
	time.Sleep(10 * time.Second)

	assert.Equal(t, true, instance1Executed)
	assert.Equal(t, true, instance1Stopped)

	assert.Equal(t, true, instance2Executed)
	assert.Equal(t, false, instance2Stopped)
}

func getRedisClient() *redis.Client {
	host := "127.0.0.1:6379"

	if redHost := os.Getenv("REDIS_HOST"); len(redHost) > 0 {
		host = redHost
	}

	return redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    host,
	})
}