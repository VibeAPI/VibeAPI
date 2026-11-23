package model

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/go-redis/redis/v8"
)

// channelUserRuleWindowSeconds is the counting window behind a rule's RPM.
const channelUserRuleWindowSeconds = 60

// channelUserRuleWindowsPruneSize is the number of in-memory counters above
// which a new hit also drops the counters whose window has ended.
const channelUserRuleWindowsPruneSize = 4096

// channelUserRuleRedisTimeout bounds each counter call to Redis. The check
// runs while channel selection holds the channel cache lock, so a slow Redis
// must not hold it longer than this; a call that times out lets the request
// use the channel.
const channelUserRuleRedisTimeout = 100 * time.Millisecond

// channelUserRuleHitScript counts one request in a rule's one-minute window
// and starts the window on its first request.
const channelUserRuleHitScript = `
local count = redis.call('INCR', KEYS[1])
if count == 1 or redis.call('TTL', KEYS[1]) < 0 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`

// channelUserRuleWindowScript reads a rule's counter and the seconds left in
// its window in one round trip.
const channelUserRuleWindowScript = `
local count = tonumber(redis.call('GET', KEYS[1]) or '0')
return {count, redis.call('TTL', KEYS[1])}
`

type channelUserRuleWindow struct {
	start int64
	count int
}

// channelUserRuleWindows holds the rule counters when Redis is not used. Each
// instance then counts only the requests it serves.
var channelUserRuleWindows = struct {
	sync.Mutex
	byKey map[string]channelUserRuleWindow
}{byKey: make(map[string]channelUserRuleWindow)}

// channelUserRuleKey names a rule's counter by the rule's position in the
// channel's list, so reordering or deleting rules can move counts to another
// rule until the current minute ends.
func channelUserRuleKey(channelId int, ruleIndex int, userId int) string {
	return fmt.Sprintf("channelUserRule:v1:%d:%d:%d", channelId, ruleIndex, userId)
}

// channelAllowsUserRequest applies the channel's user blacklist and user rules
// to one request. A rule with an RPM only keeps the request off the channel
// once the user has sent the channel that many matching requests in the
// current minute; that case is reported through filter.RuleLimit. Blocking
// rules are checked first, so a channel that waiting cannot free is never
// reported as rate limited.
//
// The count is read here and increased only when the request is sent
// (RecordChannelUserRuleHits), so requests that arrive together can pass the
// same check and go a few over the limit.
func channelAllowsUserRequest(ch *Channel, modelName string, filter dto.ChannelFilter) bool {
	if filter.UserId <= 0 {
		return true
	}
	settings := ch.GetOtherSettings()
	if settings.IsUserBlacklisted(filter.UserId) {
		return false
	}
	for _, rule := range settings.UserRules {
		if rule.RPM <= 0 && rule.Matches(filter.UserId, modelName, filter.NonStream) {
			return false
		}
	}
	for index, rule := range settings.UserRules {
		if rule.RPM <= 0 || !rule.Matches(filter.UserId, modelName, filter.NonStream) {
			continue
		}
		count, secondsLeft := channelUserRuleWindowState(ch.Id, index, filter.UserId)
		if count >= rule.RPM {
			filter.RuleLimit.Record(secondsLeft)
			return false
		}
	}
	return true
}

// channelUserRuleWindowState returns how many matching requests the user has
// sent the channel under one rule in the current window, and the seconds left
// in that window.
func channelUserRuleWindowState(channelId int, ruleIndex int, userId int) (int, int) {
	key := channelUserRuleKey(channelId, ruleIndex, userId)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), channelUserRuleRedisTimeout)
		defer cancel()
		values, err := common.RDB.Eval(ctx, channelUserRuleWindowScript, []string{key}).Int64Slice()
		if err != nil || len(values) != 2 {
			// The limit only sheds load, so an unreadable counter lets the
			// request use the channel.
			if err != nil && !errors.Is(err, redis.Nil) {
				common.SysError(fmt.Sprintf("read channel user rule counter %s: %v", key, err))
			}
			return 0, 0
		}
		secondsLeft := int(values[1])
		if secondsLeft < 0 {
			secondsLeft = channelUserRuleWindowSeconds
		}
		return int(values[0]), secondsLeft
	}
	windows := &channelUserRuleWindows
	windows.Lock()
	defer windows.Unlock()
	window := windows.byKey[key]
	elapsed := time.Now().Unix() - window.start
	if elapsed >= channelUserRuleWindowSeconds {
		return 0, 0
	}
	return window.count, int(channelUserRuleWindowSeconds - elapsed)
}

// RecordChannelUserRuleHits counts one request sent to the channel against
// every matching user rule of the channel that has an RPM.
func RecordChannelUserRuleHits(ch *Channel, modelName string, userId int, nonStream bool) {
	if ch == nil || userId <= 0 {
		return
	}
	for index, rule := range ch.GetOtherSettings().UserRules {
		if rule.RPM <= 0 || !rule.Matches(userId, modelName, nonStream) {
			continue
		}
		key := channelUserRuleKey(ch.Id, index, userId)
		if common.RedisEnabled && common.RDB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), channelUserRuleRedisTimeout)
			err := common.RDB.Eval(ctx, channelUserRuleHitScript, []string{key}, channelUserRuleWindowSeconds).Err()
			cancel()
			if err != nil {
				common.SysError(fmt.Sprintf("count channel user rule %s: %v", key, err))
			}
			continue
		}
		windows := &channelUserRuleWindows
		windows.Lock()
		now := time.Now().Unix()
		window := windows.byKey[key]
		if now-window.start >= channelUserRuleWindowSeconds {
			window = channelUserRuleWindow{start: now}
		}
		window.count++
		windows.byKey[key] = window
		if len(windows.byKey) > channelUserRuleWindowsPruneSize {
			for staleKey, staleWindow := range windows.byKey {
				if now-staleWindow.start >= channelUserRuleWindowSeconds {
					delete(windows.byKey, staleKey)
				}
			}
		}
		windows.Unlock()
	}
}
