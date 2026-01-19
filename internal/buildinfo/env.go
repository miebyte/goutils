package buildinfo

import "github.com/miebyte/goutils/internal/share"

func GetHostName() string {
	return share.HostName
}

func GetServiceName() string {
	return share.ServiceName()
}
