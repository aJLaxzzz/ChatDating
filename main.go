package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"

    "github.com/gorilla/websocket"
)

type User struct {
    Conn *websocket.Conn
    Name string
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

var users = make(map[string]*User) // Хранит активных пользователей
var registeredUsers = make(map[string]string) // Хранит зарегистрированных пользователей (имя: пароль)
var mu sync.Mutex

type Message struct {
    From string `json:"from"`
    To   string `json:"to"`
    Text string `json:"text"`
}

type Registration struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println("Ошибка подключения:", err)
        return
    }
    defer conn.Close()

    var username string
    err = conn.ReadJSON(&username) // Ожидание имени пользователя
    if err != nil {
        log.Println("Ошибка чтения имени пользователя:", err)
        return
    }

    mu.Lock()
    users[username] = &User{Conn: conn, Name: username}
    mu.Unlock()

    log.Printf("Пользователь %s подключился", username)

    // Отправляем обновленный список пользователей всем клиентам
    updateUsers()

    for {
        var msg Message
        err := conn.ReadJSON(&msg)
        if err != nil {
            log.Println("Ошибка чтения сообщения:", err)
            mu.Lock()
            delete(users, username)
            mu.Unlock()
            // Отправляем обновленный список пользователей всем клиентам
            updateUsers()
            break
        }

        log.Printf("Сообщение от %s к %s: %s", msg.From, msg.To, msg.Text)

        if recipient, ok := users[msg.To]; ok {
            err := recipient.Conn.WriteJSON(msg)
            if err != nil {
                log.Println("Ошибка отправки сообщения:", err)
            } else {
                log.Printf("Сообщение от %s успешно отправлено пользователю %s", msg.From, msg.To)
            }
        } else {
            log.Printf("Пользователь %s не найден", msg.To)
        }
    }
}

func updateUsers() {
    mu.Lock()
    defer mu.Unlock()

    // Формируем список пользователей
    var userList []string
    for _, user := range users {
        userList = append(userList, user.Name)
    }

    // Отправляем список пользователей каждому клиенту
    for _, user := range users {
        err := user.Conn.WriteJSON(userList)
        if err != nil {
            log.Println("Ошибка отправки списка пользователей:", err)
        }
    }
}

func registerUser(username, password string) bool {
    mu.Lock()
    defer mu.Unlock()

    if _, exists := registeredUsers[username]; exists {
        return false // Пользователь уже зарегистрирован
    }

    registeredUsers[username] = password
    return true // Регистрация успешна
}

func handleRegistration(w http.ResponseWriter, r *http.Request) {
    var reg Registration
    err := json.NewDecoder(r.Body).Decode(&reg)
    if err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    if registerUser(reg.Username, reg.Password) {
        w.WriteHeader(http.StatusCreated)
        fmt.Fprintf(w, "Пользователь %s зарегистрирован", reg.Username)
        log.Printf("Пользователь %s зарегистрирован", reg.Username)
    } else {
        http.Error(w, "Пользователь уже существует", http.StatusConflict)
        log.Printf("Ошибка регистрации: Пользователь %s уже существует", reg.Username)
    }
}

func main() {
    http.HandleFunc("/ws", handleConnection)
    http.HandleFunc("/register", handleRegistration)
    http.Handle("/", http.FileServer(http.Dir("./")))

    fmt.Println("Сервер запущен на :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
