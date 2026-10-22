package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Cache stores small values (media metadata by slug). Backed by memory, or by any
// Redis-protocol server (Valkey, KeyDB, Dragonfly, Redis) when SSX_REDIS is set.
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, val []byte, ttl time.Duration)
	Del(key string)
}

type memEntry struct {
	val []byte
	exp time.Time
}

type memCache struct {
	mu sync.Mutex
	m  map[string]memEntry
}

func newMemCache() *memCache { return &memCache{m: map[string]memEntry{}} }

func (c *memCache) Get(k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Now().After(e.exp) {
		delete(c.m, k)
		return nil, false
	}
	return e.val, true
}

func (c *memCache) Set(k string, v []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 10000 { // crude bound: drop everything rather than track LRU
		c.m = map[string]memEntry{}
	}
	c.m[k] = memEntry{v, time.Now().Add(ttl)}
}

func (c *memCache) Del(k string) {
	c.mu.Lock()
	delete(c.m, k)
	c.mu.Unlock()
}

// respCache is a minimal RESP2 client: one connection, serialized commands.
// Errors are swallowed: the cache is an optimisation, the DB stays the source of truth.
type respCache struct {
	mu   sync.Mutex
	addr string
	pass string
	db   string
	conn net.Conn
	rd   *bufio.Reader
}

func newRespCache(raw string) (*respCache, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	c := &respCache{addr: u.Host}
	if p, ok := u.User.Password(); ok {
		c.pass = p
	}
	if len(u.Path) > 1 {
		c.db = u.Path[1:]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.do("PING")
	return c, err
}

func (c *respCache) do(args ...string) (any, error) {
	if c.conn == nil {
		conn, err := net.DialTimeout("tcp", c.addr, 2*time.Second)
		if err != nil {
			return nil, err
		}
		c.conn, c.rd = conn, bufio.NewReader(conn)
		if c.pass != "" {
			if _, err := c.send("AUTH", c.pass); err != nil {
				return nil, c.reset(err)
			}
		}
		if c.db != "" {
			if _, err := c.send("SELECT", c.db); err != nil {
				return nil, c.reset(err)
			}
		}
	}
	v, err := c.send(args...)
	if err != nil {
		return nil, c.reset(err)
	}
	return v, nil
}

func (c *respCache) reset(err error) error {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	return err
}

func (c *respCache) send(args ...string) (any, error) {
	c.conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := fmt.Appendf(nil, "*%d\r\n", len(args))
	for _, a := range args {
		buf = fmt.Appendf(buf, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := c.conn.Write(buf); err != nil {
		return nil, err
	}
	line, err := c.rd.ReadString('\n')
	if err != nil || len(line) < 3 {
		return nil, fmt.Errorf("resp: bad reply %q: %v", line, err)
	}
	body := line[1 : len(line)-2]
	switch line[0] {
	case '+', ':':
		return body, nil
	case '-':
		return nil, fmt.Errorf("resp: %s", body)
	case '$':
		n, _ := strconv.Atoi(body)
		if n < 0 {
			return nil, nil
		}
		data := make([]byte, n+2)
		if _, err := io.ReadFull(c.rd, data); err != nil {
			return nil, err
		}
		return data[:n], nil
	}
	return nil, fmt.Errorf("resp: unsupported reply %q", line)
}

func (c *respCache) Get(k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, err := c.do("GET", "ssx:"+k)
	b, ok := v.([]byte)
	return b, err == nil && ok
}

func (c *respCache) Set(k string, v []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.do("SET", "ssx:"+k, string(v), "EX", strconv.Itoa(int(ttl.Seconds())))
}

func (c *respCache) Del(k string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.do("DEL", "ssx:"+k)
}
