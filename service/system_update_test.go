package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/pkg/systemupdate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type systemUpdateRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn systemUpdateRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestUpdaterClientDoesNotExposeTransportDetails(t *testing.T) {
	client := &systemUpdateUpdaterClient{token: "secret", socketPath: "/private/updater.sock", client: &http.Client{Transport: systemUpdateRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, errors.New("dial unix /private/updater.sock: permission denied")
	})}}

	_, err := client.capabilities(t.Context())
	require.Error(t, err)
	assert.Equal(t, "system updater is unavailable", err.Error())
	assert.NotContains(t, err.Error(), "private")
}

func TestStartOperationRejectsIdentifiersBeforeUpdaterCall(t *testing.T) {
	service := &SystemUpdateService{currentVersion: "v1.0.0", updater: &systemUpdateUpdaterClient{}}
	_, err := service.StartOperation(t.Context(), systemupdate.StartOperationRequest{ReleaseID: "../latest", ExpectedDigest: "latest", IdempotencyKey: "short"})
	require.Error(t, err)
	assert.Equal(t, "invalid idempotency_key", err.Error())
}
