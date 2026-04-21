package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
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

// maxURLLen is the maximum accepted length for a destination URL.
const maxURLLen = 2048

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

// ── SSRF guard ────────────────────────────────────────────────────────────────

// dangerousSchemes are explicitly rejected before any normalization.
var dangerousSchemes = []string{
	"file:", "gopher:", "dict:", "ftp:", "blob:", "data:",
	"chrome:", "javascript:", "vbscript:", "ldap:", "mailto:", "tel:",
}

// blockedHostnames are cloud metadata / always-internal hostnames.
var blockedHostnames = []string{
	"metadata.google.internal",
	"metadata.internal",
	"metadata.aws.internal",
	"169.254.169.254",
}

// altIPRe detects hex-encoded IP components (e.g. 0x7f.0x0.0x0.0x1).
var altIPRe = regexp.MustCompile(`(?i)(^|\.)0x[0-9a-f]`)

// looksLikeAltEncodedIP detects non-standard IP representations that Go's
// net.ParseIP does not handle but browsers resolve to private addresses.
func looksLikeAltEncodedIP(hostname string) bool {
	// Hex components: 0x7f.0x0.0x0.0x1
	if altIPRe.MatchString(hostname) {
		return true
	}
	// Single large integer: 2130706433 (= 127.0.0.1 in decimal)
	if !strings.ContainsAny(hostname, ".:[") {
		allDigits := true
		for _, c := range hostname {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits && len(hostname) >= 8 {
			return true
		}
	}
	// Octal-encoded octets: 0177.0.0.1
	for _, part := range strings.Split(hostname, ".") {
		if len(part) > 1 && part[0] == '0' && part[1] >= '0' && part[1] <= '9' {
			return true
		}
	}
	return false
}

// ownDomains returns the set of hostnames operated by this service so that
// shortener-chaining (txtcl.com/x → shorten txtcl.com/y) is blocked.
func ownDomains() []string {
	var hosts []string
	if d := strings.TrimSpace(os.Getenv("SHORT_DOMAIN")); d != "" {
		d = strings.TrimRight(strings.ToLower(d), "/")
		if idx := strings.Index(d, "://"); idx >= 0 {
			d = d[idx+3:]
		}
		hosts = append(hosts, d)
	}
	if d := strings.TrimSpace(os.Getenv("PUBLIC_API_URL")); d != "" {
		if u, err := url.Parse(d); err == nil && u.Hostname() != "" {
			hosts = append(hosts, strings.ToLower(u.Hostname()))
		}
	}
	return hosts
}

// isPrivateHost checks whether a host resolves to a private/loopback IP.
func isPrivateHost(host string) bool {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	// Strip IPv6 brackets
	hostname = strings.Trim(hostname, "[]")

	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.Equal(net.IPv4zero)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil {
		// Unresolvable hostname — reject to be safe
		return true
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil {
			if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.Equal(net.IPv4zero) {
				return true
			}
		}
	}
	return false
}

func normalizeURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLLen {
		return "", false
	}

	lower := strings.ToLower(raw)

	// Reject dangerous schemes explicitly before any prefix addition.
	for _, ds := range dangerousSchemes {
		if strings.HasPrefix(lower, ds) {
			return "", false
		}
	}
	// Any scheme that isn't http/https is rejected.
	if strings.Contains(lower, "://") {
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			return "", false
		}
	}

	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	// Double-check scheme after parsing.
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	// Reject userinfo (user:pass@host) — SSRF bypass vector.
	if u.User != nil {
		return "", false
	}

	hostname := strings.ToLower(u.Hostname())

	// Block 0.0.0.0 explicitly.
	if hostname == "0.0.0.0" {
		return "", false
	}
	// Block non-decimal IP representations browsers resolve to private ranges.
	if looksLikeAltEncodedIP(hostname) {
		return "", false
	}
	// Block known cloud metadata hostnames (DNS-based bypass prevention).
	for _, blocked := range blockedHostnames {
		if hostname == blocked || strings.HasSuffix(hostname, "."+blocked) {
			return "", false
		}
	}
	// Block shortener-chaining on own domains.
	for _, own := range ownDomains() {
		if hostname == own || strings.HasSuffix(hostname, "."+own) {
			return "", false
		}
	}
	// Block private/loopback IPs (including DNS-resolved ones).
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
		now := time.Now().UTC()
		doc := bson.M{
			"short_code":   c,
			"original_url": orig,
			"clicks":       int64(0),
			"created_at":   now,
		}
		if userOID != nil {
			doc["user_id"] = userOID
		} else {
			// Anonymous links expire after 48 hours via sparse TTL index on expires_at.
			doc["expires_at"] = now.Add(48 * time.Hour)
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
