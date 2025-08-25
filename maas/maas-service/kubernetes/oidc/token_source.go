package oidc

import (
	"fmt"
	"os"
	"path"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

type fileTokenSource struct {
	mu       sync.RWMutex
	logger   logging.Logger
	token    atomic.Pointer[string]
	tokenDir string
}

// if now func nil then time.Now is used
func newFileTokenSource(logger logging.Logger, tokenDir string) (*fileTokenSource, error) {
	ts := &fileTokenSource{
		logger:   logger,
		tokenDir: tokenDir,
	}
	err := ts.refreshToken()
	if err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize file watcher: %w", err)
	}
	err = watcher.Add(ts.tokenDir)
	if err != nil {
		return nil, fmt.Errorf("failed to add path %s to file watcher: %w", err)
	}
	go ts.listenFs(watcher.Events)
	go func(errs chan error) {
		for err := range errs {
			logger.Errorf("error at volume mounted token watcher at path %s: %w", ts.tokenDir, err)
		}
	}(watcher.Errors)
	return ts, nil
}

func (f *fileTokenSource) Token() (string, error) {
	return *f.token.Load(), nil
}

func (f *fileTokenSource) listenFs(events chan fsnotify.Event) {
	for ev := range events {
		if path.Base(ev.Name) == "..data" && ev.Op.Has(fsnotify.Create) {
			f.logger.Info("volume mounted token updated, refreshing token at dir %s", f.tokenDir)
			err := f.refreshToken()
			if err != nil {
				f.logger.Errorf("watching volume token at dir %s: %w", f.tokenDir, err)
			}

		}
	}
}

func (f *fileTokenSource) refreshToken() error {
	freshToken, err := os.ReadFile(f.tokenDir + "/token")
	if err != nil {
		return fmt.Errorf("failed to refresh token at path %s: %w", f.tokenDir+"/token", err)
	}
	freshTokenString := string(freshToken)
	f.token.Store(&freshTokenString)
	return nil
}
