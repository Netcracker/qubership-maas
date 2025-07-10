package replicator

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
	"sync"
	"time"
)

type MasterMonitor struct {
	mu sync.RWMutex

	lastError     error
	isMasterAlive bool

	db            *gorm.DB
	log           logging.Logger
	checkInterval time.Duration

	healthMetric prometheus.Gauge
}

func NewMasterMonitor(db *gorm.DB, checkInterval time.Duration) *MasterMonitor {
	healthMetric := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "maas",
		Subsystem:   "health",
		Name:        "is_master_db_alive",
		Help:        "main database availability status: UP=1, DOWN=0",
		ConstLabels: nil,
	})
	if err := prometheus.Register(healthMetric); err != nil {
		log.Errorf("error register database availability metric: %v", err)
	}

	return &MasterMonitor{
		mu:            sync.RWMutex{},
		lastError:     nil,
		isMasterAlive: true,
		db:            db,
		log:           logging.GetLogger("master-monitor"),
		checkInterval: checkInterval,
		healthMetric:  healthMetric,
	}
}

func (mm *MasterMonitor) IsAlive() bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	return mm.isMasterAlive
}

func (mm *MasterMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(mm.checkInterval)
	mm.updateStatus(ctx)

	go func() {
		mm.log.InfoC(ctx, "Start master monitor (check interval: %v)", mm.checkInterval)
		defer mm.log.InfoC(ctx, "Master monitor stopped")
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mm.updateStatus(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (mm *MasterMonitor) updateStatus(ctx context.Context) {
	err := mm.db.WithContext(ctx).Exec("select 1").Error

	mm.mu.Lock()
	logState := func() {} // intentionally move logging out of sync scope
	newIsAlive := err == nil
	switch {
	case mm.isMasterAlive && !newIsAlive:
		logState = func() { mm.log.ErrorC(ctx, "Master database is unavailable") }
	case !mm.isMasterAlive && newIsAlive:
		logState = func() { mm.log.InfoC(ctx, "Master database is back to life") }
	}
	mm.isMasterAlive = newIsAlive
	mm.lastError = err
	mm.mu.Unlock()

	// publish status to prometheus metrics
	mm.healthMetric.Set(float64(utils.Iff(mm.isMasterAlive, 1, 0)))

	logState()
}

func (mm *MasterMonitor) Status() error {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	if mm.isMasterAlive {
		return nil
	} else {
		return mm.lastError
	}
}
