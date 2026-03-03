// Package vote implements the vote-service: Redis-backed vote state management,
// gRPC service, Kafka producer, and karma consumer.
package vote

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// VoteStore manages vote state in Redis using Lua scripts for atomicity.
// Redis DB 5 is reserved for the vote-service.
type VoteStore struct {
	rdb    *redis.Client
	logger *zap.Logger
	// castVoteScript is preloaded for efficient execution.
	castVoteScript *redis.Script
}

// NewVoteStore creates a new vote store backed by the given Redis client.
func NewVoteStore(rdb *redis.Client, logger *zap.Logger) *VoteStore {
	s := &VoteStore{
		rdb:    rdb,
		logger: logger,
	}
	s.castVoteScript = redis.NewScript(castVoteLua)
	return s
}

// castVoteLua is an atomic Lua script that handles all 9 vote state transitions.
//
// KEYS[1] = votes:state:{user_id}:{target_id}   (STRING: "up"/"down")
// KEYS[2] = votes:up:{target_id}                 (SET of user_ids)
// KEYS[3] = votes:down:{target_id}               (SET of user_ids)
// KEYS[4] = votes:score:{target_id}              (STRING: integer)
//
// ARGV[1] = user_id
// ARGV[2] = new_direction ("up", "down", "none")
//
// Returns: {delta, new_score, old_direction}
//   - delta=0 means idempotent no-op (same direction or none→none)
//   - old_direction is "" if no previous vote
const castVoteLua = `
local state_key = KEYS[1]
local up_key    = KEYS[2]
local down_key  = KEYS[3]
local score_key = KEYS[4]
local user_id   = ARGV[1]
local new_dir   = ARGV[2]

-- Get current vote state
local cur = redis.call("GET", state_key)
if cur == false then cur = "" end

-- Idempotent: same direction = no-op
if cur == new_dir then
    local sc = redis.call("GET", score_key)
    if sc == false then sc = "0" end
    return {0, tonumber(sc), cur}
end

-- none → none is also a no-op
if cur == "" and new_dir == "none" then
    local sc = redis.call("GET", score_key)
    if sc == false then sc = "0" end
    return {0, tonumber(sc), ""}
end

-- Compute delta based on transition
local delta = 0
if cur == "" and new_dir == "up" then
    delta = 1
elseif cur == "" and new_dir == "down" then
    delta = -1
elseif cur == "up" and new_dir == "down" then
    delta = -2
elseif cur == "down" and new_dir == "up" then
    delta = 2
elseif cur == "up" and new_dir == "none" then
    delta = -1
elseif cur == "down" and new_dir == "none" then
    delta = 1
end

-- Remove from old set
if cur == "up" then
    redis.call("SREM", up_key, user_id)
elseif cur == "down" then
    redis.call("SREM", down_key, user_id)
end

-- Add to new set and update state
if new_dir == "up" then
    redis.call("SADD", up_key, user_id)
    redis.call("SET", state_key, "up")
elseif new_dir == "down" then
    redis.call("SADD", down_key, user_id)
    redis.call("SET", state_key, "down")
elseif new_dir == "none" then
    redis.call("DEL", state_key)
end

-- Update score
local new_score = redis.call("INCRBY", score_key, delta)

return {delta, new_score, cur}
`

// CastVote atomically records a vote and returns the delta, new score, and old direction.
// direction must be "up", "down", or "none".
// Returns delta=0 if the vote was idempotent (same direction requested).
func (s *VoteStore) CastVote(ctx context.Context, userID, targetID, direction string) (delta int, newScore int, oldDirection string, err error) {
	stateKey := fmt.Sprintf("votes:state:%s:%s", userID, targetID)
	upKey := fmt.Sprintf("votes:up:%s", targetID)
	downKey := fmt.Sprintf("votes:down:%s", targetID)
	scoreKey := fmt.Sprintf("votes:score:%s", targetID)

	result, err := s.castVoteScript.Run(ctx, s.rdb,
		[]string{stateKey, upKey, downKey, scoreKey},
		userID, direction,
	).Slice()
	if err != nil {
		return 0, 0, "", fmt.Errorf("cast vote lua: %w", err)
	}

	if len(result) != 3 {
		return 0, 0, "", fmt.Errorf("unexpected lua result length: %d", len(result))
	}

	d, err := toInt(result[0])
	if err != nil {
		return 0, 0, "", fmt.Errorf("parse delta: %w", err)
	}
	ns, err := toInt(result[1])
	if err != nil {
		return 0, 0, "", fmt.Errorf("parse new_score: %w", err)
	}
	old := ""
	if s, ok := result[2].(string); ok {
		old = s
	}

	return d, ns, old, nil
}

// GetVoteState returns the user's current vote direction on a target.
// Returns "up", "down", or "" (no vote).
func (s *VoteStore) GetVoteState(ctx context.Context, userID, targetID string) (string, error) {
	key := fmt.Sprintf("votes:state:%s:%s", userID, targetID)
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get vote state: %w", err)
	}
	return val, nil
}

// GetScore returns the current net score for a target. Returns 0 if not set.
func (s *VoteStore) GetScore(ctx context.Context, targetID string) (int, error) {
	key := fmt.Sprintf("votes:score:%s", targetID)
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get score: %w", err)
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse score %q: %w", val, err)
	}
	return n, nil
}

// BatchGetVoteStates returns the user's vote direction for multiple targets.
// Returns a map of targetID -> direction ("up", "down", or "" for no vote).
// Uses Redis pipelining for efficiency.
func (s *VoteStore) BatchGetVoteStates(ctx context.Context, userID string, targetIDs []string) (map[string]string, error) {
	if len(targetIDs) == 0 {
		return make(map[string]string), nil
	}

	pipe := s.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(targetIDs))
	for i, tid := range targetIDs {
		key := fmt.Sprintf("votes:state:%s:%s", userID, tid)
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	// Ignore redis.Nil errors — some keys may not exist
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("batch get vote states: %w", err)
	}

	result := make(map[string]string, len(targetIDs))
	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err == redis.Nil {
			result[targetIDs[i]] = ""
		} else if err != nil {
			result[targetIDs[i]] = ""
		} else {
			result[targetIDs[i]] = val
		}
	}
	return result, nil
}

// toInt converts a Lua result value to an int.
// Lua returns numbers as int64 via go-redis.
func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int64:
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("unexpected type %T for int conversion", v)
	}
}
