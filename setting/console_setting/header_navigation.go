package console_setting

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

// ValidateHeaderNavModules keeps sponsor links safe while accepting existing navigation options.
func ValidateHeaderNavModules(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var config struct {
		Sponsors []struct {
			Enabled bool   `json:"enabled"`
			Name    string `json:"name"`
			URL     string `json:"url"`
		} `json:"sponsors"`
	}
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return fmt.Errorf("顶部导航配置无效: %w", err)
	}
	if len(config.Sponsors) > 3 {
		return fmt.Errorf("最多配置 3 个导航链接")
	}
	for index, site := range config.Sponsors {
		name := strings.TrimSpace(site.Name)
		if utf8.RuneCountInString(name) > 80 {
			return fmt.Errorf("导航链接 %d 的标题最多 80 个字符", index+1)
		}
		if site.Enabled && (name == "" || strings.TrimSpace(site.URL) == "") {
			return fmt.Errorf("启用导航链接 %d 前请填写标题和地址", index+1)
		}
		address := strings.TrimSpace(site.URL)
		if address == "" {
			continue
		}
		parsed, err := url.Parse(address)
		if utf8.RuneCountInString(address) > 2048 || err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("导航链接 %d 必须是有效的 HTTP(S) URL（最多 2048 个字符）", index+1)
		}
	}
	return nil
}
