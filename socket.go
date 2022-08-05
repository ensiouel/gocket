package gocket

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"log"
)

var (
	ErrConnectionIsNil = fmt.Errorf("socket creation error: %s", "connection is nil")
	ErrServerIsNil     = fmt.Errorf("socket creation error: %s", "server is nil")
)

type Socket struct {
	id         uuid.UUID
	room       *Room
	conn       *websocket.Conn
	server     *Server
	events     map[string]EventsChain
	sendBuffer chan []byte
}

func NewSocket(conn *websocket.Conn, server *Server) (*Socket, error) {
	if conn == nil {
		return nil, ErrConnectionIsNil
	}

	if server == nil {
		return nil, ErrServerIsNil
	}

	socket := new(Socket)

	socket.id = uuid.New()
	socket.conn = conn
	socket.server = server
	socket.events = make(map[string]EventsChain)
	socket.sendBuffer = make(chan []byte)

	return socket, nil
}

func (s *Socket) On(event string, f ...EventFunc) {
	s.events[event] = f
}

func (s *Socket) read() {
	defer func() {
		s.close()
	}()

	for {
		var event *EventResponse

		err := s.conn.ReadJSON(&event)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println(err)
			}
			break
		}

		s.processEvent(event)
	}
}

func (s *Socket) processEvent(event *EventResponse) {
	if event.IsEmit() {
		eventsChain, ok := s.events[event.Name]
		if !ok {
			return
		}

		ctx := NewContext(context.Background(), s)

		for _, eventFunc := range eventsChain {
			eventFunc(ctx, EventData(event.Data))
		}
	}
}

func (s *Socket) send() {
	defer func() {
		s.close()
	}()

	for {
		message, ok := <-s.sendBuffer
		if !ok {
			_ = s.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		if err := s.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Println(err)
			return
		}
	}
}

func (s *Socket) Join(name string) {
	if s.room != nil && s.room.name == name {
		return
	}
	s.server.joinSocketToRoom(s, name)
}

func (s *Socket) Leave() {
	if s.room != nil {
		return
	}
	s.server.leaveSocket(s)
}

func (s *Socket) Emit(name string, data interface{}) {
	b, err := marshalEventRequest(name, data)
	if err != nil {
		log.Println(err)
		return
	}

	s.sendBuffer <- b
}

func (s *Socket) To(name string) *Emitter {
	room := s.server.GetRoom(name)
	if room == nil {
		return newEmptyEmitter()
	}

	return NewEmitter(EmitTypeBroadcastExceptSender, s, room.sockets)
}

func (s *Socket) close() {
	if s.room != nil {
		s.room.Leave(s)
	}

	_ = s.conn.Close()
}

func (s *Socket) GetID() uuid.UUID {
	return s.id
}

func SocketFromContext(ctx context.Context) (*Socket, error) {
	socket, ok := ctx.Value("socket").(*Socket)
	if !ok {
		return nil, errors.New("context it doesn't matter the socket")
	}

	return socket, nil
}
