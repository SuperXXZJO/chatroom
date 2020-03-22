package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"net/http"
)
func main (){
	go Manager.Start()
	chatroom := gin.Default()
	chatroom.GET("/ws",WsHandler)
	chatroom.Run("59.110.23.117:2222")
}

func WsHandler (c *gin.Context){
	//升级
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	client := &Client{id: uuid.NewV4().String(), socket: conn, sendmsg: make(chan []byte)}
	//注册新连接
	Manager.register <- client
	go client.Read()
	go client.Write()

}


//客户端
type Client struct {
	id string
	socket *websocket.Conn
	sendmsg chan []byte
}
//消息格式化
type Message struct {
	Sender string		`json:"sender"`
	Recipient string	`json:"recipient"`
	Content string		`json:"content"`
}

//客户端管理
type ClientManager struct {
	clients map[*Client]bool  //是否在线
	broadcast chan []byte   //接受的消息
	register chan *Client //注册长连接
	unregister chan *Client //注销长连接
}

var Manager =ClientManager{
	clients:    make(map[*Client]bool),
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
}
//发送消息
func (Manager *ClientManager)Send (message []byte){
	for conn := range Manager.clients{
		conn.sendmsg <- message
	}
}

//读消息
func (c *Client)Read () {
	defer func() {
		Manager.unregister <- c
		c.socket.Close()
	}()
	for {
		//读取消息
		_, message, err := c.socket.ReadMessage()
		if err != nil {
			Manager.unregister <- c
			c.socket.Close()
			break
		}
		jsonMessage,_ :=json.Marshal(&Message{
			Sender:   c.id,
			Content:   string(message),
		})
		Manager.broadcast <- jsonMessage
	}
}

func (c *Client)Write(){
	defer func(){
		c.socket.Close()
	}()

	for{
		select {
		//读消息
		case message,ok := <- c.sendmsg:
			//如果没有就写消息
			if !ok {
				c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}



func (Manager *ClientManager)Start(){
	for{
		select {
		//连接
		case conn := <- Manager.register:
			Manager.clients[conn]=true
			jsonMessage,_ := json.Marshal(&Message{Content:"连接成功"})
			Manager.Send(jsonMessage)
		case conn := <- Manager.unregister:
			if _, ok := Manager.clients[conn]; ok {
				close(conn.sendmsg)
				delete(Manager.clients, conn)
				jsonMessage, _ := json.Marshal(&Message{Content: "/A socket has disconnected."})
				Manager.Send(jsonMessage)
			}
		//广播
		case message := <- Manager.broadcast:
			for conn :=range Manager.clients{
				select {
				case conn.sendmsg <- message:
				default:
					close(conn.sendmsg)
					delete(Manager.clients, conn)
				}
			}


		}


	}

}