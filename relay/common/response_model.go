package common

import (
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
