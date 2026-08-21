package lib

import (
	"net/http"
	"os"

	"github.com/rs/cors"
	"go.uber.org/zap"
	"golang.org/x/net/webdav"
)

type handlerUser struct {
	User
	webdav.Handler
}

type Handler struct {
	noPassword     bool
	behindProxy    bool
	browserListing BrowserListing
	prefix         string
	user           *handlerUser
	users          map[string]*handlerUser
}

func NewHandler(c *Config) (http.Handler, error) {
	ls := webdav.NewMemLS()

	logFunc := func(r *http.Request, err error) {
		lZap := getRequestLogger(r, c.BehindProxy)
		lZap.Debug("handle webdav request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Error(err))
	}

	h := &Handler{
		noPassword:     c.NoPassword,
		behindProxy:    c.BehindProxy,
		browserListing: c.BrowserListing,
		prefix:         c.Prefix,
		user: &handlerUser{
			User:    User{UserPermissions: c.UserPermissions},
			Handler: buildWebdavHandler(c.UserPermissions, c.Prefix, c.NoSniff, ls, logFunc),
		},
		users: map[string]*handlerUser{},
	}

	for _, u := range c.Users {
		h.users[u.Username] = &handlerUser{
			User:    u,
			Handler: buildWebdavHandler(u.UserPermissions, c.Prefix, c.NoSniff, ls, logFunc),
		}
	}

	if c.CORS.Enabled {
		return cors.New(cors.Options{
			AllowCredentials:   c.CORS.Credentials,
			AllowedOrigins:     c.CORS.AllowedHosts,
			AllowedMethods:     c.CORS.AllowedMethods,
			AllowedHeaders:     c.CORS.AllowedHeaders,
			ExposedHeaders:     c.CORS.ExposedHeaders,
			OptionsPassthrough: false,
		}).Handler(h), nil
	}

	if len(c.Users) == 0 {
		zap.L().Warn("unprotected config: no users have been set, so no authentication will be used")
	}

	if c.NoPassword {
		zap.L().Warn("unprotected config: password check is disabled, only intended when delegating authentication to another service")
	}

	return h, nil
}

// buildWebdavHandler creates the [webdav.Handler] for a set of user permissions,
// selecting between single-directory and multi-directory backing depending on
// whether directories are configured.
func buildWebdavHandler(p UserPermissions, prefix string, noSniff bool, ls webdav.LockSystem, logFunc func(*http.Request, error)) webdav.Handler {
	h := webdav.Handler{
		Prefix: prefix,
		Logger: logFunc,
	}

	if p.useDirectories {
		h.FileSystem = multiDir{
			mounts:  p.Directories,
			noSniff: noSniff,
		}
		h.LockSystem = newMultiDirLockSystem(ls, p.Directories)
	} else {
		h.FileSystem = Dir{
			Dir:     webdav.Dir(p.Directory),
			noSniff: noSniff,
		}
		h.LockSystem = newLockSystem(ls, p.Directory)
	}

	return h
}

// ServeHTTP determines if the request is for this plugin, and if all prerequisites are met.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := h.user

	lZap := getRequestLogger(r, h.behindProxy)

	// Authentication
	if len(h.users) > 0 {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

		// Gets the correct user for this request.
		username, password, ok := r.BasicAuth()
		if !ok {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		user, ok = h.users[username]
		if !ok {
			// Log invalid username
			lZap.Info("invalid username", zap.String("username", username))
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		if !h.noPassword && !user.checkPassword(password) {
			// Log invalid password
			lZap.Info("invalid password", zap.String("username", username))
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		// Log successful authorization
		lZap.Info("user authorized", zap.String("username", username))
	}

	// Convert the HTTP request into an internal request type
	req, err := newRequest(r, h.user.Prefix)
	if err != nil {
		lZap.Info("invalid request path or destination", zap.Error(err))
		http.Error(w, "Invalid request path or destination", http.StatusBadRequest)
		return
	}

	// Checks for user permissions relatively to this PATH.
	allowed := user.Allowed(req, func(filename string) bool {
		_, err := user.FileSystem.Stat(r.Context(), filename)
		return !os.IsNotExist(err)
	})

	lZap.Debug("allowed & method & path", zap.Bool("allowed", allowed), zap.String("method", r.Method), zap.String("path", r.URL.Path))

	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if r.Method == "HEAD" {
		w = responseWriterNoBody{w}
	}

	// Excerpt from RFC4918, section 9.4:
	//
	// 		GET, when applied to a collection, may return the contents of an
	//		"index.html" resource, a human-readable view of the contents of
	//		the collection, or something else altogether.
	//
	//    Similarly, since the definition of HEAD is a GET without a response
	// 		message body, the semantics of HEAD are unmodified when applied to
	// 		collection resources.
	//
	// GET (or HEAD), when applied to collection, will serve directory listing
	// if browser listing is enabled, otherwise return same as PROPFIND method.
	if r.Method == "GET" || r.Method == "HEAD" {
		info, err := user.FileSystem.Stat(r.Context(), req.path)
		if err == nil && info.IsDir() {
			if h.browserListing.Enabled {
				sorting := ParseListingSortQuery(r.URL.Query())
				html, err := RenderDirectoryListing(
					r.Context(),
					user.FileSystem,
					req.path,
					sorting,
					h.browserListing,
				)
				if err != nil {
					lZap.Error("failed to render directory listing", zap.Error(err))
					http.Error(w, "Failed to render directory listing", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if r.Method == "GET" {
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte(html)); err != nil {
						lZap.Error("failed to write directory listing", zap.Error(err))
					}
				} else {
					// HEAD: write headers only
					w.WriteHeader(http.StatusOK)
				}
				return
			}

			r.Method = "PROPFIND"

			if r.Header.Get("Depth") == "" {
				r.Header.Add("Depth", "1")
			}
		}
	}

	if r.Method == "OPTIONS" {
		user.handleOptions(w, r, req.path)
		return
	}

	if r.Method == "PATCH" || (r.Method == "PUT" && r.Header.Get("Content-Range") != "") {
		user.handlePartialUpdate(w, r, req.path)
		return
	}

	// Runs the WebDAV.
	user.ServeHTTP(w, r)
}

// getRequestLogger creates a zap.Logger using the request remote ip.
func getRequestLogger(r *http.Request, behindProxy bool) *zap.Logger {
	// Retrieve the real client IP address using the updated helper function
	remoteAddr := getRealRemoteIP(r, behindProxy)

	return zap.L().With(zap.String("remote_address", remoteAddr))
}

// getRealRemoteIP retrieves the client's actual IP address, considering reverse proxies.
func getRealRemoteIP(r *http.Request, behindProxy bool) string {
	if behindProxy {
		if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
			return ip
		}
	}
	return r.RemoteAddr
}

type responseWriterNoBody struct {
	http.ResponseWriter
}

func (w responseWriterNoBody) Write(data []byte) (int, error) {
	return len(data), nil
}
