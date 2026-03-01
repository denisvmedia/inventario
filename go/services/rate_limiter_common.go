package services

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
)

func hashKeyPart(s string) string {
	n := strings.TrimSpace(strings.ToLower(s))
	h := sha256.Sum256([]byte(n))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	default:
		return 0
	}
}

const redisSlidingWindowScript = `
-- Sliding-window rate limiter.
--
-- KEYS[1] = zset key
-- ARGV[1] = now (ns)
-- ARGV[2] = window (ns)
-- ARGV[3] = limit (int)
--
-- Returns: {allowed(0/1), count_after, reset_at_ns}

local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

local window_start = now - window

redis.call('ZREMRANGEBYSCORE', key, 0, window_start)
local count = redis.call('ZCARD', key)

local oldest = nil
if count > 0 then
  local oldest_with_score = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  oldest = tonumber(oldest_with_score[2])
end

if count >= limit then
  local reset_at = (oldest or now) + window
  return {0, count, reset_at}
end

local seq_key = key .. ':seq'
local seq = redis.call('INCR', seq_key)
local member = tostring(now) .. ':' .. tostring(seq)

redis.call('ZADD', key, now, member)
-- We only need to retain data for up to the window.
local ttl_ms = math.ceil(window / 1000000)
redis.call('PEXPIRE', key, ttl_ms)
redis.call('PEXPIRE', seq_key, ttl_ms)

count = count + 1
if oldest == nil then
  oldest = now
end

local reset_at = oldest + window
return {1, count, reset_at}
`
