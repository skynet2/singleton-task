package singleton_task

import "context"

type Singleton interface {
	Close() error
	StartAsync() error
}

type Fn func(ctx context.Context)
