package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type NorrisServer struct {
	Port        int
	Host        string
	connections []*Connection
	norrisMd    *NorrisMd
}

type Connection struct {
	ws   *websocket.Conn
	send chan []byte
}

func newNorrisServer(port int, hostname string, norrisMd *NorrisMd) *NorrisServer {
	return &NorrisServer{Port: port, Host: hostname, connections: make([]*Connection, 0), norrisMd: norrisMd}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (c *Connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteMessage(mt, payload)
}

func (ns *NorrisServer) writePump(c *Connection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		log.Printf("Stopping writePump for WS connection %v", c)
		ticker.Stop()
		c.ws.Close()
		ns.removeConn(c)
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				log.Println("Error writing ping: %v", err)
				return
			}
		}
	}
}

func (ns *NorrisServer) removeConn(c *Connection) {
	found := -1
	for idx, conn := range ns.connections {
		if conn == c {
			found = idx
			break
		}
	}
	if found > -1 {
		ns.connections = append(ns.connections[:found], ns.connections[found+1:]...)
		log.Printf("Removed connection. Now %v active websocket connections.", len(ns.connections))
	}
}

func (ns *NorrisServer) serveTree(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	tree, err := ns.norrisMd.readTree()

	if err != nil {
		msg := fmt.Sprintf("Error reading NorrisMd document tree: %v", err)
		log.Printf(msg)
		http.Error(w, msg, 500)
		return
	}

	json, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		msg := fmt.Sprintf("Error serializing NorrisMd document tree: %v", err)
		log.Printf(msg)
		http.Error(w, msg, 500)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(json)

	return
}

func (ns *NorrisServer) serveStatic(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	if r.URL.Path == "/" {
		r.URL.Path = "/index.html"
	} else if strings.HasPrefix(r.URL.Path, "/norris_md/static") {
		r.URL.Path = r.URL.Path[len("/norris_md/static/"):]
	}

	requestedAbsPath, _ := filepath.Abs(filepath.Join(ns.norrisMd.StaticPath, r.URL.Path))
	ns.serveFile(requestedAbsPath, w, r)
}

func (ns *NorrisServer) serveFile(absPath string, w http.ResponseWriter, r *http.Request) {

	log.Printf("serveFile %v", absPath)

	if !fileExists(absPath) {
		http.Error(w, fmt.Sprintf("Requested file %v does not exist!", absPath), 404)
		return
	}

	file, err := ioutil.ReadFile(absPath)
	if err != nil {
		msg := fmt.Sprintf("Error reading template file (%v): %v", absPath, err)
		log.Println(msg)
		http.Error(w, msg, 500)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(absPath))
	if "" != contentType {
		w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	}
	w.Write(file)
}

func (ns *NorrisServer) serveContent(w http.ResponseWriter, r *http.Request) {

	if "/" == r.URL.Path || "/index.html" == r.URL.Path {
		ns.serveStatic(w, r)
		return
	}

	log.Printf("serveContent %v", r.URL.Path)

	contentPath := r.URL.Path[len(PATH_CONTENT):len(r.URL.Path)]
	log.Printf("serveContent.contentPath %v", contentPath)

	if !ns.norrisMd.contentExists(contentPath) {

		msg := fmt.Sprintf("Requested file %v does not exist!", contentPath)
		log.Println(msg)
		http.Error(w, msg, 404)
		return
	}

	if strings.HasSuffix(contentPath, ".md") {

		log.Printf("Serving markdown content: %v", contentPath)

		content, err := ns.norrisMd.render(contentPath)
		if err != nil {
			msg := fmt.Sprintf("Error rendering markdown content for %v: %v", contentPath, err)
			log.Println(msg)
			http.Error(w, msg, 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)

	} else {

		absPath, _ := filepath.Abs(filepath.Join(ns.norrisMd.RootPath, r.URL.Path))
		ns.serveFile(absPath, w, r)
	}
}

func (ns *NorrisServer) serveWs(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	c := &Connection{send: make(chan []byte, 256), ws: ws}
	ns.connections = append(ns.connections, c)
	log.Printf("now %v active websocket connections", len(ns.connections))
	go ns.writePump(c)
	log.Println("Accepted WebSocket connection")
}

func (ns *NorrisServer) sendUpdate(nu *NorrisUpdate) {
	jsonContent, err := json.MarshalIndent(nu, "", "  ")
	if err != nil {
		log.Printf("Error while serializing JSON for update event %v: %v", nu, err)
	} else {
		log.Printf("Sending update to %v currently connected WebSocket clients: %v", len(ns.connections), string(jsonContent))
		for _, conn := range ns.connections {
			log.Printf("Sending update to %v", conn)

			json, err := json.MarshalIndent(nu, "", "  ")
			if err != nil {
				msg := fmt.Sprintf("Error serializing NorrisMd document tree: %v", err)
				log.Printf(msg)
				return
			}
			conn.send <- json
		}
	}
}

const (
	PATH_CONTENT = "/"
	PATH_TREE    = "/norris_md/tree.json"
	PATH_WS      = "/norris_md/ws"
	PATH_STATIC  = "/norris_md/static/"
)

func (ns *NorrisServer) run() error {

	http.HandleFunc(PATH_CONTENT, ns.serveContent)
	http.HandleFunc(PATH_TREE, ns.serveTree)
	http.HandleFunc(PATH_WS, ns.serveWs)
	http.HandleFunc(PATH_STATIC, ns.serveStatic)

	err := http.ListenAndServe(fmt.Sprintf("%v:%v", ns.Host, ns.Port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
		return err
	}

	return nil
}

func (ns *NorrisServer) shutdown() error {
	// nothing to do
	return nil
}
