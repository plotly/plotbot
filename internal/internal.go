package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type InternalAPI struct {
	Config InternalAPIConfig
}

type InternalAPIConfig map[string]*InternalAPIEnvironment

type InternalAPIEnvironment struct {
	BaseURL string `json:"base_url"`
	AuthKey string `json:"auth_key"`
}

type PlolyCurrentHeadResponse struct {
	CurrentHead string `json:"current_head"`
}

func New(config InternalAPIConfig) *InternalAPI {
	return &InternalAPI{
		Config: config,
	}
}

func (i *InternalAPI) GetCurrentHead(env string) string {
	conf := i.Config[env]
	if conf == nil || conf.BaseURL == "" || conf.AuthKey == "" {
		return ""
	}

	req, err := http.NewRequest(
		"GET", fmt.Sprintf("%s%s", conf.BaseURL, "/current_head"), nil,
	)
	if err != nil {
		return ""
	}
	req.Header.Set("X-Internal-Key", conf.AuthKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}

	defer resp.Body.Close()
	var result = PlolyCurrentHeadResponse{}
	err = json.NewDecoder(resp.Body).Decode(&result)

	return result.CurrentHead
}
