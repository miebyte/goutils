package buildinfo

import "github.com/miebyte/goutils/internal/share"

func GetHostName() string {
	return share.HostName
}

func GetPodNamespace() string {
	return share.PodNamespace
}

func GetPodIp() string {
	return share.PodIp
}

func GetProjectName() string {
	return share.ProjectName
}

func GetServiceName() string {
	return share.ServiceName()
}
