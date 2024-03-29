package singleton_task

//go:generate mockgen -destination singleton_redlock_mock_test.go -package singleton_task -source=singleton_redlock.go

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/bsm/redislock"
)

type singletonRedLock struct {
	locker         lockObtainer
	fn             Fn
	key            string
	isClosed       bool
	ttl            time.Duration
	ttlExtendEvery time.Duration
	ctx            context.Context
	cancel         context.CancelFunc
	logger         zerolog.Logger
}

type lockObtainer interface {
	Obtain(ctx context.Context, key string, ttl time.Duration, opt *redislock.Options) (*redislock.Lock, error)
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
			func() {
				ctx, cancel := context.WithCancel(s.ctx)
				defer cancel()

				lock, err := s.locker.Obtain(ctx, s.key, s.ttl, nil)

				if errors.Is(err, redis.ErrClosed) {
					_ = s.Close()
					time.Sleep(s.ttlExtendEvery)

					return
				}

				if errors.Is(err, redislock.ErrNotObtained) {
					time.Sleep(s.ttlExtendEvery)

					return
				}

				if err != nil {
					s.logger.Err(errors.Wrap(err, "unexpected error from redislock")).Send()
					time.Sleep(s.ttlExtendEvery)

					return
				}

				defer func() {
					_ = lock.Release(s.ctx)
				}()

				s.logger.Trace().Msg("i am leader of the lock")

				fnCh := make(chan error)

				go func() {
					defer close(fnCh)
					defer s.recover(fnCh)
					fnCh <- s.fn(ctx)
				}()

				ch := make(chan error)
				go func() {
					defer close(ch)
					defer s.recover(ch)

					time.Sleep(s.ttlExtendEvery)

					for ctx.Err() == nil {
						if err = lock.Refresh(ctx, s.ttl, nil); err != nil {
							s.logger.Err(err).Msg("lock can not be extended. canceling context")
							ch <- err
							break
						}

						s.logger.Trace().Msg("lock extended")
						time.Sleep(s.ttlExtendEvery)
					}
				}()

				var finalErr error
				select {
				case finalErr = <-ch:
				case finalErr = <-fnCh:
				}

				if finalErr != nil {
					s.logger.Err(finalErr).Msg("final error")
				}
				cancel()
			}()
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
