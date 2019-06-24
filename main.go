package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/garyburd/redigo/redis"
)

var (
	addr        = ""
	passwd      = ""
	num         = 0
	concurrency = 1
	sleep       = 0
	qps         = int64(0)
	errs        = int64(0)
)

func init() {
	flag.StringVar(&addr, "addr", "", "Redis address")
	flag.StringVar(&passwd, "passwd", "", "Redis password")
	flag.IntVar(&num, "num", 100000, "The size of coordinates")
	flag.IntVar(&concurrency, "c", 1, "Number of multiple requests to make at a time")
	flag.IntVar(&sleep, "sleep", 0, "The time(microsecond) to sleep after a request")
	flag.Parse()

	if addr == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if num == 0 {
		num = 100000
	}
	if concurrency == 0 {
		concurrency = 1
	}
}

func main() {
	log.Printf("redis: %s, concurrency: %d, init Coordinates: %d, sleep: %d microsecond\n", addr, concurrency, num, sleep)
	pool := newPool(addr, passwd)
	key := fmt.Sprintf("%d", time.Now().UnixNano())
	err := setup(pool, key, num)
	if err != nil {
		log.Printf("setup failed, err: %v\n", err)
		return
	}
	log.Println("setup success")

	bench(pool, key, concurrency, sleep)
	go func() {
		tick := time.NewTicker(1 * time.Second)
		for t := range tick.C {
			log.Printf("%v qps: %d, err: %d\n", t, qps, errs)
			atomic.StoreInt64(&qps, 0)
			atomic.StoreInt64(&errs, 0)
		}
	}()

	select {}
}

func newPool(addr, passwd string) *redis.Pool {
	pool := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 300 * time.Second,
	}
	pool.Dial = func() (redis.Conn, error) {
		c, err := redis.Dial(
			"tcp",
			addr,
			redis.DialConnectTimeout(1*time.Second),
			redis.DialReadTimeout(1*time.Second),
			redis.DialWriteTimeout(1*time.Second),
			redis.DialPassword(passwd))
		if err != nil {
			log.Printf("dial to redis addr: %s, err: %v\n", addr, err)
			return nil, err
		}
		return c, err
	}

	pool.TestOnBorrow = func(c redis.Conn, t time.Time) error {
		_, err := c.Do("Ping")
		if err != nil {
			log.Printf("ping failed, %v\n", err)
		}
		return err
	}
	return pool
}

// setup data.
// It will add num coordinates to redis key.
func setup(pool *redis.Pool, key string, num int) error {
	errnum := int64(0)
	begin := time.Now()
	concurrency := num / 1000
	if concurrency > 100 {
		concurrency = 100
	}
	numper := num / concurrency
	wg := &sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < numper; i++ {
				longitude, latitude, name := randomCoordinates()
				conn := pool.Get()
				_, err := redis.Int(conn.Do("GEOADD", key, longitude, latitude, name))
				if err != nil {
					log.Printf("geoadd %s %f %f %s, err: %v\n", key, longitude, latitude, name, err)
					atomic.AddInt64(&errnum, 1)
				}
				conn.Close()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if int(errnum) == num {
		return fmt.Errorf("failed")
	}
	end := time.Now()
	log.Printf("setup finish, cost: %v\n", end.Sub(begin))
	return nil
}

func bench(pool *redis.Pool, key string, concurrency, sleep int) {
	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				err := request(pool, key)
				if err != nil {
					log.Printf("request failed, key: %s, err: %v", key, err)
				}
				if sleep > 0 {
					time.Sleep(time.Duration(sleep) * time.Microsecond)
				}
			}
		}()
	}
}

func request(pool *redis.Pool, key string) error {
	longitude, latitude, _ := randomCoordinates()
	conn := pool.Get()
	defer conn.Close()
	_, err := redis.Strings(conn.Do(
		"GEORADIUS",
		key,
		longitude,
		latitude,
		10,
		"km",
		"count",
		100))
	atomic.AddInt64(&qps, 1)
	if err != nil {
		atomic.AddInt64(&errs, 1)
	}
	return err
}

// get a tuple of coordinates
// the precision of the result is 6
func randomCoordinates() (float64, float64, string) {
	// 73.0 ~ 136.0
	longitude := float64(rand.Intn(63000000)+73000000) / 1000000.0
	// 3 ~ 54
	latitude := float64(rand.Intn(51000000)+3000000) / 1000000.0
	return longitude, latitude, fmt.Sprintf("%f%f", longitude, latitude)
}
