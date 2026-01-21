package provider

import (
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"

	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/internal/share"
)

type ConsulProvider struct {
	serviceName string
	tag         string
}

func NewConsulProvider(serviceName, tag string) *ConsulProvider {
	return &ConsulProvider{serviceName: serviceName, tag: tag}
}

func (p *ConsulProvider) getServerKey(name string) string {
	return fmt.Sprintf("/etc/configs/%v", name)
}

func (p *ConsulProvider) listPossibleTags(tag string) []string {
	v, err := semver.NewVersion(tag)
	if err != nil {
		return []string{tag}
	}

	majorV := v.Major()
	minorV := v.Minor()
	patchV := v.Patch()

	possibleTags := []string{tag}
	for i := int(patchV - 1); i >= 0; i-- {
		possibleTags = append(possibleTags, fmt.Sprintf("v%d.%d.%d", majorV, minorV, i))
	}

	possibleTags = append(possibleTags, fmt.Sprintf("v%d.%d", majorV, minorV))

	return possibleTags
}

func (p *ConsulProvider) getRemotePossiblePath(name, tag string) string {
	var possiblePath string
	defer func() {
		innerlog.Logger.Infof("Reading consul config from possiblePath(%s)", possiblePath)
	}()

	kv := consulutils.GetConsulClient().KV()
	if tag == "dev" {
		path := fmt.Sprintf("%s/%s.json", p.getServerKey(name), tag)
		pair, _, err := kv.Get(path, nil)
		if err == nil && pair != nil {
			possiblePath = path
			return possiblePath
		}
	}

	for _, t := range p.listPossibleTags(tag) {
		path := fmt.Sprintf("%s/%s.json", p.getServerKey(name), t)

		pair, _, err := kv.Get(path, nil)
		if err == nil && pair != nil {
			possiblePath = path
			break
		}
	}

	return possiblePath
}

func (p *ConsulProvider) ReadConfig() (map[string]any, error) {
	innerlog.Logger.Infof("Reading consul config from Service(%s) Tag(%s)", p.serviceName, p.tag)

	path := p.getRemotePossiblePath(p.serviceName, p.tag)
	if path == "" {
		return nil, fmt.Errorf("no config found")
	}

	kv := consulutils.GetConsulClient().KV()
	pair, _, err := kv.Get(path, nil)
	if err != nil {
		return nil, err
	}

	if pair == nil {
		return nil, fmt.Errorf("no config found")
	}

	temp := make(map[string]any)
	if err := json.Unmarshal(pair.Value, &temp); err != nil {
		return nil, err
	}

	return temp, nil
}

func (p *ConsulProvider) WatchConfig() <-chan Event {
	ch := make(chan Event)

	key := p.getRemotePossiblePath(p.serviceName, p.tag)
	plan, err := watch.Parse(map[string]any{"type": "key", "key": key})
	innerlog.Logger.PanicError(err)

	first := true
	var currentVal []byte

	plan.Handler = func(index uint64, data any) {
		kv, ok := (data).(*api.KVPair)
		if !ok {
			innerlog.Logger.Errorf("Failed to watch remote config data.")
			return
		}
		// There is always a trigger at first launch, ignore it.
		if first {
			first = false
			currentVal = kv.Value
			return
		}
		if string(kv.Value) == string(currentVal) {
			innerlog.Logger.Warnf("Remote index changed, but value was not changed")
			return
		}
		innerlog.Logger.Debugf("Remote config changed")

		temp := make(map[string]any)
		if err := json.Unmarshal(kv.Value, &temp); err != nil {
			innerlog.Logger.Errorf("Failed to unmarshal remote config data. err=%v", err)
			return
		}

		ch <- Event{
			Key:    key,
			Path:   key,
			Config: temp,
			Err:    nil,
		}

		currentVal = kv.Value

	}

	go innerlog.Logger.PanicError(plan.Run(share.ConsulAddr()))

	innerlog.Logger.Debugf("Start watch consul config(%s)", key)

	return ch
}
