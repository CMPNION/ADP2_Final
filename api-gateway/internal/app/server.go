package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	catpb "github.com/cmpnion/adp-final/proto/catalog"
	invpb "github.com/cmpnion/adp-final/proto/inventory"
	notpb "github.com/cmpnion/adp-final/proto/notification"
	ordpb "github.com/cmpnion/adp-final/proto/order"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_gateway_http_requests_total",
			Help: "Total HTTP requests handled by the API gateway.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_gateway_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	metricsOnce sync.Once
)

func ensureMetricsRegistered() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
	})
}

func (s *Server) requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentRole, _ := r.Context().Value(contextKey("role")).(string)
		if !strings.EqualFold(currentRole, role) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		next.ServeHTTP(w, r)
	}
}

type Server struct {
	cfg     Config
	inv     invpb.InventoryServiceClient
	cat     catpb.CatalogServiceClient
	ord     ordpb.OrderServiceClient
	not     notpb.NotificationServiceClient
	limiter *rate.Limiter
	usersMu sync.RWMutex
	users   map[string]demoUser
}

type demoUser struct {
	Username     string
	PasswordHash []byte
	Role         string
}

func NewServer(cfg Config, invConn, catConn, ordConn, notConn *grpc.ClientConn) *Server {
	ensureMetricsRegistered()
	return &Server{
		cfg:     cfg,
		inv:     invpb.NewInventoryServiceClient(invConn),
		cat:     catpb.NewCatalogServiceClient(catConn),
		ord:     ordpb.NewOrderServiceClient(ordConn),
		not:     notpb.NewNotificationServiceClient(notConn),
		limiter: rate.NewLimiter(rate.Every(time.Minute/time.Duration(cfg.RateLimitPerMinute)), cfg.RateLimitPerMinute),
		users:   make(map[string]demoUser),
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	// auth
	mux.HandleFunc("/auth/register", s.registerHandler)
	mux.HandleFunc("/auth/login", s.loginHandler)
	// core inventory actions
	mux.HandleFunc("/inventory/reserve", s.auth(s.limit(s.reserveStock)))
	mux.HandleFunc("/inventory/release", s.auth(s.limit(s.releaseStock)))
	mux.HandleFunc("/inventory/confirm", s.auth(s.limit(s.confirmStock)))
	// read operations
	mux.HandleFunc("/inventory/sku", s.auth(s.limit(s.getStockBySKU)))                      // GET ?sku=...
	mux.HandleFunc("/inventory/warehouses", s.auth(s.limit(s.listWarehouses)))              // GET
	mux.HandleFunc("/inventory/warehouse/stocks", s.auth(s.limit(s.listStocksByWarehouse))) // GET ?warehouse_id=...
	mux.HandleFunc("/inventory/low", s.auth(s.limit(s.getLowStockItems)))                   // GET ?limit=10
	// admin operations
	mux.HandleFunc("/inventory/receipt", s.auth(s.limit(s.requireRole("admin", s.addStockReceipt))))
	mux.HandleFunc("/inventory/transfer", s.auth(s.limit(s.requireRole("admin", s.transferStock))))
	mux.HandleFunc("/inventory/safety", s.auth(s.limit(s.requireRole("admin", s.updateSafetyStockLevel))))
	mux.HandleFunc("/inventory/warehouse", s.auth(s.limit(s.requireRole("admin", s.createOrUpdateWarehouse)))) // POST create, PUT update
	// catalog operations
	mux.HandleFunc("/catalog/products", s.auth(s.limit(s.catalogHandler)))
	mux.HandleFunc("/catalog/search", s.auth(s.limit(s.catalogSearch)))
	mux.HandleFunc("/catalog/price", s.auth(s.limit(s.catalogUpdatePrice)))
	mux.HandleFunc("/catalog/bulk", s.auth(s.limit(s.catalogBulkGet)))
	// order operations
	mux.HandleFunc("/orders", s.auth(s.limit(s.ordersHandler)))
	mux.HandleFunc("/orders/cancel", s.auth(s.limit(s.ordersCancel)))
	mux.HandleFunc("/orders/status", s.auth(s.limit(s.ordersStatus)))
	mux.HandleFunc("/orders/calculate", s.auth(s.limit(s.ordersCalculate)))
	mux.HandleFunc("/orders/bulk", s.auth(s.limit(s.ordersBulk)))
	// notification operations
	mux.HandleFunc("/notifications/email", s.auth(s.limit(s.notificationEmail)))
	mux.HandleFunc("/notifications/order-confirmation", s.auth(s.limit(s.notificationOrderConfirm)))
	mux.HandleFunc("/notifications/stock-alert", s.auth(s.limit(s.notificationStockAlert)))
	return s.instrument(mux)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Register a demo user (username, password, role)
func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password required"})
		return
	}
	// hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	s.usersMu.Lock()
	defer s.usersMu.Unlock()
	if _, ok := s.users[req.Username]; ok {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "user exists"})
		return
	}
	s.users[req.Username] = demoUser{Username: req.Username, PasswordHash: hash, Role: req.Role}
	writeJSON(w, http.StatusCreated, map[string]string{"username": req.Username})
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if s.cfg.JWTSecret == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth not configured"})
		return
	}
	s.usersMu.RLock()
	user, ok := s.users[req.Username]
	s.usersMu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	claims := jwt.MapClaims{"user_id": user.Username, "role": user.Role, "exp": time.Now().Add(24 * time.Hour).Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to sign token"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": signed})
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type reserveRequest struct {
	OrderID string `json:"order_id"`
	Items   []struct {
		SKU         string `json:"sku"`
		WarehouseID string `json:"warehouse_id"`
		Qty         int64  `json:"qty"`
	} `json:"items"`
}

