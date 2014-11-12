package main

import (
	"golang.org/x/exp/fsnotify"
	"log"
	"os"
	"path/filepath"
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
	dir     string
	events  chan FSEvent
	watcher fsnotify.Watcher
	signals chan os.Signal
}

func (fsw *FSWatcher) run() {
	fsw.watcher.Watch(fsw.dir)
	defer func() {
		fsw.watcher.Close()
		log.Println("Closing file system watcher on directory %v", fsw.dir)
	}()
	for {
		fileEvent := *<-fsw.watcher.Event
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
}

func newFSWatcher(dir string) (fsw FSWatcher, err error) {
	w, err := fsnotify.NewWatcher()
	fsw = FSWatcher{dir: dir, watcher: *w, events: make(chan FSEvent, 10), signals: make(chan os.Signal, 1)}
	return
}
