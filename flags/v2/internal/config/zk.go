package config

import "fmt"

type ZkAuthData struct {
	Scheme string `json:"scheme"`
	Auth   string `json:"auth"`
}

type ZooKeeper struct {
	Host     string     `json:"host"`
	Port     int        `json:"port"`
	AuthData ZkAuthData `json:"auth_data"`
}

func (z ZooKeeper) Endpoint() string {
	return fmt.Sprintf("%s:%d", z.Host, z.Port)
}

type ZkEnv struct {
	Name      string    `json:"name"`
	Zookeeper ZooKeeper `json:"zookeeper"`
	Deploy    string    `json:"deploy"`
	Group     string    `json:"group"`
}

type ZkConfig struct {
	Env ZkEnv `json:"env"`
}
