package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"sync"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"

	"texticulo/backend/middleware"
)

type AuthHandler struct {
	DB *mongo.Database
}

// ── account lockout (5 failures in 10 min → block) ───────────────────────────

type loginAttempt struct {
	count int
	until time.Time
}

var loginLockout = struct {
	mu sync.Mutex
	m  map[string]*loginAttempt
}{m: make(map[string]*loginAttempt)}

func isLockedOut(email string) bool {
	loginLockout.mu.Lock()
	defer loginLockout.mu.Unlock()
	a, ok := loginLockout.m[email]
	if !ok || time.Now().After(a.until) {
		return false
	}
	return a.count >= 5
}

func recordFailedLogin(email string) {
	loginLockout.mu.Lock()
	defer loginLockout.mu.Unlock()
	now := time.Now()
	a, ok := loginLockout.m[email]
	if !ok || now.After(a.until) {
		loginLockout.m[email] = &loginAttempt{count: 1, until: now.Add(10 * time.Minute)}
		return
	}
	a.count++
}

func clearLockout(email string) {
	loginLockout.mu.Lock()
	defer loginLockout.mu.Unlock()
	delete(loginLockout.m, email)
}

// ── input validation ──────────────────────────────────────────────────────────

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validPassword(p string) bool {
	if len(p) < 8 {
		return false
	}
	for _, r := range p {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// ── handlers ─────────────────────────────────────────────────────────────────

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	oid, err := primitive.ObjectIDFromHex(uid)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var user struct {
		Email string `bson:"email"`
	}
	if err := h.DB.Collection("users").FindOne(ctx, bson.M{"_id": oid}).Decode(&user); err != nil {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": uid, "email": user.Email})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	if !emailRegex.MatchString(req.Email) {
		http.Error(w, `{"error":"invalid_email"}`, http.StatusBadRequest)
		return
	}
	if !validPassword(req.Password) {
		http.Error(w, `{"error":"weak_password"}`, http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	res, err := h.DB.Collection("users").InsertOne(ctx, bson.M{
		"email":         req.Email,
		"password_hash": string(hash),
		"created_at":    time.Now().UTC(),
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			http.Error(w, `{"error":"email_taken"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}

	id := res.InsertedID.(primitive.ObjectID).Hex()
	token, err := middleware.IssueToken(id, req.Email)
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token, "email": req.Email})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}

	if isLockedOut(req.Email) {
		http.Error(w, `{"error":"too_many_attempts"}`, http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var user struct {
		ID           primitive.ObjectID `bson:"_id"`
		Email        string             `bson:"email"`
		PasswordHash string             `bson:"password_hash"`
	}
	err := h.DB.Collection("users").FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		// Same generic error for not-found and wrong-password to prevent enumeration
		recordFailedLogin(req.Email)
		http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
		return
	}

	if user.PasswordHash == "" {
		// Google-only account — no password was ever set
		http.Error(w, `{"error":"use_google"}`, http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		recordFailedLogin(req.Email)
		http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
		return
	}

	clearLockout(req.Email)

	token, err := middleware.IssueToken(user.ID.Hex(), user.Email)
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token, "email": user.Email})
}
