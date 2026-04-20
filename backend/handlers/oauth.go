package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"texticulo/backend/middleware"
)

const oauthStateCookie = "oauth_state"

type OAuthHandler struct {
	DB          *mongo.Database
	Google      *oauth2.Config
	FrontendURL string
}

func oauthRedirectBase() string {
	b := strings.TrimRight(os.Getenv("PUBLIC_API_URL"), "/")
	if b == "" {
		return "http://localhost:8080"
	}
	return b
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *OAuthHandler) setStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(strings.ToLower(oauthRedirectBase()), "https://"),
	})
}

func (h *OAuthHandler) readStateCookie(r *http.Request) (string, error) {
	c, err := r.Cookie(oauthStateCookie)
	if err != nil {
		return "", err
	}
	return c.Value, nil
}

func (h *OAuthHandler) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *OAuthHandler) redirectFrontendError(w http.ResponseWriter, r *http.Request, msg string) {
	u := strings.TrimRight(h.FrontendURL, "/") + "/login?error=" + url.QueryEscape(msg)
	http.Redirect(w, r, u, http.StatusFound)
}

func (h *OAuthHandler) redirectFrontendSuccess(w http.ResponseWriter, r *http.Request, token, email string) {
	u, err := url.Parse(strings.TrimRight(h.FrontendURL, "/") + "/login")
	if err != nil {
		http.Error(w, "config error", http.StatusInternalServerError)
		return
	}
	q := u.Query()
	q.Set("token", token)
	q.Set("email", email)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// GoogleStart redirects to Google OAuth consent screen.
func (h *OAuthHandler) GoogleStart(w http.ResponseWriter, r *http.Request) {
	if h.Google == nil || h.Google.ClientID == "" {
		http.Error(w, `{"error":"oauth_not_configured"}`, http.StatusServiceUnavailable)
		return
	}
	state, err := randomState()
	if err != nil {
		http.Error(w, `{"error":"server"}`, http.StatusInternalServerError)
		return
	}
	h.setStateCookie(w, state)
	authURL := h.Google.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// GoogleCallback handles Google OAuth redirect.
func (h *OAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		h.clearStateCookie(w)
		h.redirectFrontendError(w, r, "oauth_denied")
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	cookieState, err := h.readStateCookie(r)
	h.clearStateCookie(w)
	if err != nil || state == "" || cookieState == "" || state != cookieState {
		h.redirectFrontendError(w, r, "invalid_state")
		return
	}
	if code == "" {
		h.redirectFrontendError(w, r, "missing_code")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	tok, err := h.Google.Exchange(ctx, code)
	if err != nil {
		h.redirectFrontendError(w, r, "token_exchange")
		return
	}

	client := h.Google.Client(ctx, tok)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		h.redirectFrontendError(w, r, "userinfo")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		h.redirectFrontendError(w, r, "userinfo")
		return
	}
	var gu struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &gu); err != nil {
		h.redirectFrontendError(w, r, "userinfo")
		return
	}
	email := strings.TrimSpace(strings.ToLower(gu.Email))
	if email == "" {
		h.redirectFrontendError(w, r, "no_email")
		return
	}

	h.finishOAuth(w, r, email)
}

func (h *OAuthHandler) finishOAuth(w http.ResponseWriter, r *http.Request, email string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var user struct {
		ID    primitive.ObjectID `bson:"_id"`
		Email string             `bson:"email"`
	}
	err := h.DB.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err == mongo.ErrNoDocuments {
		res, insErr := h.DB.Collection("users").InsertOne(ctx, bson.M{
			"email":      email,
			"created_at": time.Now().UTC(),
		})
		if insErr != nil {
			if mongo.IsDuplicateKeyError(insErr) {
				if err2 := h.DB.Collection("users").FindOne(ctx, bson.M{"email": email}).Decode(&user); err2 != nil {
					h.redirectFrontendError(w, r, "db")
					return
				}
			} else {
				h.redirectFrontendError(w, r, "db")
				return
			}
		} else {
			user.ID = res.InsertedID.(primitive.ObjectID)
			user.Email = email
		}
	} else if err != nil {
		h.redirectFrontendError(w, r, "db")
		return
	}

	id := user.ID.Hex()
	jwtStr, err := middleware.IssueToken(id, user.Email)
	if err != nil {
		h.redirectFrontendError(w, r, "jwt")
		return
	}
	h.redirectFrontendSuccess(w, r, jwtStr, user.Email)
}

// NewGoogleOAuthConfig builds oauth2.Config for Google from environment.
func NewGoogleOAuthConfig() *oauth2.Config {
	gID := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	gSec := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
	base := oauthRedirectBase()
	if gID == "" || gSec == "" {
		return nil
	}
	return &oauth2.Config{
		ClientID:     gID,
		ClientSecret: gSec,
		RedirectURL: base + "/api/auth/google/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}
