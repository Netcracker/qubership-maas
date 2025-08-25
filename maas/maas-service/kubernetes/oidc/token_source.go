package oidc

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

type fileTokenSource struct {
	mu       sync.RWMutex
	logger   logging.Logger
	token    string
	tokenDir string
}

func NewFileTokenSource(ctx context.Context, tokenDir string) (*fileTokenSource, error) {
	ts := &fileTokenSource{
		logger:   logging.GetLogger("oidc.fileTokenSource"),
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
	err = watcher.Add(ts.tokenDir + "/")
	if err != nil {
		return nil, fmt.Errorf("failed to add path %s to file watcher: %w", ts.tokenDir, err)
	}

	go ts.listenFs(ctx, watcher.Events, watcher.Errors)

	return ts, nil
}

func (f *fileTokenSource) Token() (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.token, nil
}

func (f *fileTokenSource) listenFs(ctx context.Context, events chan fsnotify.Event, errs chan error) {
	for {
		select {
		case ev := <-events:
			// we look for event "..data file created". kubernetes updates the token by updating the "..data" symlink token file points to.
			if path.Base(ev.Name) == "..data" && ev.Op.Has(fsnotify.Create) {
				f.logger.Infof("volume mounted token updated, refreshing token at dir %s", f.tokenDir)
				f.mu.Lock()
				err := f.refreshToken()
				f.mu.Unlock()
				if err != nil {
					f.logger.Errorf("watching volume token at dir %s: %w", f.tokenDir, err)
				}
			}
		case err := <-errs:
			f.logger.Errorf("error at volume mounted token watcher at path %s: %w", f.tokenDir, err)
		case <-ctx.Done():
			f.logger.Infof("token watcher at %s shutdown", f.tokenDir)
			return
		}
	}
}

func (f *fileTokenSource) refreshToken() error {
	freshToken, err := os.ReadFile(f.tokenDir + "/token")
	if err != nil {
		return fmt.Errorf("failed to refresh token at path %s: %w", f.tokenDir+"/token", err)
	}
	f.token = string(freshToken)
	return nil
}
