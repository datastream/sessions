package sessions

import "net/http"

func NewSession(store RedisStore, name string) *Session {
	s := &Session{
		Values: make(map[interface{}]interface{}),
		Cookie: &http.Cookie{},
	}
	s.Path = store.Path
	s.Domain = store.Domain
	s.MaxAge = store.DefaultMaxAge
	s.Secure = store.Secure
	s.HttpOnly = store.HttpOnly
	return s
}

type Session struct {
	*http.Cookie
	Values map[interface{}]interface{}
}

type Store interface {
	Get(r *http.Request, name string) (*Session, error)
	Set(r *http.Request, w http.ResponseWriter, s *Session) error
}
