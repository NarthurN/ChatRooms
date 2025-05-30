package main

import (
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Room представляет игровую комнату
type Room struct {
	PIN       string
	Host      *websocket.Conn
	Players   map[*websocket.Conn]*Player
	GameState GameState
	mutex     sync.Mutex
}

// Player представляет игрока
type Player struct {
	Name  string
	Score int
}

// GameState представляет состояние игры
type GameState struct {
	IsStarted       bool
	CurrentQuestion int
	Questions       []Question
}

// Question представляет вопрос квиза
type Question struct {
	Text    string
	Options []string
	Answer  int
}

// Server хранит все комнаты
type Server struct {
	Rooms map[string]*Room
	mutex sync.Mutex
}

func NewServer() *Server {
	return &Server{
		Rooms: make(map[string]*Room),
	}
}

// Генерация 4-значного PIN
func generatePIN() string {
	rand.Seed(time.Now().UnixNano())
	return strconv.Itoa(rand.Intn(9000) + 1000)
}

// Создание новой комнаты
func (s *Server) createRoom(host *websocket.Conn) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Генерация уникального PIN
	var pin string
	for {
		pin = generatePIN()
		if _, exists := s.Rooms[pin]; !exists {
			break
		}
	}

	room := &Room{
		PIN:     pin,
		Host:    host,
		Players: make(map[*websocket.Conn]*Player),
		GameState: GameState{
			IsStarted: false,
			Questions: sampleQuestions(),
		},
	}

	s.Rooms[pin] = room
	return pin
}

// Пример вопросов
func sampleQuestions() []Question {
	return []Question{
		{
			Text:    "Какая столица Франции?",
			Options: []string{"Лондон", "Париж", "Берлин", "Мадрид"},
			Answer:  1,
		},
		{
			Text:    "2 + 2?",
			Options: []string{"3", "4", "5", "6"},
			Answer:  1,
		},
	}
}

// Обработчик WebSocket соединения
func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Ошибка WebSocket: %v", err)
		return
	}
	defer conn.Close()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			break
		}

		switch msg["type"] {
		case "create":
			pin := s.createRoom(conn)
			conn.WriteJSON(map[string]interface{}{
				"type": "created",
				"pin":  pin,
			})

		case "join":
			pin := msg["pin"].(string)
			name := msg["name"].(string)

			s.mutex.Lock()
			room, exists := s.Rooms[pin]
			s.mutex.Unlock()

			if !exists {
				conn.WriteJSON(map[string]interface{}{
					"type":    "error",
					"message": "Комната не найдена",
				})
				continue
			}

			room.mutex.Lock()
			room.Players[conn] = &Player{Name: name, Score: 0}
			room.mutex.Unlock()

			// Уведомляем хозяина о новом игроке
			room.Host.WriteJSON(map[string]interface{}{
				"type": "player_joined",
				"name": name,
			})

			conn.WriteJSON(map[string]interface{}{
				"type": "joined",
			})

		case "start":
			pin := msg["pin"].(string)
			room, exists := s.Rooms[pin]
			if !exists {
				conn.WriteJSON(map[string]interface{}{
					"type":    "error",
					"message": "Комната не найдена",
				})
				continue
			}

			room.mutex.Lock()
			room.GameState.IsStarted = true
			room.GameState.CurrentQuestion = 0
			room.mutex.Unlock()

			// Отправляем первый вопрос всем
			s.broadcastQuestion(room)

		case "answer":
			pin := msg["pin"].(string)
			answer := int(msg["answer"].(float64))
			room, exists := s.Rooms[pin]
			if !exists {
				continue
			}

			room.mutex.Lock()
			player, exists := room.Players[conn]
			if exists && room.GameState.CurrentQuestion < len(room.GameState.Questions) {
				currentQuestion := room.GameState.Questions[room.GameState.CurrentQuestion]
				if answer == currentQuestion.Answer {
					player.Score += 10
				}
			}
			room.mutex.Unlock()
		case "next_question":
			pin := msg["pin"].(string)
			//question := msg["question"].(int)
			// Отправляем первый вопрос всем
			s.Rooms[pin].GameState.CurrentQuestion++
			s.broadcastQuestion(s.Rooms[pin])

		default:
			log.Printf("Неизвестный тип сообщения: %v", msg)
		}
	}
}

// Отправка текущего вопроса всем участникам
func (s *Server) broadcastQuestion(room *Room) {
	room.mutex.Lock()
	defer room.mutex.Unlock()

	if room.GameState.CurrentQuestion >= len(room.GameState.Questions) {
		// Игра окончена
		s.endGame(room)
		return
	}

	question := room.GameState.Questions[room.GameState.CurrentQuestion]
	questionData := map[string]interface{}{
		"type":     "question",
		"text":     question.Text,
		"options":  question.Options,
		"question": room.GameState.CurrentQuestion,
		"total":    len(room.GameState.Questions),
	}

	// Отправляем вопрос хозяину
	room.Host.WriteJSON(questionData)

	// Отправляем вопрос игрокам
	for conn := range room.Players {
		conn.WriteJSON(questionData)
	}
}

// Завершение игры
func (s *Server) endGame(room *Room) {
	// Собираем результаты
	results := make([]map[string]interface{}, 0)
	for conn, player := range room.Players {
		results = append(results, map[string]interface{}{
			"name":  player.Name,
			"score": player.Score,
		})
		conn.Close()
	}

	// Отправляем результаты хозяину
	room.Host.WriteJSON(map[string]interface{}{
		"type":    "game_over",
		"results": results,
	})

	// Удаляем комнату
	s.mutex.Lock()
	delete(s.Rooms, room.PIN)
	s.mutex.Unlock()
}

func main() {
	server := NewServer()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.handleConnections(w, r)
	})

	http.Handle("/", http.FileServer(http.Dir("./static")))

	log.Println("Сервер запущен на :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
