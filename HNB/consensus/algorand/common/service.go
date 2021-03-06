package common

import (
	"errors"
	"fmt"
	"sync/atomic"
	//"github.com/op/go-logging"
)

var (
	ErrAlreadyStarted = errors.New("already started")
	ErrAlreadyStopped = errors.New("already stopped")
)

const LOGGER_NAME = "tendermint"

// Service defines a service that can be started, stopped, and reset.
type Service interface {
	Start() error
	OnStart() error

	// Stop the service.
	// If it's already stopped, will return an error.
	// OnStop must never error.
	Stop() error
	OnStop()

	// Reset the service.
	// Panics by default - must be overwritten to enable reset.
	Reset() error
	OnReset() error

	// Return true if the service is running
	IsRunning() bool

	// Quit returns a channel, which is closed once service is stopped.
	Quit() <-chan struct{}

	// String representation of the service
	String() string
}

type BaseService struct {
	name    string
	started uint32 // atomic
	stopped uint32 // atomic
	quit    chan struct{}

	// The "subclass" of BaseService
	impl Service
}

// NewBaseService creates a new BaseService.
func NewBaseService(name string, impl Service) *BaseService {
	return &BaseService{
		name: name,
		quit: make(chan struct{}),
		impl: impl,
	}
}
func (bs *BaseService) Start() error {
	if atomic.CompareAndSwapUint32(&bs.started, 0, 1) {
		if atomic.LoadUint32(&bs.stopped) == 1 {
			//bs.Logger.Error(Fmt("Not starting %v -- already stopped", bs.name), "impl", bs.impl)
			//TODO log
			return ErrAlreadyStopped
		}
		err := bs.impl.OnStart()
		if err != nil {
			// revert flag
			atomic.StoreUint32(&bs.started, 0)
			return err
		}
		return nil
	}
	//	bs.Logger.Debug(Fmt("Not starting %v -- already started", bs.name), "impl", bs.impl)
	return ErrAlreadyStarted
}

func (bs *BaseService) OnStart() error { return nil }

func (bs *BaseService) Stop() error {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) {
		//		bs.Logger.Info(Fmt("Stopping %v", bs.name), "impl", bs.impl)
		bs.impl.OnStop()
		close(bs.quit)
		return nil
	}
	//	bs.Logger.Debug(Fmt("Stopping %v (ignoring: already stopped)", bs.name), "impl", bs.impl)
	return ErrAlreadyStopped
}

// OnStop implements Service by doing nothing.
// NOTE: Do not put anything in here,
// that way users don't need to call BaseService.OnStop()
func (bs *BaseService) OnStop() {}

// Reset implements Service by calling OnReset callback (if defined). An error
// will be returned if the service is running.
func (bs *BaseService) Reset() error {
	if !atomic.CompareAndSwapUint32(&bs.stopped, 1, 0) {
		//		bs.Logger.Debug(Fmt("Can't reset %v. Not stopped", bs.name), "impl", bs.impl)
		return fmt.Errorf("can't reset running %s", bs.name)
	}

	// whether or not we've started, we can reset
	atomic.CompareAndSwapUint32(&bs.started, 1, 0)

	bs.quit = make(chan struct{})
	return bs.impl.OnReset()
}

// OnReset implements Service by panicking.
func (bs *BaseService) OnReset() error {
	PanicSanity("The service cannot be reset")
	return nil
}

// IsRunning implements Service by returning true or false depending on the
// service's state.
func (bs *BaseService) IsRunning() bool {
	//	bs.Logger.Debugf("(service) %s state start %d stop %d", bs.impl.String(), atomic.LoadUint32(&bs.started), atomic.LoadUint32(&bs.stopped))
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}

// Wait blocks until the service is stopped.
func (bs *BaseService) Wait() {
	<-bs.quit
}

// String implements Servce by returning a string representation of the service.
func (bs *BaseService) String() string {
	return bs.name
}

// Quit Implements Service by returning a quit channel.
func (bs *BaseService) Quit() <-chan struct{} {
	return bs.quit
}
