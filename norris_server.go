package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"path/filepath"
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

func newNorrisServer(port int, host string, norrisMd *NorrisMd) *NorrisServer {
	return &NorrisServer{Port: port, Host: host, connections: make([]*Connection, 0), norrisMd: norrisMd}
}

var port = flag.String("port", ":3456", "HTTP port to listen on")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (c *Connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.ws.WriteMessage(mt, payload)
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.ws.Close()
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
				log.Println("error writing ping: %v", err)
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
		log.Printf("removed connection. now %v active websocket connections.", len(ns.connections))
	}
}

func (ns NorrisServer) serveMeta(w http.ResponseWriter, r *http.Request) {

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

func (ns NorrisServer) serveStatic(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	if r.URL.Path == "/" {
		r.URL.Path = "index.html"
	}

	requestedAbsPath, _ := filepath.Abs(filepath.Join(ns.norrisMd.StaticPath, r.URL.Path))
	if !fileExists(requestedAbsPath) {
		http.Error(w, fmt.Sprintf("Requested file %v does not exist!", requestedAbsPath), 404)
		return
	}

	file, err := ioutil.ReadFile(requestedAbsPath)
	if err != nil {
		msg := fmt.Sprintf("Error reading template file (%v): %v", requestedAbsPath, err)
		log.Println(msg)
		http.Error(w, msg, 500)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(requestedAbsPath))
	if "" != contentType {
		w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	}
	w.Write(file)
}

func (ns NorrisServer) serveContent(w http.ResponseWriter, r *http.Request) {

	contentPath := r.URL.Path[len(PATH_CONTENT):len(r.URL.Path)]
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
	go c.writePump()
	log.Println("Accepted WebSocket connection")
}

func (ns NorrisServer) sendUpdate(nu *NorrisUpdate) {
	jsonContent, err := json.MarshalIndent(nu, "", "  ")
	if err != nil {
		log.Printf("error while serializing JSON for update event %v: %v", nu, err)
	} else {
		log.Println("FAKE sending to connected WebSocket clients: %v", string(jsonContent))
		log.Printf("there are currently %v open connections", len(ns.connections))
		for _, conn := range ns.connections {
			log.Printf("sending update to %v", conn)
			conn.ws.WriteJSON(nu)
		}
	}
}

const (
	PATH_CONTENT = "/norris_md/content/"
	PATH_TREE    = "/norris_md/tree.json"
	PATH_WS      = "/norris_md/ws"
)

func (ns NorrisServer) run() error {

	http.HandleFunc("/norris_md/content/", ns.serveContent)
	http.HandleFunc("/norris_md/tree.json", ns.serveMeta)
	http.HandleFunc("/norris_md/ws", ns.serveWs)
	http.HandleFunc("/", ns.serveStatic)

	err := http.ListenAndServe(*port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
		return err
	}

	return nil
}

func (ns NorrisServer) shutdown() error {
	// nothing to do
	return nil
}
