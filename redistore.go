package sessions

import (
	"bytes"
	"encoding/gob"
	"github.com/garyburd/redigo/redis"
	"net/http"
)

var sessionExpire = 86400 * 30

type RedisStore struct {
	*http.Cookie
	*redis.Pool
	exitChannel   chan int
	queryChannel  chan *RedisQuery
	DefaultMaxAge int
}

func NewRediStore(network, address, password string) *RedisStore {
	con := func() (redis.Conn, error) {
		c, err := redis.Dial(network, address)
		if err != nil {
			return nil, err
		}
		if password != "" {
			if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
		}
		return c, err
	}
	return &RedisStore{
		Pool: redis.NewPool(con, 3),
		Cookie: &http.Cookie{
			Path:   "/",
			MaxAge: sessionExpire,
		},
		DefaultMaxAge: sessionExpire,
	}
}

type RedisQuery struct {
	Action        string
	Options       []interface{}
	resultChannel chan *QueryResult
}

type QueryResult struct {
	Err   error
	Value interface{}
}

func (q *RedisStore) Run() {
	con := q.Pool.Get()
	defer con.Close()
	for {
		select {
		case <-q.exitChannel:
			return
		case query := <-q.queryChannel:
			value, err := con.Do(query.Action, query.Options...)
			if err != nil && err != redis.ErrNil {
				con.Close()
				con = q.Pool.Get()
			}
			query.resultChannel <- &QueryResult{
				Err:   err,
				Value: value,
			}
		}
	}
}

func (q *RedisStore) Stop() {
	close(q.exitChannel)
	q.Pool.Close()
}

func (q *RedisStore) Get(r *http.Request, name string) (*Session, error) {
	c, err := r.Cookie(name)
	var s *Session
	if err != nil {
		return s, err
	}
	s = &Session{
		Cookie: c,
		Values: make(map[interface{}]interface{}),
	}
	query := &RedisQuery{
		Action:        "GET",
		Options:       []interface{}{"session_:" + c.Value},
		resultChannel: make(chan *QueryResult),
	}
	q.queryChannel <- query
	queryresult := <-query.resultChannel
	value, err := redis.Bytes(queryresult.Value, queryresult.Err)
	if err == nil {
		dec := gob.NewDecoder(bytes.NewBuffer(value))
		dec.Decode(&s.Values)
	}
	return s, err
}

func (q *RedisStore) Set(r *http.Request, w http.ResponseWriter, session *Session) error {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(session.Values)
	if err != nil {
		return err
	}
	b := buf.Bytes()
	age := session.MaxAge
	query := &RedisQuery{
		Action:        "SETEX",
		Options:       []interface{}{"session_:" + session.Value, age, b},
		resultChannel: make(chan *QueryResult),
	}
	q.queryChannel <- query
	queryresult := <-query.resultChannel
	if queryresult.Err == nil {
		http.SetCookie(w, session.Cookie)
	}
	return queryresult.Err
}
