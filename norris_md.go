package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
)

type NodeInfo struct {
	NodeType string
	Title    string
	Path     string
	Children []*NodeInfo
}

func convert(path string, fileInfo os.FileInfo) NodeInfo {
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

func initTree(rootPath string) (*NodeInfo, error) {

	// the map will contain NodeInfo objects for all visited nodes in the content tree
	all := map[string]*NodeInfo{}

	// walk all nodes in the content tree
	err := filepath.Walk(rootPath, func(filePath string, fileInfo os.FileInfo, err error) error {

		rootPathAbs, err := filepath.Abs(rootPath)
		filePathAbs, err := filepath.Abs(filePath)
		filePathRel, err := filepath.Rel(rootPathAbs, filePathAbs)

		nodeInfo := convert(filePathRel, fileInfo)

		all[filePathRel] = &nodeInfo

		return err
	})

	// build a tree structure out of the nodes in the map
	root := all["."]
	buildTree(root, &all)

	return root, err
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

func buildTree(root *NodeInfo, all *map[string]*NodeInfo) {
	for _, child := range *all {
		if isChild(root, child) {
			root.Children = append(root.Children, child)
			if child.NodeType == "dir" {
				buildTree(child, all)
			}
		}
	}
}

func printTree(root *NodeInfo, indent int) {
	var indentStr string
	for i := 0; i < indent; i++ {
		indentStr += " "
	}
	if len(root.Children) == 0 {
		log.Printf("%v{nodeType=%v,title=%v,path=%v}", indentStr, root.NodeType, root.Title, root.Path)
	} else {
		log.Printf("%v{nodeType=%v,title=%v,path=%v,children=[", indentStr, root.NodeType, root.Title, root.Path)
		for _, child := range root.Children {
			printTree(child, indent+2)
		}
		log.Printf("%v]}", indentStr)
	}
}

func dirExists(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}
	return true
}

func main() {

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	/*
		input := []byte("#Hello, World")
		mdr := MarkdownRenderer{}
		fmt.Println(string(mdr.render(input)))
	*/

	contentDir := "/Users/danbim/Desktop/norris_content/"

	if !dirExists(contentDir) {
		fmt.Printf("no such file or directory: %s", contentDir)
		os.Exit(1)
		return
	}

	treeRoot, err := initTree(contentDir)

	//printTree(treeRoot, 0)

	json, err := json.MarshalIndent(treeRoot, "", "  ")
	if err != nil {
		log.Println(err)
		os.Exit(2)
		return
	}

	log.Println(string(json))

	fsw, err := newFSWatcher(contentDir)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	for {
		log.Println("waiting for events")
		select {
		case sig := <-signals:
			log.Println("caught signal")
			fsw.signals <- sig
			os.Exit(1)
		case evt := <-fsw.events:
			log.Println(evt)
		}
	}
}
