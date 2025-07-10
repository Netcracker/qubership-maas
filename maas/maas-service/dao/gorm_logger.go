package dao

import (
	"context"
	"errors"
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/prometheus/client_golang/prometheus"
	gormlogger "gorm.io/gorm/logger"
	"runtime"
	"strings"
	"time"
)

type GormLogger struct {
	log    logging.Logger
	metric prometheus.Histogram
}

func (gl *GormLogger) LogMode(_ gormlogger.LogLevel) gormlogger.Interface {
	log.Errorf("Changing log level is not supported in Gorm Logger")
	return gl
}

func (gl *GormLogger) Info(ctx context.Context, format string, args ...interface{}) {
	gl.log.InfoC(ctx, format, args...)
}
func (gl *GormLogger) Warn(ctx context.Context, format string, args ...interface{}) {
	gl.log.WarnC(ctx, format, args...)
}
func (gl *GormLogger) Error(ctx context.Context, format string, args ...interface{}) {
	gl.log.ErrorC(ctx, format, args...)
}

func (gl *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	for i := 0; i < 15; i++ {
		_, file, _, ok := runtime.Caller(i)
		// exclude master_monitor's calls from log
		if ok && strings.Contains(file, "master_monitor") {
			return
		}
	}

	rowf := func(c int64) string {
		return utils.Iff(c == -1, "-", fmt.Sprintf("%d", c))
	}

	sql, rows := fc()
	elapsed := time.Since(begin)
	gl.metric.Observe(float64(elapsed) / float64(time.Second))
	switch {
	case err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound):
		log.ErrorC(ctx, "gorm error: %s, [%.3fms] [rows:%v]\n\t%s", err, float64(elapsed.Nanoseconds())/1e6, rowf(rows), sql)
	default:
		log.InfoC(ctx, "gorm: [%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rowf(rows), sql)
	}
}
