package sora

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateBillingUsesConfiguredFixedPriceUnit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		priceUnit  string
		duration   int
		wantSecond bool
		wantValue  float64
	}{
		{
			name:      "per request without duration stays a single charge",
			priceUnit: model.PriceUnitRequest,
		},
		{
			name:       "per second without duration reserves four seconds",
			priceUnit:  model.PriceUnitSecond,
			wantSecond: true,
			wantValue:  4,
		},
		{
			name:       "per second uses explicit duration",
			priceUnit:  model.PriceUnitSecond,
			duration:   8,
			wantSecond: true,
			wantValue:  8,
		},
		{
			name:      "per request ignores explicit duration for billing",
			priceUnit: model.PriceUnitRequest,
			duration:  8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Set("task_request", relaycommon.TaskSubmitReq{
				Duration: tt.duration,
				Size:     "720x1280",
			})
			info := &relaycommon.RelayInfo{
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
				PriceData: types.PriceData{
					UsePrice:  true,
					PriceUnit: tt.priceUnit,
				},
			}

			ratios := (&TaskAdaptor{}).EstimateBilling(ctx, info)

			require.Equal(t, 1.0, ratios["size"])
			seconds, exists := ratios["seconds"]
			assert.Equal(t, tt.wantSecond, exists)
			if tt.wantSecond {
				assert.Equal(t, tt.wantValue, seconds)
			}
		})
	}
}

func TestEstimateBillingKeepsLegacyRatioDurationBehavior(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Duration: 6})

	ratios := (&TaskAdaptor{}).EstimateBilling(ctx, &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	})

	assert.Equal(t, 6.0, ratios["seconds"])
}
