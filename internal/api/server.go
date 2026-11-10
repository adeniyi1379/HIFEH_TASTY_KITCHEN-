package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	mux.HandleFunc("GET /static/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/static/")
		diskPath := filepath.Join("web", "static", filepath.Clean("/"+rel))
		if fi, err := os.Stat(diskPath); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, diskPath)
			return
		}
		static.ServeHTTP(w, r)
	})

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
	mux.Handle("GET /api/orders/recent", s.requireCap(core.CapSell, http.HandlerFunc(s.apiRecentOrders)))
	mux.Handle("GET /api/order/{id}", s.auth(http.HandlerFunc(s.apiGetOrder)))
	mux.Handle("POST /api/order/{id}/reprint", s.auth(http.HandlerFunc(s.apiReprintOrder)))
	mux.Handle("POST /api/order/{id}/reopen", s.requireCap(core.CapSell, http.HandlerFunc(s.apiReopenOrder)))
	mux.Handle("POST /api/order/{id}/void", s.requireCap(core.CapSell, http.HandlerFunc(s.apiVoidOrder)))

	// Admin / Manager CRUD & Configurations
	mux.Handle("GET /api/admin/workers", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminListWorkers)))
	mux.Handle("POST /api/admin/workers", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminCreateWorker)))
	mux.Handle("PUT /api/admin/workers/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminUpdateWorker)))
	mux.Handle("POST /api/admin/workers/{id}/toggle", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminToggleWorker)))
	mux.Handle("POST /api/admin/workers/{id}/reset-pin", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminResetWorkerPIN)))
	mux.Handle("DELETE /api/admin/workers/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminDeleteWorker)))

	mux.Handle("GET /api/admin/menu/items", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminListItems)))
	mux.Handle("POST /api/admin/menu/items", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminCreateItem)))
	mux.Handle("PUT /api/admin/menu/items/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminUpdateItem)))
	mux.Handle("POST /api/admin/menu/items/{id}/toggle", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminToggleItem)))
	mux.Handle("DELETE /api/admin/menu/items/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminDeleteItem)))

	mux.Handle("GET /api/admin/menu/categories", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminListCategories)))
	mux.Handle("POST /api/admin/menu/categories", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminCreateCategory)))
	mux.Handle("PUT /api/admin/menu/categories/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminUpdateCategory)))
	mux.Handle("DELETE /api/admin/menu/categories/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminDeleteCategory)))

	mux.Handle("GET /api/admin/settings", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminGetSettings)))
	mux.Handle("PUT /api/admin/settings", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminUpdateSettings)))
	mux.Handle("POST /api/admin/upload/logo", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminUploadLogo)))
	mux.Handle("POST /api/admin/upload/item/{id}", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminUploadItemImage)))

	mux.Handle("GET /api/admin/reports/today", s.requireCap(core.CapAdminister, http.HandlerFunc(s.apiAdminGetTodayReport)))

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
		"IsOwner":     user.Role == "owner",
	})
}

