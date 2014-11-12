package main

import (
	"golang.org/x/exp/fsnotify"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	CREATED = iota
	UPDATED = iota
	DELETED = iota
)

type FSEvent struct {
	EventType int
	Path      string
}

type FSWatcher struct {
	dir      string
	events   chan FSEvent
	watchers map[string]*fsnotify.Watcher
	signals  chan os.Signal
}

func (fsw FSWatcher) rel(dir string) (relDir string, err error) {
	rootAbs, err := filepath.Abs(fsw.dir)
	dirAbs, err := filepath.Abs(dir)
	relDir, err = filepath.Rel(rootAbs, dirAbs)
	return
}

func (fsw FSWatcher) watchRecursive(dir string) error {

	log.Printf("watching %v", dir)

	watcher, err := fsnotify.NewWatcher()
	fsw.watchers[dir] = watcher
	fsw.watchers[dir].Watch(dir)

	go func() {
		for {
			fileEvent, ok := <-fsw.watchers[dir].Event
			if !ok {
				fsw.watchers[dir].Close()
				log.Print("closing file system watcher on directory %v", fsw.dir)
				return
			}
			pathAbs, _ := filepath.Abs(fileEvent.Name)
			path, _ := filepath.Rel(fsw.dir, pathAbs)
			if fileEvent.IsCreate() {
				fsw.events <- FSEvent{EventType: CREATED, Path: path}
			} else if fileEvent.IsDelete() {
				fsw.events <- FSEvent{EventType: DELETED, Path: path}
			} else if fileEvent.IsModify() {
				fsw.events <- FSEvent{EventType: UPDATED, Path: path}
			} else if fileEvent.IsRename() {
				log.Println(fileEvent)
			}
		}
	}()

	err = filepath.Walk(dir, func(filePath string, fileInfo os.FileInfo, err error) error {
		if filePath != dir && fileInfo.IsDir() && filePath != ".." && !strings.HasPrefix(filePath, ".") {
			return fsw.watchRecursive(filepath.Join(dir, fileInfo.Name()))
		}
		return nil
	})

	return err
}

func (fsw FSWatcher) run() {
	fsw.watchRecursive(fsw.dir)
}

func newFSWatcher(dir string) (fsw FSWatcher, err error) {
	fsw = FSWatcher{
		dir:      dir,
		watchers: map[string]*fsnotify.Watcher{},
		events:   make(chan FSEvent, 1),
		signals:  make(chan os.Signal, 1),
	}
	return
}
