// File:		mysql.go
// Created by:	Hoven
// Created on:	2025-04-07
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package mysqlutils

import (
	"fmt"
	"time"

	"github.com/miebyte/goutils/discover"
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

type MysqlConfig struct {
	Instance string `json:"instance"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Charset  string `json:"charset"`
	PoolSize int    `json:"pool_size"`
}

func (c *MysqlConfig) Address() string {
	return discover.GetServiceFinder().GetAddress(c.Instance)
}

func (c *MysqlConfig) generateDSN() string {
	if c.Charset == "" {
		c.Charset = "utf8mb4"
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?charset=%s&parseTime=True&loc=Local",
		c.Username,
		c.Password,
		c.Address(),
		c.Database,
		c.Charset,
	)
	return dsn
}

func (conf *MysqlConfig) DialMysqlGorm() (*gorm.DB, error) {
	gormConf := &gorm.Config{
		PrepareStmt: true,
		Logger:      GormLogger,
	}

	db, err := gorm.Open(
		mysql.Open(conf.generateDSN()),
		gormConf,
	)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "getRawDb")
	}

	sqlDB.SetMaxIdleConns(conf.PoolSize)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func DialMysqlGorm(conf *MysqlConfig) (*gorm.DB, error) {
	return conf.DialMysqlGorm()
}

type MysqlPool map[string]*gorm.DB

type MysqlConfigMap map[string]*MysqlConfig

func (mc MysqlConfigMap) DialMysqlGorm(names ...string) (*gorm.DB, error) {
	name := "default"
	if len(names) != 0 {
		name = names[0]
	}

	conf, exists := mc[name]
	if !exists {
		return nil, fmt.Errorf("mysql(%s) config not exists", name)
	}

	return conf.DialMysqlGorm()
}

func (mc MysqlConfigMap) DialGormPool() (MysqlPool, error) {
	mp := make(MysqlPool)

	for name, conf := range mc {
		db, err := conf.DialMysqlGorm()
		if err != nil {
			return nil, errors.Wrapf(err, "dial mysqlPool of %s", name)
		}

		mp[name] = db
	}

	return mp, nil
}

func (mp MysqlPool) GetMysql(names ...string) (*gorm.DB, error) {
	name := "default"
	if len(names) != 0 {
		name = names[0]
	}

	db, exists := mp[name]
	if !exists {
		return nil, errors.Errorf("mysql(%s) not init", name)
	}
	return db, nil
}

func (mp MysqlPool) GetDBWithResolver(routerConfig *DBRouterConfig) (*gorm.DB, error) {
	if routerConfig == nil {
		return nil, errors.New("routerConfig is nil")
	}

	defaultDb, err := mp.GetMysql(routerConfig.DefaultDB)
	if err != nil {
		return nil, err
	}

	dbResolver := &dbresolver.DBResolver{}

	for appName, resolverConfig := range routerConfig.Resolvers {
		config := dbresolver.Config{}

		if len(resolverConfig.Replicas) > 0 {
			var replicas []gorm.Dialector
			for _, replicaName := range resolverConfig.Replicas {
				replicaDb, exists := mp[replicaName]
				if !exists {
					return nil, fmt.Errorf("replica mysql(%s) connection not exists", replicaName)
				}
				replicas = append(replicas, replicaDb.Dialector)
			}
			config.Replicas = replicas
		}

		if len(resolverConfig.Sources) > 0 {
			var sources []gorm.Dialector
			for _, sourceName := range resolverConfig.Sources {
				sourceDb, exists := mp[sourceName]
				if !exists {
					return nil, fmt.Errorf("source mysql(%s) connection not exists", sourceName)
				}
				sources = append(sources, sourceDb.Dialector)
			}
			config.Sources = sources
		}

		config.TraceResolverMode = true
		tables := resolverConfig.ResolversTables()
		resolvers := append(tables, appName)
		dbResolver = dbResolver.Register(config, resolvers...)
	}

	if err := defaultDb.Use(dbResolver); err != nil {
		return nil, errors.Wrapf(err, "register dbresolver")
	}

	return defaultDb, nil
}

func (mp MysqlPool) Close() {
}
