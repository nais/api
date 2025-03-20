package authn

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/nais/api/internal/session"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	RedirectURICookie = "redirecturi"
	OAuthStateCookie  = "oauthstate"
	SessionCookieName = "session_id"
	IDTokenKey        = "id_token"
)

type OAuth2 interface {
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

type Handler interface {
	Login(w http.ResponseWriter, r *http.Request)
	Callback(w http.ResponseWriter, r *http.Request)
	Logout(w http.ResponseWriter, r *http.Request)
	SetSessionCookie(w http.ResponseWriter, session *session.Session)
	DeleteCookie(w http.ResponseWriter, name string)
}

type handler struct {
	oauth2Config OAuth2
	log          logrus.FieldLogger
}

func New(oauth2Config OAuth2, log logrus.FieldLogger) Handler {
	return &handler{
		oauth2Config: oauth2Config,
		log:          log,
	}
}

type claims struct {
	Email string
}

func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	redirectURI := r.URL.Query().Get("redirect_uri")
	if len(redirectURI) > 0 && strings.HasPrefix(redirectURI, "/") {
		http.SetCookie(w, &http.Cookie{
			Name:     RedirectURICookie,
			Value:    redirectURI,
			Path:     "/",
			Expires:  time.Now().Add(30 * time.Minute),
			Secure:   true,
			HttpOnly: true,
		})
	}

	oauthState := uuid.New().String()
	http.SetCookie(w, &http.Cookie{
		Name:     OAuthStateCookie,
		Value:    oauthState,
		Path:     "/",
		Expires:  time.Now().Add(30 * time.Minute),
		Secure:   true,
		HttpOnly: true,
	})

	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(oauthState, oauth2.SetAuthURLParam("prompt", "select_account")), http.StatusFound)
}

func (h *handler) Callback(w http.ResponseWriter, r *http.Request) {
	frontendURL := "/"

	redirectURIRaw, err := r.Cookie(RedirectURICookie)
	if err == nil {
		if redirectPath, err := url.QueryUnescape(redirectURIRaw.Value); err == nil {
			frontendURL = redirectPath
		}
	}

	h.DeleteCookie(w, RedirectURICookie)
	code := r.URL.Query().Get("code")
	if len(code) == 0 {
		h.log.WithError(fmt.Errorf("missing query parameter")).Error("check code param")
		http.Redirect(w, r, "/?error=unauthenticated", http.StatusFound)
		return
	}

	oauthCookie, err := r.Cookie(OAuthStateCookie)
	if err != nil {
		h.log.WithError(err).Error("missing oauth state cookie")
		http.Redirect(w, r, "/?error=invalid-state", http.StatusFound)
		return
	}

	h.DeleteCookie(w, OAuthStateCookie)
	state := r.URL.Query().Get("state")
	if state != oauthCookie.Value {
		h.log.WithError(fmt.Errorf("state mismatch")).Error("check incoming state matches local state")
		http.Redirect(w, r, "/?error=invalid-state", http.StatusFound)
		return
	}

	tokens, err := h.oauth2Config.Exchange(r.Context(), code)
	if err != nil {
		h.log.WithError(err).Error("exchanging authorization code for tokens")
		http.Redirect(w, r, "/?error=unauthenticated", http.StatusFound)
		return
	}

	rawIDToken, ok := tokens.Extra(IDTokenKey).(string)
	if !ok {
		h.log.WithError(fmt.Errorf("missing id_token")).Error("id token presence")
		http.Redirect(w, r, "/?error=unauthenticated", http.StatusFound)
		return
	}

	idToken, err := h.oauth2Config.Verify(r.Context(), rawIDToken)
	if err != nil {
		h.log.WithError(err).Error("verify id_token")
		http.Redirect(w, r, "/?error=unauthenticated", http.StatusFound)
		return
	}

	claims := &claims{}
	if err := idToken.Claims(claims); err != nil {
		h.log.WithError(err).Error("parse claims")
		http.Redirect(w, r, "/?error=unauthenticated", http.StatusFound)
		return
	}

	u, err := user.GetByEmail(r.Context(), claims.Email)
	if err != nil {
		h.log.WithError(err).Errorf("get user (%s) from db", claims.Email)
		http.Redirect(w, r, "/?error=unknown-user", http.StatusFound)
		return
	}

	sess, err := session.Create(r.Context(), u.UUID)
	if err != nil {
		h.log.WithError(err).Error("create session")
		http.Redirect(w, r, "/?error=unable-to-create-session", http.StatusFound)
		return
	}

	h.SetSessionCookie(w, sess)
	http.Redirect(w, r, frontendURL, http.StatusFound)
}

func (h *handler) Logout(w http.ResponseWriter, r *http.Request) {
	redirectUrl := "/"
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		h.log.WithError(err).Error("logout session")
		http.Redirect(w, r, redirectUrl, http.StatusFound)
		return
	}

	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		h.log.WithError(err).Error("parse cookie value as UUID")
		http.Redirect(w, r, redirectUrl, http.StatusFound)
		return
	}

	if err := session.Delete(r.Context(), sessionID); err != nil {
		h.log.WithError(err).Error("delete session from database")
		http.Redirect(w, r, redirectUrl, http.StatusFound)
		return
	}

	http.Redirect(w, r, redirectUrl, http.StatusFound)
}

func (h *handler) SetSessionCookie(w http.ResponseWriter, session *session.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.ID.String(),
		Path:     "/",
		Expires:  session.Expires,
		Secure:   true,
		HttpOnly: true,
	})
}

func (h *handler) DeleteCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		Secure:   true,
		HttpOnly: true,
	})
}
