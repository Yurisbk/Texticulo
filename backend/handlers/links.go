package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"texticulo/backend/middleware"
)

const base62 = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type LinksHandler struct {
	DB *mongo.Database
}

type shortenResp struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
}

// publicBase is used for the redirect (API server URL).
func publicBase() string {
	b := strings.TrimRight(os.Getenv("PUBLIC_API_URL"), "/")
	if b == "" {
		b = "http://localhost:8080"
	}
	return b
}

// shortBase returns the domain used in displayed short URLs.
// If SHORT_DOMAIN is set (e.g. "txt.io"), it builds "https://txt.io".
// Otherwise falls back to publicBase().
func shortBase() string {
	d := strings.TrimSpace(os.Getenv("SHORT_DOMAIN"))
	if d == "" {
		return publicBase()
	}
	d = strings.TrimRight(d, "/")
	if strings.Contains(d, "://") {
		return d
	}
	return "https://" + d
}

// isPrivateHost checks if a URL host resolves to a private/loopback IP (SSRF guard).
func isPrivateHost(host string) bool {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
				return true
			}
		}
	}
	return false
}

func normalizeURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	if isPrivateHost(u.Host) {
		return "", false
	}
	return u.String(), true
}

func randomCode(n int) (string, error) {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		v := make([]byte, 1)
		if _, err := rand.Read(v); err != nil {
			return "", err
		}
		b[i] = base62[int(v[0])%len(base62)]
	}
	return string(b), nil
}

func (h *LinksHandler) Shorten(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
		return
	}
	orig, ok := normalizeURL(req.URL)
	if !ok {
		http.Error(w, `{"error":"invalid_url"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var userOID *primitive.ObjectID
	if uid, ok := middleware.UserIDFromContext(r.Context()); ok {
		oid, err := primitive.ObjectIDFromHex(uid)
		if err == nil {
			count, _ := h.DB.Collection("links").CountDocuments(ctx, bson.M{"user_id": oid})
			if count >= 5 {
				http.Error(w, `{"error":"link_limit"}`, http.StatusForbidden)
				return
			}
			userOID = &oid
		}
	}

	var code string
	for attempt := 0; attempt < 8; attempt++ {
		c, err := randomCode(6)
		if err != nil {
			http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
			return
		}
		doc := bson.M{
			"short_code":   c,
			"original_url": orig,
			"clicks":       int64(0),
			"created_at":   time.Now().UTC(),
		}
		if userOID != nil {
			doc["user_id"] = userOID
		}
		_, err = h.DB.Collection("links").InsertOne(ctx, doc)
		if err == nil {
			code = c
			break
		}
		if !mongo.IsDuplicateKeyError(err) {
			http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
			return
		}
	}
	if code == "" {
		http.Error(w, `{"error":"try_again"}`, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(shortenResp{
		ShortCode: code,
		ShortURL:  shortBase() + "/" + code,
	})
}

func (h *LinksHandler) List(w http.ResponseWriter, r *http.Request) {
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

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := h.DB.Collection("links").Find(ctx, bson.M{"user_id": oid}, opts)
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var docs []struct {
		ShortCode   string    `bson:"short_code"`
		OriginalURL string    `bson:"original_url"`
		Clicks      int64     `bson:"clicks"`
		CreatedAt   time.Time `bson:"created_at"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}

	type outRow struct {
		ShortCode   string    `json:"short_code"`
		OriginalURL string    `json:"original_url"`
		Clicks      int64     `json:"clicks"`
		CreatedAt   time.Time `json:"created_at"`
		ShortURL    string    `json:"short_url"`
	}

	out := make([]outRow, len(docs))
	base := shortBase()
	for i, doc := range docs {
		out[i] = outRow{
			ShortCode:   doc.ShortCode,
			OriginalURL: doc.OriginalURL,
			Clicks:      doc.Clicks,
			CreatedAt:   doc.CreatedAt,
			ShortURL:    base + "/" + doc.ShortCode,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"links": out})
}

func (h *LinksHandler) Stats(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
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

	var link struct {
		ID          primitive.ObjectID  `bson:"_id"`
		OriginalURL string              `bson:"original_url"`
		Clicks      int64               `bson:"clicks"`
		CreatedAt   time.Time           `bson:"created_at"`
		UserID      *primitive.ObjectID `bson:"user_id"`
	}
	err = h.DB.Collection("links").FindOne(ctx, bson.M{"short_code": code}).Decode(&link)
	if err == mongo.ErrNoDocuments {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}
	if link.UserID == nil || *link.UserID != oid {
		// Return 404 instead of 403 to avoid resource enumeration
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}

	clickOpts := options.Find().SetSort(bson.D{{Key: "clicked_at", Value: -1}}).SetLimit(100)
	cursor, err := h.DB.Collection("clicks").Find(ctx, bson.M{"link_id": link.ID}, clickOpts)
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var clickDocs []struct {
		ClickedAt time.Time `bson:"clicked_at"`
	}
	_ = cursor.All(ctx, &clickDocs)

	recent := make([]string, len(clickDocs))
	for i, c := range clickDocs {
		recent[i] = c.ClickedAt.UTC().Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"short_code":    code,
		"original_url":  link.OriginalURL,
		"clicks":        link.Clicks,
		"created_at":    link.CreatedAt,
		"short_url":     shortBase() + "/" + code,
		"recent_clicks": recent,
	})
}

func (h *LinksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
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

	var link struct {
		ID     primitive.ObjectID  `bson:"_id"`
		UserID *primitive.ObjectID `bson:"user_id"`
	}
	err = h.DB.Collection("links").FindOne(ctx, bson.M{"short_code": code}).Decode(&link)
	if err == mongo.ErrNoDocuments {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}
	if link.UserID == nil || *link.UserID != oid {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
		return
	}

	if _, err := h.DB.Collection("links").DeleteOne(ctx, bson.M{"_id": link.ID}); err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// reservedShortCodes are codes the redirect handler must not process.
// robots.txt and sitemap.xml are served by dedicated routes registered before /{code}.
var reservedShortCodes = map[string]struct{}{
	"api": {}, "health": {}, "favicon.ico": {}, "favicon.svg": {},
}

func (h *LinksHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.NotFound(w, r)
		return
	}
	if _, reserved := reservedShortCodes[code]; reserved {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var link struct {
		ID          primitive.ObjectID `bson:"_id"`
		OriginalURL string             `bson:"original_url"`
	}
	if err := h.DB.Collection("links").FindOne(ctx, bson.M{"short_code": code}).Decode(&link); err != nil {
		if err == mongo.ErrNoDocuments {
			http.NotFound(w, r)
		} else {
			http.Error(w, "server error", http.StatusInternalServerError)
		}
		return
	}

	ip := r.RemoteAddr
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		ip = strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	ua := r.Header.Get("User-Agent")

	go func() {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bgCancel()
		_, _ = h.DB.Collection("clicks").InsertOne(bgCtx, bson.M{
			"link_id":    link.ID,
			"clicked_at": time.Now().UTC(),
			"ip_address": ip,
			"user_agent": ua,
		})
		_, _ = h.DB.Collection("links").UpdateOne(bgCtx,
			bson.M{"_id": link.ID},
			bson.M{"$inc": bson.M{"clicks": 1}},
		)
	}()

	http.Redirect(w, r, link.OriginalURL, http.StatusFound)
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}
