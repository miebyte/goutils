// File:		dbrouter.go
// Created by:	Hoven
// Created on:	2025-05-28
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package mysqlutils

type DBResolverConfig struct {
	Sources  []string `json:"sources"`
	Replicas []string `json:"replicas"`
	Tables   []Table  `json:"tables"`
}

type DBRouterConfig struct {
	DefaultDB string                       `json:"default_db"`
	Resolvers map[string]*DBResolverConfig `json:"resolvers"`
}

func NewDBRouterConfig(defaultDB string) *DBRouterConfig {
	return &DBRouterConfig{
		DefaultDB: defaultDB,
		Resolvers: make(map[string]*DBResolverConfig),
	}
}

// AddResolver 添加一个解析器配置
func (config *DBRouterConfig) AddResolver(name string, resolver *DBResolverConfig) *DBRouterConfig {
	config.Resolvers[name] = resolver
	return config
}

// NewDBResolverConfig 创建一个新的解析器配置
func NewDBResolverConfig() *DBResolverConfig {
	return &DBResolverConfig{
		Sources:  make([]string, 0),
		Replicas: make([]string, 0),
		Tables:   make([]Table, 0),
	}
}

// WithSources 设置写库连接
func (config *DBResolverConfig) WithSources(sources ...string) *DBResolverConfig {
	config.Sources = append(config.Sources, sources...)
	return config
}

// WithReplicas 设置读库连接
func (config *DBResolverConfig) WithReplicas(replicas ...string) *DBResolverConfig {
	config.Replicas = append(config.Replicas, replicas...)
	return config
}

// WithTables 设置表接口
func (config *DBResolverConfig) WithTables(tables ...Table) *DBResolverConfig {
	config.Tables = append(config.Tables, tables...)
	return config
}

func (config *DBResolverConfig) ResolversTables() []any {
	var tables []any
	for _, table := range config.Tables {
		tables = append(tables, table.TableName())
	}

	return tables
}
