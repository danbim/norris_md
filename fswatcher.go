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
	dir     string
	events  chan FSEvent
	watcher *fsnotify.Watcher
	watches []string
	signals chan os.Signal
}

func (fsw FSWatcher) rel(dir string) (relDir string, err error) {
	rootAbs, err := filepath.Abs(fsw.dir)
	dirAbs, err := filepath.Abs(dir)
	relDir, err = filepath.Rel(rootAbs, dirAbs)
	return
}

func (fsw FSWatcher) isDir(path string) bool {
	file, err := os.Open(filepath.Join(fsw.dir, path))
	if err != nil {
		log.Println(err)
		return false
	}
	fileInfo, err := file.Stat()
	if err != nil {
		log.Println(err)
		return false
	}
	isDir := fileInfo.IsDir()
	file.Close()
	return isDir
}

func (fsw FSWatcher) watchRecursive(dir string) error {

	log.Printf("watching %v", dir)

	err := fsw.watcher.Watch(dir)
	if err != nil {
		log.Println(err)
		os.Exit(4)
	}
	fsw.watches = append(fsw.watches, dir)

	go func() {
		for {

			fileEvent, ok := <-fsw.watcher.Event
			log.Println(fileEvent)

			if !ok {
				fsw.watcher.Close()
				log.Print("closing file system watcher")
				return
			}

			pathAbs, _ := filepath.Abs(fileEvent.Name)
			path, _ := filepath.Rel(fsw.dir, pathAbs)

			if fileEvent.IsCreate() {

				fsw.events <- FSEvent{EventType: CREATED, Path: path}

				if fsw.isDir(path) {
					fsw.watchRecursive(filepath.Join(fsw.dir, path))
				}

			} else if fileEvent.IsDelete() {

				fsw.events <- FSEvent{EventType: DELETED, Path: path}
				fsw.watcher.RemoveWatch(path)

			} else if fileEvent.IsModify() {
				fsw.events <- FSEvent{EventType: UPDATED, Path: path}
			} else if fileEvent.IsRename() {
				log.Println(fileEvent)
			}
		}
	}()

	err = filepath.Walk(dir, func(filePath string, fileInfo os.FileInfo, err error) error {
		if filePath != dir && fileInfo.IsDir() && filePath != ".." && !strings.HasPrefix(filePath, ".") {
			return fsw.watchRecursive(filePath)
		}
		return nil
	})

	return err
}

func (fsw FSWatcher) run() {
	fsw.watchRecursive(fsw.dir)
}

func newFSWatcher(dir string) (fsw FSWatcher, err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println(err)
		os.Exit(3)
	}
	fsw = FSWatcher{
		dir:     dir,
		watcher: watcher,
		watches: make([]string, 0, 0),
		events:  make(chan FSEvent, 1),
		signals: make(chan os.Signal, 1),
	}
	return
}
