package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	EXIT_INTERRUPTED               = iota
	EXIT_KILLED                    = iota
	EXIT_STARTUP_FSWATCHER_FAILED  = iota
	EXIT_SHUTDOWN_FSWATCHER_FAILED = iota
	EXIT_SHUTDOWN_HTTP_FAILED      = iota
)

type NorrisMd struct {
	RootPath string
}

type NorrisUpdate struct {
	Type     string
	Path     string
	NodeInfo NodeInfo
}

type NodeInfo struct {
	NodeType string
	Title    string
	Path     string
	Children []*NodeInfo
}

func (n NorrisMd) convert(path string, fileInfo os.FileInfo) NodeInfo {
	var nodeType string
	if fileInfo.IsDir() {
		nodeType = "dir"
	} else {
		nodeType = "file"
	}
	return NodeInfo{
		NodeType: nodeType,
		Title:    fileInfo.Name(),
		Path:     path,
		Children: make([]*NodeInfo, 0, 0),
	}
}

func (n *NorrisMd) readTree() (NodeInfo, error) {

	// the map will contain NodeInfo objects for all visited nodes in the content tree
	all := map[string]*NodeInfo{}

	// walk all nodes in the content tree
	err := filepath.Walk(n.RootPath, func(filePath string, fileInfo os.FileInfo, err error) error {

		rootPathAbs, err := filepath.Abs(n.RootPath)
		filePathAbs, err := filepath.Abs(filePath)
		filePathRel, err := filepath.Rel(rootPathAbs, filePathAbs)

		nodeInfo := n.convert(filePathRel, fileInfo)

		all[filePathRel] = &nodeInfo

		return err
	})

	// build a tree structure out of the nodes in the map
	rootNode := *all["."]
	n.buildTree(&rootNode, &all)

	return rootNode, err
}

func isChild(parentNode *NodeInfo, childNode *NodeInfo) bool {
	if parentNode.NodeType == "file" {
		log.Printf("isChild => parentnode %v is file")
		return false
	}
	var childParentPath string
	if childNode.NodeType == "file" {
		childParentPath, _ = filepath.Abs(filepath.Dir(childNode.Path))
	} else {
		childParentPath, _ = filepath.Abs(filepath.Join(childNode.Path, ".."))
	}
	parentPath, _ := filepath.Abs(parentNode.Path)
	return parentPath == childParentPath
}

func (n NorrisMd) buildTree(root *NodeInfo, all *map[string]*NodeInfo) {
	for _, child := range *all {
		if isChild(root, child) {
			root.Children = append(root.Children, child)
			if child.NodeType == "dir" {
				n.buildTree(child, all)
			}
		}
	}
}

func (n NorrisMd) printTree(root *NodeInfo, indent int) {
	var indentStr string
	for i := 0; i < indent; i++ {
		indentStr += " "
	}
	if len(root.Children) == 0 {
		log.Printf("%v{nodeType=%v,title=%v,path=%v}", indentStr, root.NodeType, root.Title, root.Path)
	} else {
		log.Printf("%v{nodeType=%v,title=%v,path=%v,children=[", indentStr, root.NodeType, root.Title, root.Path)
		for _, child := range root.Children {
			n.printTree(child, indent+2)
		}
		log.Printf("%v]}", indentStr)
	}
}

func (n NorrisMd) dirExists(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}
	return true
}

func (n NorrisMd) shutdown(sig os.Signal) {
	log.Println("Shutting down norris_md")
}

func (n NorrisMd) run() {

	if !n.dirExists(n.RootPath) {
		fmt.Printf("Site root path does not exist: %s", n.RootPath)
		os.Exit(1)
		return
	}

	rootNode, err := n.readTree()

	jsonContent, err := json.MarshalIndent(rootNode, "", "  ")
	if err != nil {
		log.Println(err)
		os.Exit(2)
		return
	}

	log.Println(string(jsonContent))

	fsWatcher, err := newFSWatcher(n.RootPath)
	if err != nil {
		log.Println(err)
		os.Exit(EXIT_STARTUP_FSWATCHER_FAILED)
	}

	fsWatcher.run()
	defer func() {
		err := fsWatcher.shutdown()
		if err != nil {
			log.Println("error shutting down NorrisMd file system watcher: %v", err)
			os.Exit(EXIT_SHUTDOWN_FSWATCHER_FAILED)
		}
	}()

	ns := newNorrisServer(3456, "localhost")
	go ns.run()
	defer func() {
		err := ns.shutdown()
		if err != nil {
			log.Println("error shutting down NorrisMd web server: %v", err)
			os.Exit(EXIT_SHUTDOWN_HTTP_FAILED)
		}
	}()

	log.Printf("Up and running at %v:%v", ns.Host, ns.Port)

	for {
		evt := <-fsWatcher.events
		switch {
		case evt.EventType == DELETED:
			log.Printf("deleted %v", evt.Path)
			ns.sendUpdate(&NorrisUpdate{
				Type: "DELETED",
				Path: evt.Path,
			})
		case evt.EventType == UPDATED || evt.EventType == CREATED:
			var Type string
			if evt.EventType == UPDATED {
				Type = "CREATED"
			} else {
				Type = "UPDATED"
			}
			log.Printf("%v %v", Type, evt.Path)
			fileInfo, err := os.Stat(filepath.Join(n.RootPath, evt.Path))
			if err != nil {
				log.Printf("error while reading file info for file %v", err)
			} else {
				ns.sendUpdate(&NorrisUpdate{
					Type:     Type,
					Path:     evt.Path,
					NodeInfo: n.convert(evt.Path, fileInfo),
				})
			}
		}
	}
}

var renderer *MarkdownRenderer = &MarkdownRenderer{}

func (n NorrisMd) render(path string) (html []byte, err error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println("error reading file %v: %v", path, err)
		return nil, err
	}
	return renderer.render(file), nil
}

func main() {

	norrisMd := NorrisMd{RootPath: "/Users/danbim/Desktop/norris_content"}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		for sig := range signals {
			norrisMd.shutdown(sig)
			switch {
			case sig == os.Interrupt:
				os.Exit(EXIT_INTERRUPTED)
			case sig == os.Kill || sig == syscall.SIGTERM:
				os.Exit(EXIT_KILLED)
			}
		}
	}()

	norrisMd.run()

}
