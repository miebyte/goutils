package config

import "fmt"

type NacosConfig struct {
	Env NacosEnv `json:"env"`
}

func (c *NacosConfig) Validate() error {
	if c.Env.Nacos.Host == "" {
		return fmt.Errorf("supernacos.json 缺少必需的字段: 'env.nacos.host'")
	}
	if c.Env.Nacos.Port == 0 {
		return fmt.Errorf("supernacos.json 缺少必需的字段: 'env.nacos.port'")
	}
	if c.Env.Group == "" {
		return fmt.Errorf("supernacos.json 缺少必需的字段: 'env.group'")
	}

	if c.Env.Nacos.AuthData.Username == "" {
		return fmt.Errorf("supernacos.json 缺少必需的字段: 'env.nacos.auth_data.username'")
	}

	return nil
}

type NacosEnv struct {
	Name        string  `json:"name"`
	Nacos       Nacos   `json:"nacos"`
	Group       string  `json:"group"`
	NamespaceID *string `json:"namespace_id,omitempty"`
}

func (e *NacosEnv) GetNamespaceID() string {
	if e.NamespaceID != nil && *e.NamespaceID != "" {
		return *e.NamespaceID
	}
	return e.Name
}

type Nacos struct {
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	AuthData NacosAuthData `json:"auth_data"`
}

type NacosAuthData struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
