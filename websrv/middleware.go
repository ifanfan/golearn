package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

// LogRequest print a request status
func LogRequest(w ResponseWriteReader, r *http.Request, next func()) {
	t := time.Now()
	next()
	log.Printf("%v %v %v use time %v content-length %v",
		r.Method,
		w.StatusCode(),
		r.URL.String(),
		time.Now().Sub(t).String(),
		w.ContentLength())
}

// ErrCatch catch and recover
func ErrCatch(w ResponseWriteReader, r *http.Request, next func()) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			debug.PrintStack()
			w.WriteHeader(http.StatusInternalServerError) // 500
		}
	}()
	next()
	if w.StatusCode() == 404 {
		w.Write([]byte("404!"))
	}
}

const cookieName = "cid"

// CreateCookie if request cookies.length == 0 then add a cookie
func CreateCookie(w ResponseWriteReader, r *http.Request, next func()) {

	if _, err := r.Cookie(cookieName); err != nil {
		c := new(http.Cookie)
		c.HttpOnly = true
		c.Expires = time.Now().Add(time.Hour)
		c.Name = cookieName
		c.Value = randStr(40)
		http.SetCookie(w, c)
	}
	next()
}

const strs = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var randsrc = rand.NewSource(time.Now().UnixNano())

// randStr rand string
func randStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = strs[randsrc.Int63()%int64(len(strs))]
	}
	return string(b)
}

// AuthInfo login info
type AuthInfo struct {
	ID        int
	UID       []byte
	loginTime time.Time
	expries   time.Time
}

// AuthCheck auth plugin
type AuthCheck struct {
	lock  sync.RWMutex // for thread safe
	infos map[string]*AuthInfo
}

// NewAuthCheck Create AuthCheck
func NewAuthCheck() *AuthCheck {
	auth := new(AuthCheck)
	auth.infos = make(map[string]*AuthInfo)
	go func() {
		for {
			var eks []string
			now := time.Now()
			auth.lock.RLock()
			for key, info := range auth.infos {
				if info.expries.Before(now) {
					eks = append(eks, key)
				}
			}
			auth.lock.RUnlock()
			auth.Remove(eks...)
			time.Sleep(10 * time.Second)
		}
	}()
	return auth
}

// Remove delete auth info
func (p *AuthCheck) Remove(keys ...string) {
	if len(keys) > 0 {
		p.lock.Lock()
		for _, key := range keys {
			delete(p.infos, key)
		}
		p.lock.Unlock()
	}
}

// FilterFunc as middleware func
func (p *AuthCheck) FilterFunc(w ResponseWriteReader, r *http.Request, next func()) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized) // 401
		return
	}
	p.lock.RLock()
	info := p.infos[cookie.Value]
	p.lock.RUnlock()
	if info != nil {
		next()
	}
}
