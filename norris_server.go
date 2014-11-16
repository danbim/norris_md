package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	//"html"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
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
	ticker := time.NewTicker(time.Second)
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
			log.Println("ping")
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				log.Println("error writing ping: %v", err)
				return
			}
		}
	}
}

func (ns NorrisServer) serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/norris_md/tree" {
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
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	file, err := ioutil.ReadFile("template.html")
	if err != nil {
		log.Println("error reading template file")
		http.Error(w, "Error reading template file", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(file)
}

func (ns NorrisServer) serveWs(w http.ResponseWriter, r *http.Request) {
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
	go c.writePump()
	log.Println("Accepted WebSocket connection")
}

func (ns NorrisServer) sendUpdate(nu *NorrisUpdate) {
	jsonContent, err := json.MarshalIndent(nu, "", "  ")
	if err != nil {
		log.Printf("error while serializing JSON for update event %v: %v", nu, err)
	} else {
		log.Println("FAKE sending to connected WebSocket clients: %v", string(jsonContent))
	}
}

func (ns NorrisServer) run() error {
	http.HandleFunc("/", ns.serveHome)
	http.HandleFunc("/ws", ns.serveWs)
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
