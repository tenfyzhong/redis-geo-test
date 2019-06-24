# redis-geo-test
This repo is a tool to test the performance of redis geohash.

You can start a redis by docker. I wrote a docker-compse configuration named `redis.yml`.
It's very easy to start a redis process by `docker-compose -f redis.yml up -d`.

The test client is a golang program. Use `go build` to build a program.
The usage of `redis-geo-test`:
```
Usage of ./redis-geo-test:
  -addr string
        Redis address
  -c int
        Number of multiple requests to make at a time (default 1)
  -num int
        The size of coordinates (default 100000)
  -passwd string
        Redis password
  -sleep int
        The time(microsecond) to sleep after a request
```

You can make a test now. Example: 
```
./redis-geo-test -addr redis-address:6279 -c 20 -num 100000 -sleep 600
```
