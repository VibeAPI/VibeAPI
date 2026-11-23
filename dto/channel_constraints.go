package dto

type ChannelPinSource string

const (
	PinSourceToken      ChannelPinSource = "token"       // Rank 0, highest
	PinSourceOriginTask ChannelPinSource = "origin_task" // Rank 10
)

const (
	PinRankToken      = 0
	PinRankOriginTask = 10
)

type PinRetryMode int

const (
	PinRetrySameChannel PinRetryMode = iota
	PinRetrySingleAttempt
)

func (m PinRetryMode) Stricter(other PinRetryMode) PinRetryMode {
	if m == PinRetrySingleAttempt || other == PinRetrySingleAttempt {
		return PinRetrySingleAttempt
	}
	return PinRetrySameChannel
}

type ChannelPin struct {
	ChannelId int
	Source    ChannelPinSource
	Rank      int
	RetryMode PinRetryMode
}

type ChannelFilterKind string

const (
	FilterRequestPath        ChannelFilterKind = "request_path"
	FilterUserBlacklist      ChannelFilterKind = "user_blacklist"
	FilterTaskPluginIdentity ChannelFilterKind = "task_plugin_identity"
	FilterResponsesWebSocket ChannelFilterKind = "responses_websocket"
)

type ChannelFilter struct {
	Kind        ChannelFilterKind
	RequestPath string
	UserId      int
	// NonStream marks a user_blacklist filter for a request that did not ask
	// for streaming, which channel user rules can match.
	NonStream bool
	// RuleLimit, when set on a user_blacklist filter, collects the channels a
	// user rule's RPM kept the request off.
	RuleLimit              *UserRuleLimit
	TaskPluginKey          string
	TaskPluginKeys         []string
	TaskPluginChannelTypes []int
}

// UserRuleLimit records, for one request, that a channel user rule's RPM kept
// the request off at least one channel. When no channel is left, the request
// can be answered with 429 and retried after RetryAfterSeconds, the time left
// in the earliest of those windows.
type UserRuleLimit struct {
	Limited           bool
	RetryAfterSeconds int
}

func (l *UserRuleLimit) Record(retryAfterSeconds int) {
	if l == nil {
		return
	}
	retryAfterSeconds = max(retryAfterSeconds, 1)
	if !l.Limited || retryAfterSeconds < l.RetryAfterSeconds {
		l.RetryAfterSeconds = retryAfterSeconds
	}
	l.Limited = true
}

type ChannelConstraints struct {
	Pins    []ChannelPin
	Filters []ChannelFilter
}

func (cc *ChannelConstraints) AddPin(p ChannelPin) {
	if cc == nil {
		return
	}
	cc.Pins = append(cc.Pins, p)
}

func (cc *ChannelConstraints) AddFilter(f ChannelFilter) {
	if cc == nil {
		return
	}
	cc.Filters = append(cc.Filters, f)
}

// UserRuleLimit returns the rule limit report of the request's user_blacklist
// filter, or nil when the request has none.
func (cc *ChannelConstraints) UserRuleLimit() *UserRuleLimit {
	if cc == nil {
		return nil
	}
	for _, filter := range cc.Filters {
		if filter.Kind == FilterUserBlacklist && filter.RuleLimit != nil {
			return filter.RuleLimit
		}
	}
	return nil
}

// ResolvedPin returns the winning pin after priority resolution.
// Lowest Rank wins. Pins that name the same channel are merged (stricter RetryMode).
// overridden lists pins that lost to a different channel (for warn logging).
func (cc *ChannelConstraints) ResolvedPin() (ChannelPin, bool, []ChannelPin) {
	if cc == nil || len(cc.Pins) == 0 {
		return ChannelPin{}, false, nil
	}

	merged := make(map[int]ChannelPin, len(cc.Pins))
	order := make([]int, 0, len(cc.Pins))
	for _, pin := range cc.Pins {
		existing, seen := merged[pin.ChannelId]
		if !seen {
			merged[pin.ChannelId] = pin
			order = append(order, pin.ChannelId)
			continue
		}
		existing.RetryMode = existing.RetryMode.Stricter(pin.RetryMode)
		if pin.Rank < existing.Rank {
			existing.Rank = pin.Rank
			existing.Source = pin.Source
		}
		merged[pin.ChannelId] = existing
	}

	winner := merged[order[0]]
	for _, channelID := range order[1:] {
		candidate := merged[channelID]
		if candidate.Rank < winner.Rank {
			winner = candidate
		}
	}

	var overridden []ChannelPin
	for _, channelID := range order {
		candidate := merged[channelID]
		if candidate.ChannelId != winner.ChannelId {
			overridden = append(overridden, candidate)
		}
	}
	return winner, true, overridden
}

func (cc *ChannelConstraints) SuppressesRetry() bool {
	pin, found, _ := cc.ResolvedPin()
	return found && pin.RetryMode == PinRetrySingleAttempt
}
