package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/urmzd/zigbee-skill/pkg/app"
	"github.com/urmzd/zigbee-skill/pkg/device"
)

// reqCtx returns the request context, enriched with no-cache if the header is set.
func reqCtx(r *http.Request) context.Context {
	ctx := r.Context()
	if r.Header.Get(noCacheHeader) == "true" {
		ctx = device.WithNoCache(ctx)
	}
	return ctx
}

// Server is the daemon HTTP server that keeps the Zigbee connection alive.
type Server struct {
	app        *app.App
	socketPath string
	pidPath    string
}

// NewServer creates a new daemon server.
func NewServer(socketPath, pidPath string) *Server {
	return &Server{
		socketPath: socketPath,
		pidPath:    pidPath,
	}
}

// Start initializes the app, writes PID file, binds Unix socket, and blocks until signal.
func (s *Server) Start(configPath, serialPort string) error {
	a, err := app.New(context.Background(), configPath, serialPort)
	if err != nil {
		return err
	}
	s.app = a

	if err := WritePID(s.pidPath); err != nil {
		a.Close()
		return err
	}

	// Remove stale socket file if it exists.
	_ = os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		a.Close()
		RemovePID(s.pidPath)
		return err
	}

	srv := &http.Server{Handler: s.routes()}

	// Graceful shutdown on SIGTERM/SIGINT.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		log.Info().Msg("Daemon received shutdown signal")
		srv.Close()
	}()

	log.Info().Str("socket", s.socketPath).Msg("Daemon listening")
	err = srv.Serve(ln)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Daemon server error")
	}

	s.cleanup()
	return nil
}

func (s *Server) cleanup() {
	if s.app != nil {
		s.app.Close()
	}
	_ = os.Remove(s.socketPath)
	RemovePID(s.pidPath)
	log.Info().Msg("Daemon stopped")
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /health", s.handleHealth)
	mux.HandleFunc("POST /devices/list", s.handleDevicesList)
	mux.HandleFunc("POST /devices/get", s.handleDevicesGet)
	mux.HandleFunc("POST /devices/rename", s.handleDevicesRename)
	mux.HandleFunc("POST /devices/remove", s.handleDevicesRemove)
	mux.HandleFunc("POST /devices/clear", s.handleDevicesClear)
	mux.HandleFunc("POST /devices/state", s.handleDevicesState)
	mux.HandleFunc("POST /devices/set", s.handleDevicesSet)
	mux.HandleFunc("POST /discovery/permit", s.handleDiscoveryPermit)
	mux.HandleFunc("GET /discovery/events", s.handleDiscoveryEvents)
	return mux
}

// --- request/response types ---

type idRequest struct {
	ID string `json:"id"`
}

type renameRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type removeRequest struct {
	ID    string `json:"id"`
	Force bool   `json:"force"`
}

type setStateRequest struct {
	ID    string         `json:"id"`
	State map[string]any `json:"state"`
}

type permitRequest struct {
	Enable   bool `json:"enable"`
	Duration int  `json:"duration"`
}

// --- handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"connected": s.app.Controller.IsConnected(),
	})
}

func (s *Server) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	devices, err := s.app.Controller.ListDevices(reqCtx(r))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (s *Server) handleDevicesGet(w http.ResponseWriter, r *http.Request) {
	var req idRequest
	if !decodeBody(w, r, &req) {
		return
	}
	d, err := s.app.Controller.GetDevice(reqCtx(r), req.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"device": d})
}

func (s *Server) handleDevicesRename(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.app.Controller.RenameDevice(reqCtx(r), req.ID, req.Name); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleDevicesRemove(w http.ResponseWriter, r *http.Request) {
	var req removeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.app.Controller.RemoveDevice(reqCtx(r), req.ID, req.Force); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleDevicesClear(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Controller.ClearDevices(reqCtx(r)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleDevicesState(w http.ResponseWriter, r *http.Request) {
	var req idRequest
	if !decodeBody(w, r, &req) {
		return
	}
	st, err := s.app.Controller.GetDeviceState(reqCtx(r), req.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": st})
}

func (s *Server) handleDevicesSet(w http.ResponseWriter, r *http.Request) {
	var req setStateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	st, err := s.app.Controller.SetDeviceState(reqCtx(r), req.ID, req.State)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"state": st})
}

func (s *Server) handleDiscoveryPermit(w http.ResponseWriter, r *http.Request) {
	var req permitRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.app.Controller.PermitJoin(reqCtx(r), req.Enable, req.Duration); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// handleDiscoveryEvents streams discovery events as SSE to the client.
// The connection stays open until the client disconnects.
func (s *Server) handleDiscoveryEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.app.Events.Subscribe()
	defer s.app.Events.Unsubscribe(ch)

	for {
		select {
		case ev := <-ch:
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// --- helpers ---

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, device.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, device.ErrNotConnected):
		code = http.StatusServiceUnavailable
	case errors.Is(err, device.ErrValidation):
		code = http.StatusBadRequest
	case errors.Is(err, device.ErrTimeout):
		code = http.StatusGatewayTimeout
	}
	writeJSON(w, code, map[string]any{"error": err.Error()})
}
