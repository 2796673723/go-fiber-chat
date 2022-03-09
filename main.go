package main

import (
	"container/list"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/websocket/v2"
)

type Client struct {
	socket *websocket.Conn
	send   chan []byte
}

func (client *Client) init() {
	//断开连接后销毁对象及其资源
	defer func() {
		manager.unregister <- client
		close(client.send)
		client.socket.Close()
	}()

	//读取广播信息并发送
	go func() {
		for {
			message, ok := <-client.send
			if !ok {
				break
			}
			client.socket.WriteMessage(websocket.TextMessage, message)
		}
	}()

	//读取用户端信息并发送广播
	for {
		_, message, err := client.socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("read error:", err)
			}
			return // Calls the deferred function, i.e. closes the connection on error
		}
		//放入消息链表
		infoList.pushItem(string(message))
		//广播
		manager.broadcast <- message
	}

}

///客户端管理
type ClientManager struct {
	clients    map[*Client]struct{}
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = struct{}{}
			fmt.Printf("register， remain: %v\n", len(manager.clients))
		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				fmt.Printf("unregister， remain: %v\n", len(manager.clients))
			}
		case message := <-manager.broadcast:
			for conn := range manager.clients {
				conn.send <- message
			}
		}
	}
}

var manager = &ClientManager{
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]struct{}),
}

///历史消息链表
// const InfoListSize = 100
type InfoList struct {
	info         *list.List
	infoListSize int
}

func (infoList *InfoList) pushItem(message string) {
	if infoList.info.Len() >= infoList.infoListSize {
		infoList.info.Remove(infoList.info.Front())
	}
	infoList.info.PushBack(message)
}

func (infoList *InfoList) getItems() []string {
	items := make([]string, infoList.info.Len())
	index := 0
	for i := infoList.info.Front(); i != nil; i = i.Next() {
		items[index] = i.Value.(string)
		index++
	}
	return items
}

func (infoList *InfoList) empty() {
	infoList.info.Init()
}

func (infoList *InfoList) toBytes() []byte {
	bytes, err := json.Marshal(infoList.getItems())
	if err != nil {
		return []byte{}
	}
	return bytes
}

///Websocket 方法
func websocketHandler(c *websocket.Conn) {
	//升级get请求为webSocket协议
	client := &Client{socket: c, send: make(chan []byte)}
	//注册客户端
	manager.register <- client
	// 服务初始化
	client.init()

}

//go:embed public
//go:embed index.html
var static embed.FS

var port = flag.Int("p", 3000, "port")
var infoListSize = flag.Int("s", 100, "info list size")

var infoList *InfoList

func main() {
	flag.Parse()
	infoList = &InfoList{info: list.New(), infoListSize: *infoListSize}
	go manager.start()

	app := fiber.New()
	app.Use(cors.New())
	app.Use("/", filesystem.New(filesystem.Config{Root: http.FS(static)}))
	app.Use("/api/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/api/ws", websocket.New(websocketHandler))
	app.Get("/api/info_list", func(c *fiber.Ctx) error {
		return c.Send(infoList.toBytes())
	})
	app.Get("/api/empty_info", func(c *fiber.Ctx) error {
		infoList.empty()
		return c.SendString("empty the info")
	})
	log.Fatal(app.Listen(fmt.Sprintf("0.0.0.0:%v", *port)))
}
