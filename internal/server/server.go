package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"cluebot/internal/logger"
	"cluebot/internal/monitor"

	"github.com/gorilla/websocket"
)

type Server struct {
	port         int
	log          *logger.Logger
	clients      map[*websocket.Conn]bool
	mu           sync.Mutex
	upgrader     websocket.Upgrader
	currentStats *Stats
	sessions     map[string]bool
}

type Stats struct {
	CPU       *monitor.CPUResult     `json:"cpu"`
	Memory    *monitor.MemoryResult  `json:"memory"`
	Disk      *monitor.DiskResult    `json:"disk"`
	Restart   *monitor.RestartResult `json:"restart"`
	Services  *monitor.ServiceResult `json:"services,omitempty"`
	Processes *monitor.ProcessResult `json:"processes,omitempty"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Hardcoded credentials — add more entries for multiple users
var validCredentials = []Credentials{
	{Username: "admin", Password: "admin"},
	{Username: "operator", Password: "operator"},
}

func New(port int, log *logger.Logger) *Server {
	return &Server{
		port:     port,
		log:      log,
		clients:  make(map[*websocket.Conn]bool),
		sessions: make(map[string]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (s *Server) Start() error {
	http.HandleFunc("/api/login", s.handleLogin)
	http.HandleFunc("/api/verify", s.handleVerify)
	http.HandleFunc("/ws", s.authMiddleware(s.handleWS))
	http.HandleFunc("/api/stats", s.authMiddleware(s.handleStats))
	http.HandleFunc("/api/incidents", s.authMiddleware(s.handleIncidents))
	http.HandleFunc("/", s.handleIndex)

	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			// Allow WebSocket upgrade without auth header if cookie present
			// Fallback to query param for WebSocket
			token = r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			token = "Bearer " + token
		}

		if len(token) < 8 || token[:7] != "Bearer " {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		sessionToken := token[7:]
		s.mu.Lock()
		valid := s.sessions[sessionToken]
		s.mu.Unlock()

		if !valid {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	valid := false
	for _, c := range validCredentials {
		if c.Username == creds.Username && c.Password == creds.Password {
			valid = true
			break
		}
	}

	if !valid {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid username or password"})
		return
	}

	token := generateToken()
	s.mu.Lock()
	s.sessions[token] = true
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.currentStats)
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	incidents, err := s.log.GetRecentIncidents(20)
	if err != nil {
		json.NewEncoder(w).Encode([]any{})
		return
	}
	json.NewEncoder(w).Encode(incidents)
}

func (s *Server) UpdateStats(cpu *monitor.CPUResult, mem *monitor.MemoryResult, disk *monitor.DiskResult, restart *monitor.RestartResult, services *monitor.ServiceResult, processes *monitor.ProcessResult) {
	s.currentStats = &Stats{
		CPU:       cpu,
		Memory:    mem,
		Disk:      disk,
		Restart:   restart,
		Services:  services,
		Processes: processes,
	}

	data, err := json.Marshal(s.currentStats)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for conn := range s.clients {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
