package main

import (
	"context"
	"errors"
	"log"
	"reflect"
	"sync/atomic"
	"time"
)

var (
	errInvalidRequest = errors.New("input request has no valid task name")
)

// resolveTask resolves the task name to perform from the parsed request.
func resolveTask(req *request) (string, error) {
	r := reflect.ValueOf(req)
	t := r.Type()
	for i := 0; i < r.NumField(); i++ {
		if !r.Field(i).IsNil() {
			return t.Field(i).Name, nil
		}
	}
	return "", errInvalidRequest
}

// startTask starts a given task.
func startTask(name string, req *request) taskAPI {
	ctx, cancel := context.WithCancel(context.Background())
	tp := &taskState{
		cancel:  cancel,
		done:    &atomic.Int64{},
		events:  make(chan *response, taskEventsBuffer),
		stopped: make(chan any),
	}
	go tp.main(ctx, name, req)
	return tp
}

// task implements taskAPI.
type taskState struct {
	// cancel cancels the context used by this task.
	cancel context.CancelFunc

	// done indicates that this task is done.
	done *atomic.Int64

	// events is the channel where we emit task events.
	events chan *response

	// stopped indicates that the task is done.
	stopped chan any
}

var _ taskAPI = &taskState{}

// waitForNextEvent implements taskAPI.waitForNextEvent.
func (tp *taskState) waitForNextEvent(timeout time.Duration) *response {
	// Implementation note: we don't need to log any of these nil-returning conditions
	// as they are not exceptional, rather they're part of normal usage.
	ctx, cancel := contextForWaitForNextEvent(timeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return nil // timeout while blocking for reading
	case ev := <-tp.events:
		return ev // ordinary chan reading
	case <-tp.stopped:
		select {
		case ev := <-tp.events:
			return ev // still draining the chan
		default:
			tp.done.Add(1) // fully drained so we can flip "done" now
			return nil
		}
	}
}

// contextForWaitForNextEvent returns the suitable context
// for making the waitForNextEvent function time bounded.
func contextForWaitForNextEvent(timeo time.Duration) (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if timeo < 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeo)
}

// isDone implements taskAPI.isDone.
func (tp *taskState) isDone() bool {
	return tp.done.Load() > 0
}

// interrupt implements taskAPI.interrupt.
func (tp *taskState) interrupt() {
	tp.cancel()
}

// free implements taskAPI.free
func (tp *taskState) free() {
	tp.interrupt()
	for !tp.isDone() {
		const blockForever = -1
		_ = tp.waitForNextEvent(blockForever)
	}
}

// main is the main function of the task.
func (tp *taskState) main(ctx context.Context, name string, req *request) {
	defer close(tp.stopped) // synchronize with caller
	var resp *response
	runner := taskRegistry[name]
	if runner == nil {
		log.Printf("OONITaskStart: unknown task name: %s", name)
		return
	}
	emitter := &taskChanEmitter{
		out: tp.events,
	}
	defer emitter.maybeEmitEvent(resp)
	runner.main(ctx, emitter, req, resp)
}
