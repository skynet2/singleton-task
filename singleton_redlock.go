package singleton_task

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v9"
)

type singletonRedLock struct {
	locker         *redislock.Client
	fn             Fn
	key            string
	isClosed       bool
	ttl            time.Duration
	ttlExtendEvery time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	logger         zerolog.Logger
}

func NewSingletonRedLock(
	locker *redislock.Client,
	key string,
	fn Fn,
	ctx context.Context,
	ttl time.Duration,
) Singleton {
	lock := &singletonRedLock{
		locker: locker,
		key:    key,
		fn:     fn,
		ttl:    ttl,
	}

	ctx, cancel := context.WithCancel(ctx)

	lock.ttlExtendEvery = lock.ttl / 3
	lock.ctx = ctx
	lock.cancel = cancel

	host, _ := os.Hostname()
	lock.logger = log.Logger.With().
		Str("host", host).
		Str("key", lock.key).
		Int64("id", time.Now().UnixMicro()).
		Logger()

	go func() {
		<-ctx.Done()

		_ = lock.Close()
	}()

	return lock
}

func (s *singletonRedLock) StartAsync() error {
	go func() {
		for s.ctx.Err() == nil {
			if s.isClosed {
				return
			}

			lock, err := s.locker.Obtain(s.ctx, s.key, s.ttl, nil)

			if errors.Is(err, redis.ErrClosed) {
				_ = s.Close()
				return
			}

			if errors.Is(err, redislock.ErrNotObtained) {
				continue
			}

			if err != nil {
				s.logger.Err(errors.Wrap(err, "unexpected error from redislock")).Send()
				continue
			}

			s.logger.Info().Msg("i am leader of the lock")

			ctx, cancel := context.WithCancel(s.ctx)

			go func() {
				s.fn(ctx)
			}()

			ch := make(chan error)

			go func() {
				defer close(ch)
				defer s.recover(ch)

				for {
					time.Sleep(s.ttlExtendEvery)

					if err = lock.Refresh(ctx, s.ttl, nil); err != nil {
						s.logger.Err(err).Msg("lock can not be extended. canceling context")
						ch <- err
						break
					}

					s.logger.Trace().Msg("lock extended")
				}
			}()

			<-ch
			cancel()
		}
	}()

	return nil
}

func (s *singletonRedLock) Close() error {
	if s.isClosed {
		return nil
	}

	s.isClosed = true
	s.cancel()

	return nil
}

func (s *singletonRedLock) recover(ch chan error) {
	if r := recover(); r != nil {
		var recoverErr error
		switch reason := r.(type) {
		case error:
			recoverErr = errors.Wrap(reason, "got panic")
		default:
			recoverErr = errors.New(fmt.Sprintf("got panic for. %+v", reason))
		}

		ch <- recoverErr
	}
}
