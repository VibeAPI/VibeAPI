package common

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ResponseModelName returns the model name that should be exposed to the client.
// Billing and upstream requests must continue using UpstreamModelName.
func (info *RelayInfo) ResponseModelName() string {
	if info == nil {
		return ""
	}
	return info.ExposeResponseModelName(info.UpstreamModelName)
}

// ResponseModelNameOr falls back to a generated/provider model name when relay
// metadata does not carry an upstream model.
func (info *RelayInfo) ResponseModelNameOr(modelName string) string {
	if info == nil || info.UpstreamModelName == "" {
		return info.ExposeResponseModelName(modelName)
	}
	return info.ResponseModelName()
}

// ExposeResponseModelName preserves the provider response model by default and
// only replaces it when the channel explicitly enables alias exposure.
func (info *RelayInfo) ExposeResponseModelName(modelName string) string {
	if info != nil && info.ChannelMeta != nil && info.ChannelSetting.ReturnRequestModelName && info.OriginModelName != "" {
		return info.OriginModelName
	}
	return modelName
}

// RewriteResponseModel rewrites existing model fields without adding fields that
// were absent from the upstream payload. This preserves provider-specific fields
// while preventing mapped upstream model names from leaking to clients.
func RewriteResponseModel(data []byte, info *RelayInfo, paths ...string) ([]byte, error) {
	if info == nil || info.ChannelMeta == nil || !info.ChannelSetting.ReturnRequestModelName || info.OriginModelName == "" {
		return data, nil
	}

	result := data
	for _, path := range paths {
		if path == "" || !gjson.GetBytes(result, path).Exists() {
			continue
		}
		var err error
		result, err = sjson.SetBytes(result, path, info.OriginModelName)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// ResponseModel records upstream declarations before response conversion. It is
// diagnostic only: it must never change routing, pricing, or downstream output.
// Only the three names are stored; whether they disagree is computed on demand
// so every consumer applies the current comparison rule to old rows as well.
type ResponseModel struct {
	RequestedModel string `json:"requested_model"`
	UpstreamModel  string `json:"upstream_model"`
	ReturnedModel  string `json:"returned_model"`
}

// matches reports whether an upstream declaration is compatible with the
// requested or upstream model: equal ignoring case, a dated or variant name
// that extends it, or the same name behind a provider path such as
// "deepseek/deepseek-v4.1-flash".
func (r *ResponseModel) matches(model string) bool {
	returned := strings.ToLower(model)
	for _, expected := range []string{r.RequestedModel, r.UpstreamModel} {
		expected = strings.ToLower(expected)
		if expected != "" && (strings.HasPrefix(returned, expected) || strings.HasSuffix(returned, expected)) {
			return true
		}
	}
	return false
}

// Mismatch reports whether the retained upstream declaration disagrees with
// both the requested and upstream models.
func (r *ResponseModel) Mismatch() bool {
	return r != nil && r.ReturnedModel != "" && !r.matches(r.ReturnedModel)
}

// ObserveResponseModel retains the first differing model for inspection, with
// mismatches taking priority over provider-path, prefix, or case-only
// differences. A later matching or empty event cannot erase it. Only observe
// upstream declarations, never models synthesized by a response converter.
func (info *RelayInfo) ObserveResponseModel(model string) {
	if info == nil || strings.TrimSpace(model) == "" {
		return
	}
	if info.ResponseModel == nil {
		info.ResponseModel = &ResponseModel{
			RequestedModel: info.OriginModelName,
			UpstreamModel:  info.GetUpstreamModelName(),
		}
	}
	observation := info.ResponseModel
	if observation.Mismatch() {
		return
	}
	if observation.matches(model) && observation.ReturnedModel != "" &&
		observation.ReturnedModel != observation.RequestedModel && observation.ReturnedModel != observation.UpstreamModel {
		return
	}
	observation.ReturnedModel = model
}
