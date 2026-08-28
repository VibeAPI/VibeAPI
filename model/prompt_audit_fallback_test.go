package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestPromptAuditFallbackExcludesProtectedAndHigherPriorityChannels(t *testing.T) {
	priorityTen := int64(10)
	priorityFive := int64(5)
	protected := map[int]struct{}{10: {}}

	tests := []struct {
		name    string
		channel *Channel
		allowed bool
	}{
		{name: "protected same priority", channel: &Channel{Id: 10, Priority: &priorityTen}, allowed: false},
		{name: "unprotected same priority", channel: &Channel{Id: 11, Priority: &priorityTen}, allowed: true},
		{name: "unprotected lower priority", channel: &Channel{Id: 12, Priority: &priorityFive}, allowed: true},
		{name: "unprotected higher priority", channel: &Channel{Id: 13, Priority: common.GetPointer(int64(11))}, allowed: false},
		{name: "nil channel", channel: nil, allowed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.allowed, IsPromptAuditFallbackChannel(test.channel, 10, protected))
		})
	}
}