// pageAdmin is the management console: menu items, categories, workers,
// settings, sales reports, and audit trail (owner-only).
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
	cats, err := store.ListCategories(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, err := store.ListAllItems(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report, err := store.GetTodayReport(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var audit []store.AuditEntry
	if user.Role == "owner" {
		audit, _ = store.ListAudit(r.Context(), s.db, 100)
	}
	s.render(w, "admin.html", map[string]any{
		"User":       user,
		"Business":   set.BusinessName,
		"Settings":   set,
		"Workers":    workers,
		"Categories": cats,
		"Items":      items,
		"Report":     report,
		"Audit":      audit,
		"IsOwner":    user.Role == "owner",
		"CanSell":    core.Allowed(user.Role, core.CapSell),
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

func (s *Server) apiRecentOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := store.ListRecentOrders(r.Context(), s.db, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (s *Server) apiGetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	o, err := store.GetOrderByID(r.Context(), s.db, id)
	if err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiReprintOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	o, err := store.GetOrderByID(r.Context(), s.db, id)
	if err != nil {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "receipt_reprinted", "order", o.ID,
		map[string]any{"order_no": o.OrderNo, "reprinted_by": user.Name, "total_kobo": o.TotalKobo})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "order": o})
}

func (s *Server) apiReopenOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		ManagerPIN string `json:"manager_pin"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mgr, err := store.FindByPIN(r.Context(), s.db, req.ManagerPIN)
	if err != nil || mgr == nil {
		http.Error(w, "invalid PIN", http.StatusUnauthorized)
		return
	}
	if mgr.Role != "manager" && mgr.Role != "owner" {
		http.Error(w, "manager or owner authorization required", http.StatusForbidden)
		return
	}

	o, err := store.ReopenOrder(r.Context(), s.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "Recent order correction"
	}

	_ = store.Audit(r.Context(), s.db, mgr.ID, mgr.Name, "T1", "order_reopened", "order", o.ID,
		map[string]any{
			"order_no":     o.OrderNo,
			"manager_name": mgr.Name,
			"cashier_name": user.Name,
			"prev_total":   o.TotalKobo,
			"reason":       reason,
		})

	s.hub.Broadcast("order", o)
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) apiVoidOrder(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		ManagerPIN string `json:"manager_pin"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	mgr, err := store.FindByPIN(r.Context(), s.db, req.ManagerPIN)
	if err != nil || mgr == nil {
		http.Error(w, "invalid PIN", http.StatusUnauthorized)
		return
	}
	if mgr.Role != "manager" && mgr.Role != "owner" {
		http.Error(w, "manager or owner authorization required", http.StatusForbidden)
		return
	}

	o, err := store.VoidOrder(r.Context(), s.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "Order voided"
	}

	_ = store.Audit(r.Context(), s.db, mgr.ID, mgr.Name, "T1", "order_voided", "order", o.ID,
		map[string]any{
			"order_no":     o.OrderNo,
			"manager_name": mgr.Name,
			"cashier_name": user.Name,
			"total_kobo":   o.TotalKobo,
			"reason":       reason,
		})

	s.hub.Broadcast("order", nil)
	writeJSON(w, http.StatusOK, o)
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

// ---- Admin & Manager API Handlers ----

func (s *Server) apiAdminListWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := store.ListUsers(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, workers)
}

func (s *Server) apiAdminCreateWorker(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
		PIN  string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	wkr, err := store.CreateUser(r.Context(), s.db, req.Name, req.Role, req.PIN)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "worker_created", "user", wkr.ID,
		map[string]any{"name": wkr.Name, "role": wkr.Role})
	writeJSON(w, http.StatusCreated, wkr)
}

func (s *Server) apiAdminUpdateWorker(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := store.UpdateUser(r.Context(), s.db, id, req.Name, req.Role); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "worker_updated", "user", id,
		map[string]any{"name": req.Name, "role": req.Role})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminToggleWorker(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := store.SetUserActive(r.Context(), s.db, id, req.Active); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	act := "worker_deactivated"
	if req.Active {
		act = "worker_activated"
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", act, "user", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminResetWorkerPIN(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		NewPIN string `json:"new_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := store.ResetPIN(r.Context(), s.db, id, req.NewPIN); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "worker_pin_reset", "user", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminDeleteWorker(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	if id == user.ID {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}
	if err := store.DeleteUser(r.Context(), s.db, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "worker_deleted", "user", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminListItems(w http.ResponseWriter, r *http.Request) {
	items, err := store.ListAllItems(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) apiAdminCreateItem(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	var req struct {
		CategoryID string `json:"category_id"`
		Name       string `json:"name"`
		UnitType   string `json:"unit_type"`
		UnitLabel  string `json:"unit_label"`
		PriceKobo  int64  `json:"price_kobo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	it, err := store.CreateItem(r.Context(), s.db, req.CategoryID, req.Name, req.UnitType, req.UnitLabel, req.PriceKobo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "item_created", "item", it.ID,
		map[string]any{"name": it.Name, "price_kobo": it.PricePerUnitKobo})
	writeJSON(w, http.StatusCreated, it)
}

func (s *Server) apiAdminUpdateItem(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		CategoryID string `json:"category_id"`
		Name       string `json:"name"`
		UnitType   string `json:"unit_type"`
		UnitLabel  string `json:"unit_label"`
		PriceKobo  int64  `json:"price_kobo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	it, err := store.UpdateItem(r.Context(), s.db, id, req.CategoryID, req.Name, req.UnitType, req.UnitLabel, req.PriceKobo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "item_updated", "item", id,
		map[string]any{"name": it.Name, "price_kobo": it.PricePerUnitKobo})
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) apiAdminToggleItem(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := store.SetItemActive(r.Context(), s.db, id, req.Active); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	act := "item_deactivated"
	if req.Active {
		act = "item_activated"
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", act, "item", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminDeleteItem(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	if err := store.DeleteItem(r.Context(), s.db, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "item_deleted", "item", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := store.ListCategories(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

func (s *Server) apiAdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cat, err := store.CreateCategory(r.Context(), s.db, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "category_created", "category", cat.ID,
		map[string]any{"name": cat.Name})
	writeJSON(w, http.StatusCreated, cat)
}

func (s *Server) apiAdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := store.UpdateCategory(r.Context(), s.db, id, req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "category_updated", "category", id,
		map[string]any{"name": req.Name})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	if err := store.DeleteCategory(r.Context(), s.db, id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "category_deleted", "category", id, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	set, err := store.GetSettings(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, set)
}

func (s *Server) apiAdminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	var req struct {
		BusinessName     string `json:"business_name"`
		ReceiptFooter    string `json:"receipt_footer"`
		VATEnabled       bool   `json:"vat_enabled"`
		VATRateBP        int64  `json:"vat_rate_bp"`
		VATMode          string `json:"vat_mode"`
		PackagingFeeKobo int64  `json:"packaging_fee_kobo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	set, err := store.UpdateSettings(r.Context(), s.db, req.BusinessName, req.ReceiptFooter, req.VATEnabled, req.VATRateBP, req.VATMode, req.PackagingFeeKobo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "settings_updated", "settings", "1",
		map[string]any{"business_name": set.BusinessName, "vat_enabled": set.VATEnabled})
	writeJSON(w, http.StatusOK, set)
}

func (s *Server) apiAdminGetTodayReport(w http.ResponseWriter, r *http.Request) {
	rep, err := store.GetTodayReport(r.Context(), s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func saveImageFromRequest(r *http.Request, targetPath string) error {
	contentType := r.Header.Get("Content-Type")

	// 1. Base64 payload
	if strings.Contains(contentType, "application/json") {
		var req struct {
			Image string `json:"image"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}
		data := req.Image
		if idx := strings.Index(data, ","); idx != -1 {
			data = data[idx+1:]
		}
		raw, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, raw, 0644)
	}

	// 2. Multipart form file
	if err := r.ParseMultipartForm(10 << 20); err == nil {
		file, _, err := r.FormFile("file")
		if err != nil {
			return err
		}
		defer file.Close()
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		out, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, file)
		return err
	}

	return errors.New("unsupported content type")
}

func (s *Server) apiAdminUploadLogo(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	target := filepath.Join("web", "static", "images", "logo.png")
	if err := saveImageFromRequest(r, target); err != nil {
		http.Error(w, "upload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "logo_uploaded", "settings", "1", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": "/static/images/logo.png?v=" + fmt.Sprint(time.Now().Unix())})
}

func (s *Server) apiAdminUploadItemImage(w http.ResponseWriter, r *http.Request) {
	user, _ := s.currentUser(r)
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing item id", http.StatusBadRequest)
		return
	}
	target := filepath.Join("web", "static", "images", id+".png")
	if err := saveImageFromRequest(r, target); err != nil {
		http.Error(w, "upload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	_ = store.Audit(r.Context(), s.db, user.ID, user.Name, "T1", "item_image_uploaded", "item", id, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": "/static/images/" + id + ".png?v=" + fmt.Sprint(time.Now().Unix())})
}
