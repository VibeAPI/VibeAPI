package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/requestrule"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func GetRequestPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"options":              model.CurrentRequestPolicy().Options,
		"request_rule_presets": requestrule.Presets(),
	}})
}

func UpdateRequestPolicy(c *gin.Context) {
	var request struct {
		Options map[string]string `json:"options"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.UpdateRequestPolicyOptions(request.Options); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	GetRequestPolicy(c)
}

// TestRequestRules checks a sample request against request rules without
// relaying it. Rules defaults to the saved rules, so an editor can test
// unsaved changes; the response is what a client of that endpoint receives.
func TestRequestRules(c *gin.Context) {
	var request struct {
		Rules       *string           `json:"rules"`
		Endpoint    string            `json:"endpoint"`
		Model       string            `json:"model"`
		PassThrough bool              `json:"pass_through"`
		Body        string            `json:"body"`
		Headers     map[string]string `json:"headers"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !requestrule.IsEndpoint(request.Endpoint) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unknown endpoint: " + request.Endpoint})
		return
	}
	if !gjson.Valid(request.Body) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "the request body is not valid JSON"})
		return
	}
	rulesJSON := operation_setting.GetRequestRuleSetting().Rules
	if request.Rules != nil {
		rulesJSON = *request.Rules
	}
	rules, err := requestrule.Parse(rulesJSON)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	header := http.Header{}
	for name, value := range request.Headers {
		header.Set(name, value)
	}
	paths := map[string]string{
		requestrule.EndpointClaudeMessages:  "/v1/messages",
		requestrule.EndpointOpenAIChat:      "/v1/chat/completions",
		requestrule.EndpointOpenAIResponses: "/v1/responses",
		requestrule.EndpointGemini:          "/v1beta/models/" + request.Model + ":generateContent",
	}
	match, traces := rules.Explain(requestrule.Input{
		Endpoint:    request.Endpoint,
		Model:       request.Model,
		Path:        paths[request.Endpoint],
		PassThrough: request.PassThrough,
		Body:        []byte(request.Body),
		Header:      header,
	})
	data := gin.H{"matched": match != nil, "trace": traces}
	if match != nil {
		// Like the relay: the same error, the request id appended, and the
		// response shape of the endpoint.
		apiErr := match.APIError()
		apiErr.SetMessage(common.MessageWithRequestId(apiErr.Error(), c.GetString(common.RequestIdKey)))
		body := gin.H{"error": apiErr.ToOpenAIError()}
		if request.Endpoint == requestrule.EndpointClaudeMessages {
			body = gin.H{"type": "error", "error": apiErr.ToClaudeError()}
		}
		data["rule"] = gin.H{"position": match.Index + 1, "id": match.Rule.ID, "name": match.Rule.Name}
		data["field"] = match.Field
		data["value"] = match.Value
		data["status"] = match.Rule.Response.Status
		data["response"] = body
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
