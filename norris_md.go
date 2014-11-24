package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	EXIT_INTERRUPTED               = 1
	EXIT_KILLED                    = 2
	EXIT_STARTUP_FSWATCHER_FAILED  = 101
	EXIT_SHUTDOWN_FSWATCHER_FAILED = 201
	EXIT_SHUTDOWN_HTTP_FAILED      = 202
	VERSION                        = "0.0.1"
)

type NorrisMd struct {
	RootPath   string
	StaticPath string
	Port       int
	Hostname   string
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

type NodeMetaData struct {
	Title string
}

func (n *NorrisMd) metaData(path string, fileInfo os.FileInfo) *NodeMetaData {
	metaPath := path[0:strings.LastIndex(path, ".")] + ".json"
	fileContents, err := ioutil.ReadFile(metaPath)
	if err != nil {
		return nil
	}
	nodeMetaData := &NodeMetaData{}
	if err := json.Unmarshal(fileContents, nodeMetaData); err != nil {
		log.Printf("Error unmarshalling meta data in %v: %v", metaPath, err)
		return nil
	}
	return nodeMetaData
}

func (n *NorrisMd) convert(pathAbs string, pathRel string, fileInfo os.FileInfo) *NodeInfo {
	var nodeType, title string
	if fileInfo.IsDir() {
		nodeType = "dir"
		title = fileInfo.Name()
	} else {
		nodeType = "file"
		metaData := n.metaData(pathAbs, fileInfo)
		if metaData != nil {
			title = metaData.Title
		} else {
			title = fileInfo.Name()[0:strings.LastIndex(fileInfo.Name(), ".")]
		}
	}
	return &NodeInfo{
		NodeType: nodeType,
		Title:    title,
		Path:     pathRel,
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
		dir, file := filepath.Split(filePathAbs)
		mode := fileInfo.Mode()

		log.Printf("Checking path=%v, file=%v", dir, file)

		// only add dirs and .md files to the document tree, all other files are assets
		if rootPathAbs == filePathAbs || mode.IsDir() || strings.HasSuffix(file, ".md") {
			log.Printf("Adding path=%v, file=%v", dir, file)
			all[filePathRel] = n.convert(filePathAbs, filePathRel, fileInfo)
		}

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

func (n *NorrisMd) contentExists(path string) bool {
	return fileExists(filepath.Join(n.RootPath, path))
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func dirExists(dir string) bool {
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		log.Printf("Error retrieving absolute path of %v", dir)
		return false
	}
	if _, err := os.Stat(dirAbs); os.IsNotExist(err) {
		return false
	}
	return true
}

func (n NorrisMd) shutdown(sig os.Signal) {
	log.Println("Shutting down norris_md")
}

func (n NorrisMd) run() {

	if !dirExists(n.RootPath) {
		fmt.Printf("Site root path does not exist: %s", n.RootPath)
		os.Exit(1)
		return
	}
	/*
		rootNode, err := n.readTree()

		jsonContent, err := json.MarshalIndent(rootNode, "", "  ")
		if err != nil {
			log.Println(err)
			os.Exit(2)
			return
		}

		log.Println(string(jsonContent))
	*/
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

	ns := newNorrisServer(n.Port, n.Hostname, &n)
	go ns.run()
	defer func() {
		err := ns.shutdown()
		if err != nil {
			log.Println("error shutting down NorrisMd web server: %v", err)
			os.Exit(EXIT_SHUTDOWN_HTTP_FAILED)
		}
	}()

	log.Printf("NorrisMd up and running at %v:%v", ns.Host, ns.Port)

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
				Type = "UPDATED"
			} else {
				Type = "CREATED"
			}

			log.Printf("%v %v", Type, evt.Path)

			filePathRel := evt.Path
			filePathAbs := filepath.Join(n.RootPath, evt.Path)
			fileInfo, err := os.Stat(filePathAbs)

			if err != nil {
				log.Printf("error while reading file info for file %v", err)
			} else {
				ns.sendUpdate(&NorrisUpdate{
					Type:     Type,
					Path:     evt.Path,
					NodeInfo: *n.convert(filePathAbs, filePathRel, fileInfo),
				})
			}
		}
	}
}

var renderer *MarkdownRenderer = &MarkdownRenderer{}

func (n NorrisMd) render(path string) (html []byte, err error) {
	if "" == path {
		path = "Home.md"
	}
	absPath := filepath.Join(n.RootPath, path)
	log.Printf("Rendering %v", absPath)
	file, err := ioutil.ReadFile(absPath)
	if err != nil {
		log.Println("Error reading file %v: %v", path, err)
		return nil, err
	}
	return renderer.render(file), nil
}

func main() {

	staticPath := flag.String("static", "./static", "Directory to serve static assets from [default: ./static]")
	port := flag.Int("port", 3456, "HTTP port to listen on")
	hostname := flag.String("hostname", "0.0.0.0", "Hostname to bind to (0.0.0.0 binds to all interfaces)")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s DOC_ROOT \n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	rootPath := flag.Arg(0)

	norrisMd := NorrisMd{
		RootPath:   rootPath,
		StaticPath: *staticPath,
		Port:       *port,
		Hostname:   *hostname,
	}

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
