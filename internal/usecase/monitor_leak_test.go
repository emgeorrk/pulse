package usecase

import (
	"context"
	"testing"

	"github.com/emgeorrk/pulse/config"
	"github.com/emgeorrk/pulse/internal/entity"
	"github.com/emgeorrk/pulse/internal/sensors"
	"github.com/emgeorrk/pulse/internal/sensors/mocks"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
)

// TestStartStopsGoroutineOnCancel proves the sampling goroutine started by
// Start terminates when its context is canceled and leaves nothing hanging.
// It can't run in parallel: goleak inspects the process-wide goroutine set, so
// concurrent siblings would produce false positives.
//
//nolint:paralleltest,tparallel // goleak needs an exclusive view of all goroutines.
func TestStartStopsGoroutineOnCancel(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)

	cpu := mocks.NewMockCPUSource(ctrl)
	cpu.EXPECT().Ticks().Return([]entity.CoreTicks{{Idle: 100}}, nil).AnyTimes()

	mem := mocks.NewMockMemSource(ctrl)
	mem.EXPECT().Read().Return(entity.MemStats{Total: 1}, nil).AnyTimes()

	mon := NewMonitor(&sensors.Sources{CPU: cpu, Mem: mem}, config.Load(""))

	ctx, cancel := context.WithCancel(context.Background())
	out := mon.Start(ctx)

	cancel()

	// Draining until the channel closes means the goroutine's deferred
	// close(out) ran — i.e. it returned — so goleak sees no survivor.
	for range out { //nolint:revive // intentional drain to synchronize on goroutine exit
	}
}