func (s *Server) reserveStock(w http.ResponseWriter, r *http.Request) {
	var req reserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	orderID := strings.TrimSpace(req.OrderID)
	if orderID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order_id is required"})
		return
	}
	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}
	items := make([]*invpb.ReservationItem, 0, len(req.Items))
	skus := make([]string, 0, len(req.Items))
	seen := make(map[string]struct{}, len(req.Items))
	for _, item := range req.Items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sku is required"})
			return
		}
		if item.Qty <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty must be greater than zero"})
			return
		}
		items = append(items, &invpb.ReservationItem{Sku: sku, WarehouseId: strings.TrimSpace(item.WarehouseID), Qty: item.Qty})
		if _, ok := seen[sku]; !ok {
			seen[sku] = struct{}{}
			skus = append(skus, sku)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.validateCatalogProducts(ctx, skus); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	orderResp, err := s.ord.GetOrder(ctx, &ordpb.GetOrderRequest{OrderId: orderID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if orderResp.GetOrderId() == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}
	if !isMutableOrderStatus(orderResp.GetStatus()) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order is already finalized"})
		return
	}
	resp, err := s.inv.ReserveStock(ctx, &invpb.ReserveStockRequest{OrderId: orderID, Items: items})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type orderRefRequest struct {
	OrderID string `json:"order_id"`
}

func (s *Server) releaseStock(w http.ResponseWriter, r *http.Request) {
	var req orderRefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	orderResp, err := s.ord.GetOrder(ctx, &ordpb.GetOrderRequest{OrderId: req.OrderID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if orderResp.GetOrderId() == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
		return
	}
	if !isMutableOrderStatus(orderResp.GetStatus()) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order is already finalized"})
		return
	}
	resp, err := s.inv.ReleaseStock(ctx, &invpb.ReleaseStockRequest{OrderId: req.OrderID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) confirmStock(w http.ResponseWriter, r *http.Request) {
	var req orderRefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.ConfirmStockDeduction(ctx, &invpb.ConfirmStockDeductionRequest{OrderId: req.OrderID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getStockBySKU(w http.ResponseWriter, r *http.Request) {
	sku := r.URL.Query().Get("sku")
	if sku == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing sku"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.GetStockBySKU(ctx, &invpb.GetStockBySKURequest{Sku: sku})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sku":             resp.GetSku(),
		"total_available": resp.GetTotalAvailable(),
		"total_reserved":  resp.GetTotalReserved(),
	})
}

func (s *Server) listWarehouses(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.ListWarehouses(ctx, &invpb.ListWarehousesRequest{})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) listStocksByWarehouse(w http.ResponseWriter, r *http.Request) {
	wid := r.URL.Query().Get("warehouse_id")
	if wid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing warehouse_id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.ListStocksByWarehouse(ctx, &invpb.ListStocksByWarehouseRequest{WarehouseId: wid})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) getLowStockItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("limit")
	limit := int32(50)
	if q != "" {
		if parsed, err := strconv.ParseInt(q, 10, 32); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.GetLowStockItems(ctx, &invpb.GetLowStockItemsRequest{Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// admin write endpoints
func (s *Server) addStockReceipt(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	itemsRaw, ok := req["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}
	items := make([]map[string]any, 0, len(itemsRaw))
	skus := make([]string, 0, len(itemsRaw))
	seen := make(map[string]struct{}, len(itemsRaw))
	if arr, ok := req["items"].([]any); ok {
		for _, it := range arr {
			m, ok := it.(map[string]any)
			if !ok {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items must be objects"})
				return
			}
			sku, _ := m["sku"].(string)
			wid, _ := m["warehouse_id"].(string)
			qty := int64(0)
			if v, ok := m["qty"].(float64); ok {
				qty = int64(v)
			}
			sku = strings.TrimSpace(sku)
			wid = strings.TrimSpace(wid)
			if sku == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sku is required"})
				return
			}
			if wid == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "warehouse_id is required"})
				return
			}
			if qty <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty must be greater than zero"})
				return
			}
			m["sku"] = sku
			m["warehouse_id"] = wid
			items = append(items, m)
			if _, ok := seen[sku]; !ok {
				seen[sku] = struct{}{}
				skus = append(skus, sku)
			}
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.validateCatalogProducts(ctx, skus); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	outs := make([]map[string]any, 0, len(items))
	for _, m := range items {
		sku, _ := m["sku"].(string)
		wid, _ := m["warehouse_id"].(string)
		qty := int64(0)
		if v, ok := m["qty"].(float64); ok {
			qty = int64(v)
		}
		_, err := s.inv.AddStockReceipt(ctx, &invpb.AddStockReceiptRequest{Sku: sku, WarehouseId: wid, Quantity: qty})
		if err != nil {
			outs = append(outs, map[string]any{"sku": sku, "warehouse_id": wid, "success": false, "error": err.Error()})
		} else {
			outs = append(outs, map[string]any{"sku": sku, "warehouse_id": wid, "success": true})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": outs})
}

func (s *Server) transferStock(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sku           string `json:"sku"`
		FromWarehouse string `json:"from_warehouse"`
		ToWarehouse   string `json:"to_warehouse"`
		Qty           int64  `json:"qty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	sku := strings.TrimSpace(req.Sku)
	if sku == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sku is required"})
		return
	}
	fromWarehouse := strings.TrimSpace(req.FromWarehouse)
	toWarehouse := strings.TrimSpace(req.ToWarehouse)
	if fromWarehouse == "" || toWarehouse == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from_warehouse and to_warehouse are required"})
		return
	}
	if fromWarehouse == toWarehouse {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from_warehouse and to_warehouse must differ"})
		return
	}
	if req.Qty <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty must be greater than zero"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.TransferStock(ctx, &invpb.TransferStockRequest{Sku: sku, FromWarehouseId: fromWarehouse, ToWarehouseId: toWarehouse, Quantity: req.Qty})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) updateSafetyStockLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sku         string `json:"sku"`
		WarehouseID string `json:"warehouse_id"`
		Level       int64  `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.inv.UpdateSafetyStockLevel(ctx, &invpb.UpdateSafetyStockLevelRequest{Sku: req.Sku, WarehouseId: req.WarehouseID, Level: req.Level})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) createOrUpdateWarehouse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Id       string `json:"id,omitempty"`
		Name     string `json:"name"`
		Location string `json:"location"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		resp, err := s.inv.CreateWarehouse(ctx, &invpb.CreateWarehouseRequest{Name: req.Name, Location: req.Location})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	// PUT -> update
	resp, err := s.inv.UpdateWarehouseInfo(ctx, &invpb.UpdateWarehouseInfoRequest{WarehouseId: req.Id, Name: req.Name, Location: req.Location, IsActive: req.IsActive})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If JWT secret is configured, validate and attach user info
		if s.cfg.JWTSecret != "" {
			token, err := extractBearerToken(r.Header.Get("Authorization"))
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenSignatureInvalid
				}
				return []byte(s.cfg.JWTSecret), nil
			}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
			if err != nil || !parsed.Valid {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
				if uid, _ := claims["user_id"].(string); uid != "" {
					r = r.WithContext(context.WithValue(r.Context(), userIDContextKey, uid))
				}
				if role, _ := claims["role"].(string); role != "" {
					r = r.WithContext(context.WithValue(r.Context(), contextKey("role"), role))
				}
			}
		} else if s.cfg.AuthToken != "" {
			token, err := extractBearerToken(r.Header.Get("Authorization"))
			if err != nil || token != s.cfg.AuthToken {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

func (s *Server) limit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (s *Server) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := requestID(r.Header.Get("X-Request-Id"))
		w.Header().Set("X-Request-Id", reqID)
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		path := r.URL.Path
		httpRequestsTotal.WithLabelValues(r.Method, path, statusText(rw.status)).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
		log.Printf(`{"component":"api-gateway","request_id":%q,"method":%q,"path":%q,"status":%d,"duration_ms":%d}`,
			reqID, r.Method, r.URL.Path, rw.status, time.Since(start).Milliseconds())
	})
}

func requestID(existing string) string {
	if existing != "" {
		return existing
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func extractBearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", jwt.ErrTokenMalformed
	}
	return parts[1], nil
}

func statusText(code int) string {
	return http.StatusText(code)
}

func (s *Server) validateCatalogProducts(ctx context.Context, skus []string) error {
	if len(skus) == 0 {
		return nil
	}
	resp, err := s.cat.BulkGetProducts(ctx, &catpb.BulkGetProductsRequest{Sku: skus})
	if err != nil {
		return err
	}
	found := make(map[string]struct{}, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		found[p.GetSku()] = struct{}{}
	}
	missing := make([]string, 0)
	for _, sku := range skus {
		if _, ok := found[sku]; !ok {
			missing = append(missing, sku)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("products not found: %s", strings.Join(missing, ","))
	}
	return nil
}

// Catalog handlers
func (s *Server) catalogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Sku         string  `json:"sku"`
			ProductID   string  `json:"product_id"`
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Price       float64 `json:"price"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		sku := req.Sku
		if sku == "" {
			sku = req.ProductID
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		resp, err := s.cat.CreateProduct(ctx, &catpb.CreateProductRequest{
			Product: &catpb.Product{
				Sku:         sku,
				Name:        req.Name,
				Description: req.Description,
				Price:       req.Price,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if r.Method == http.MethodGet {
		sku := r.URL.Query().Get("sku")
		if sku == "" {
			sku = r.URL.Query().Get("product_id")
		}
		if sku == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing sku param"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		resp, err := s.cat.GetProduct(ctx, &catpb.GetProductRequest{Sku: sku})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
func (s *Server) catalogSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing q param"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.cat.SearchProducts(ctx, &catpb.SearchProductsRequest{Query: query})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
func (s *Server) catalogUpdatePrice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sku       string  `json:"sku"`
		ProductID string  `json:"product_id"`
		Price     float64 `json:"price"`
		NewPrice  float64 `json:"new_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	sku := req.Sku
	if sku == "" {
		sku = req.ProductID
	}
	price := req.Price
	if req.NewPrice != 0 {
		price = req.NewPrice
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.cat.UpdatePrice(ctx, &catpb.UpdatePriceRequest{Sku: sku, Price: price})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
func (s *Server) catalogBulkGet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Skus []string `json:"skus"`
		SKU  []string `json:"sku"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	skus := req.Skus
	if len(skus) == 0 {
		skus = req.SKU
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.cat.BulkGetProducts(ctx, &catpb.BulkGetProductsRequest{Sku: skus})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Order handlers
func (s *Server) ordersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			UserID     string `json:"user_id"`
			CustomerID string `json:"customer_id"`
			Items      []struct {
				SKU       string `json:"sku"`
				ProductID string `json:"product_id"`
				Qty       int64  `json:"qty"`
			} `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		userID := strings.TrimSpace(req.UserID)
		if userID == "" {
			userID = strings.TrimSpace(req.CustomerID)
		}
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
			return
		}
		items := make([]*ordpb.OrderItem, 0, len(req.Items))
		skus := make([]string, 0, len(req.Items))
		seen := make(map[string]struct{}, len(req.Items))
		for _, item := range req.Items {
			sku := strings.TrimSpace(item.SKU)
			if sku == "" {
				sku = strings.TrimSpace(item.ProductID)
			}
			if sku == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sku is required"})
				return
			}
			if item.Qty <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty must be greater than zero"})
				return
			}
			items = append(items, &ordpb.OrderItem{Sku: sku, Qty: item.Qty})
			if _, ok := seen[sku]; !ok {
				seen[sku] = struct{}{}
				skus = append(skus, sku)
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		// items are optional — allow creating empty orders. If items provided, validate them against catalog
		if len(skus) > 0 {
			if err := s.validateCatalogProducts(ctx, skus); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		}
		resp, err := s.ord.CreateOrder(ctx, &ordpb.CreateOrderRequest{UserId: userID, Items: items})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"order_id": resp.GetOrderId(),
			"status":   "CREATED",
		})
		return
	}
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("order_id")
		if id == "" {
			id = r.URL.Query().Get("id")
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if id != "" {
			resp, err := s.ord.GetOrder(ctx, &ordpb.GetOrderRequest{OrderId: id})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
		userID := r.URL.Query().Get("user_id")
		status := r.URL.Query().Get("status")
		if userID != "" {
			resp, err := s.ord.ListOrdersByUser(ctx, &ordpb.ListOrdersByUserRequest{UserId: userID})
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
		resp, err := s.ord.ListOrdersByStatus(ctx, &ordpb.ListOrdersByStatusRequest{Status: status})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
func (s *Server) ordersCancel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.ord.CancelOrder(ctx, &ordpb.CancelOrderRequest{OrderId: req.OrderID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
func (s *Server) ordersStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID   string `json:"order_id"`
		Status    string `json:"status"`
		NewStatus string `json:"new_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	status := req.Status
	if status == "" {
		status = req.NewStatus
	}
	normalized, ok := normalizeOrderStatus(status)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order status"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.ord.UpdateStatus(ctx, &ordpb.UpdateStatusRequest{OrderId: req.OrderID, Status: normalized})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func normalizeOrderStatus(value string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "CREATED", "PENDING", "CONFIRMED", "PAID", "SHIPPED", "CANCELLED", "COMPLETED":
		return strings.ToUpper(strings.TrimSpace(value)), true
	default:
		return "", false
	}
}

func isMutableOrderStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CREATED", "PENDING":
		return true
	default:
		return false
	}
}

func (s *Server) ordersCalculate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			SKU       string `json:"sku"`
			ProductID string `json:"product_id"`
			Qty       int64  `json:"qty"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items are required"})
		return
	}
	qtyBySKU := make(map[string]int64, len(req.Items))
	skus := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		sku := item.SKU
		if sku == "" {
			sku = item.ProductID
		}
		if sku == "" || item.Qty <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "each item must include sku and positive qty"})
			return
		}
		if _, exists := qtyBySKU[sku]; !exists {
			skus = append(skus, sku)
		}
		qtyBySKU[sku] += item.Qty
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	catalogResp, err := s.cat.BulkGetProducts(ctx, &catpb.BulkGetProductsRequest{Sku: skus})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	priceBySKU := make(map[string]float64, len(catalogResp.GetProducts()))
	for _, p := range catalogResp.GetProducts() {
		priceBySKU[p.GetSku()] = p.GetPrice()
	}
	missing := make([]string, 0)
	var total float64
	for sku, qty := range qtyBySKU {
		price, ok := priceBySKU[sku]
		if !ok {
			missing = append(missing, sku)
			continue
		}
		total += float64(qty) * price
	}
	if len(missing) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":        "some sku not found in catalog",
			"missing_skus": missing,
		})
		return
	}
	writeJSON(w, http.StatusOK, &ordpb.CalculateTotalResponse{Total: total})
}
func (s *Server) ordersBulk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderIDs []string `json:"order_ids"`
		OrderID  []string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	orderIDs := req.OrderIDs
	if len(orderIDs) == 0 {
		orderIDs = req.OrderID
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.ord.BulkGetOrders(ctx, &ordpb.BulkGetOrdersRequest{OrderId: orderIDs})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// Notification handlers
func (s *Server) notificationEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.not.SendEmail(ctx, &notpb.SendEmailRequest{To: req.To, Subject: req.Subject, Body: req.Body})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
func (s *Server) notificationOrderConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To            string `json:"to"`
		CustomerEmail string `json:"customer_email"`
		OrderID       string `json:"order_id"`
		OrderDetails  string `json:"order_details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	to := req.To
	if to == "" {
		to = req.CustomerEmail
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.not.SendOrderConfirmation(ctx, &notpb.SendOrderConfirmationRequest{To: to, OrderId: req.OrderID})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
func (s *Server) notificationStockAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To          string `json:"to"`
		SKU         string `json:"sku"`
		WarehouseID string `json:"warehouse_id"`
		CurrentQty  int64  `json:"current_qty"`
		Threshold   int64  `json:"threshold"`
		Body        string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	body := req.Body
	if body == "" {
		body = fmt.Sprintf("warehouse_id=%s current_qty=%d threshold=%d", req.WarehouseID, req.CurrentQty, req.Threshold)
	}
	to := req.To
	if to == "" {
		to = "admin@inventory.local"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := s.not.SendStockAlert(ctx, &notpb.SendStockAlertRequest{To: to, Sku: req.SKU, Body: body})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
