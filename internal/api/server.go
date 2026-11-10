package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"database/sql"

	"tatsy/internal/core"
	"tatsy/internal/store"
	"tatsy/web"
)

const sessionCookie = "tatsy_session"

type Server struct {
	db       *sql.DB
	hub      *Hub
	tpl      *template.Template
	mu       sync.Mutex
	sessions map[string]store.User
}

func NewServer(db *sql.DB, hub *Hub) (*Server, error) {
	tpl, err := template.ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{db: db, hub: hub, tpl: tpl, sessions: make(map[string]store.User)}, nil
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	static := http.FileServerFS(web.Static)
	mux.Handle("GET /static/", static)

	mux.HandleFunc("GET /login", s.pageLogin)
	mux.HandleFunc("POST /api/login", s.apiLogin)
	mux.HandleFunc("POST /api/logout", s.apiLogout)

	mux.HandleFunc("GET /display", s.pageDisplay)
	mux.HandleFunc("GET /api/events", s.hub.ServeSSE)
	mux.HandleFunc("GET /api/current-order", s.apiCurrentOrder)
	mux.HandleFunc("GET /api/cashiers", s.apiCashiers)

	mux.Handle("GET /{$}", s.requireCap(core.CapSell, http.HandlerFunc(s.pageSeller)))
	mux.Handle("GET /admin", s.requireCap(core.CapAdminister, http.HandlerFunc(s.pageAdmin)))

	mux.Handle("POST /api/order/items", s.requireCap(core.CapSell, http.HandlerFunc(s.apiAddItem)))
	mux.Handle("POST /api/order/lines/{id}/qty", s.requireCap(core.CapSell, http.HandlerFunc(s.apiLineQty)))
	mux.Handle("POST /api/order/lines/{id}/remove", s.requireCap(core.CapSell, http.HandlerFunc(s.apiLineRemove)))
	mux.Handle("POST /api/order/meta", s.requireCap(core.CapSell, http.HandlerFunc(s.apiOrderMeta)))
	mux.Handle("POST /api/order/send", s.requireCap(core.CapSell, http.HandlerFunc(s.apiSend)))
	mux.Handle("POST /api/order/pay", s.requireCap(core.CapSell, http.HandlerFunc(s.apiPay)))
	mux.Handle("POST /api/order/clear", s.requireCap(core.CapSell, http.HandlerFunc(s.apiClear)))

	return mux
}

// ---- sessions ----

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Server) currentUser(r *http.Request) (store.User, bool) {
	tok := ""
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		tok = c.Value
	} else if q := r.URL.Query().Get("session"); q != "" {
		tok = q
	}
	if tok == "" {
		return store.User{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.sessions[tok]
	return u, ok
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentUser(r); !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireCap enforces role capabilities server-side; the UI hiding a
// button is never the only protection.
func (s *Server) requireCap(cap string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.currentUser(r)
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !core.Allowed(u.Role, cap) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "forbidden: role "+u.Role, http.StatusForbidden)
				return
			}
			// Send each role to the console it is allowed to use.
			if core.Allowed(u.Role, core.CapAdminister) {
				http.Redirect(w, r, "/admin", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/", http.StatusSeeOther)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- pages ----

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template", "name", name, "err", err)
	}
}

func (s *Server) pageLogin(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login.html", nil)
}

func (s *Server) pageSeller(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	cats, err := store.ListCategories(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, err := store.ListItems(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	set, err := store.GetSettings(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "seller.html", map[string]any{
		"User":        user,
		"Categories":  cats,
		"Items":       items,
		"Business":    set.BusinessName,
	})
}

// pageAdmin is the owner console: the only place the audit trail is
// visible, plus workers and settings. Owner-only by capability check.
func (s *Server) pageAdmin(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	set, err := store.GetSettings(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	workers, err := store.ListUsers(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	audit, err := store.ListAudit(r.Context(), s.db, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "admin.html", map[string]any{
		"User":     user,
		"Business": set.BusinessName,
		"Settings": set,
		"Workers":  workers,
		"Audit":    audit,
	})
}

func (s *Server) pageDisplay(w http.ResponseWriter, r *http.Request) {
	set, err := store.GetSettings(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cats, err := store.ListCategories(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, err := store.ListItems(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "display.html", map[string]any{
		"Business":   set.BusinessName,
		"Categories": cats,
		"Items":      items,
	})
}

// ---- api ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, err := store.FindByPIN(r.Context(), s.db, req.PIN)
	if err != nil {
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	if u == nil {
		http.Error(w, "wrong PIN", http.StatusUnauthorized)
		return
	}
	token := newToken()
	s.mu.Lock()
	s.sessions[token] = *u
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 12 * 3600,
	})
	_ = store.Audit(r.Context(), s.db, u.ID, u.Name, "T1", "login", "user", u.ID, map[string]any{"role": u.Role})
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) apiLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		u, ok := s.sessions[c.Value]
		delete(s.sessions, c.Value)
		s.mu.Unlock()
		if ok {
			_ = store.Audit(r.Context(), s.db, u.ID, u.Name, "T1", "logout", "user", u.ID, nil)
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiCurrentOrder(w http.ResponseWriter, r *http.Request) {
	o, err := store.GetLatestOrder(r.Context(), s.db)
	if err == store.ErrNoOpenOrder {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// apiCashiers lists workers that may be mirrored on a customer display:
// cashiers only. Managers and owners never appear on the customer screen.
func (s *Server) apiCashiers(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	seen := make(map[string]store.User)
	for _, u := range s.sessions {
		if core.Allowed(u.Role, core.CapMirror) {
			seen[u.ID] = u
		}
	}
	s.mu.Unlock()
	out := make([]store.User, 0, len(seen))
	for _, u := range seen {
		out = append(out, u)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiAddItem(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	var req struct {
		ItemID string `json:"item_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	o, err := store.AddItem(r.Context(), s.db, req.ItemID, user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiLineQty(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Delta int64 `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	o, err := store.ChangeLineQty(r.Context(), s.db, r.PathValue("id"), req.Delta)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiLineRemove(w http.ResponseWriter, r *http.Request) {
	o, err := store.RemoveLine(r.Context(), s.db, r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiOrderMeta(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string `json:"type"`
		TableNo string `json:"table_no"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	current := s.latestOrderID(r)
	if current == "" {
		http.Error(w, "no order", http.StatusBadRequest)
		return
	}
	o, err := store.SetOrderMeta(r.Context(), s.db, current, req.Type, req.TableNo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiSend(w http.ResponseWriter, r *http.Request) {
	id := s.latestOrderID(r)
	if id == "" {
		http.Error(w, "no order", http.StatusBadRequest)
		return
	}
	o, err := store.SendToDisplay(r.Context(), s.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiPay(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := s.latestOrderID(r)
	if id == "" {
		http.Error(w, "no order", http.StatusBadRequest)
		return
	}
	o, err := store.PayOrder(r.Context(), s.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "order_paid", "order", o.ID,
		map[string]any{"order_no": o.OrderNo, "total_kobo": o.TotalKobo, "method": "cash"})
	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiClear(w http.ResponseWriter, r *http.Request) {
	id := s.latestOrderID(r)
	if id == "" {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	if err := store.ClearOrder(r.Context(), s.db, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.hub.Broadcast("order", nil)
	writeJSON(w, http.StatusOK, nil)
}

// latestOrderID resolves the order being worked on. Phase 1 keeps a
// single current order per terminal; stock and multi-order queues come later.
func (s *Server) latestOrderID(r *http.Request) string {
	o, err := store.GetLatestOrder(r.Context(), s.db)
	if err != nil {
		if !errors.Is(err, store.ErrNoOpenOrder) {
			slog.Error("latest order", "err", err)
		}
		return ""
	}
	if o.Status == "paid" {
		return ""
	}
	return o.ID
}
