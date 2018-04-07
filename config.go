package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"

	_ "github.com/lib/pq"
)

type Config struct {
	Src        string            `json:"src"`
	GenConfigs []json.RawMessage `json:"generators"`
	generators []Generator
	db         *sql.DB
}

type GeneratorConfig struct {
	Generator string `json:"type"`
}

func NewConfig(filename string) (*Config, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var ret Config
	if err := json.Unmarshal(buf, &ret); err != nil {
		return nil, err
	}

	db, err := ret.connect()
	if err != nil {
		return nil, err
	}

	ret.db = db
	ret.generators = make([]Generator, 0)

	for _, gc := range ret.GenConfigs {
		g, err := NewGenerator(db, gc)
		if err != nil {
			return nil, err
		}
		ret.generators = append(ret.generators, g)
	}

	return &ret, nil
}

func NewGenerator(db *sql.DB, config json.RawMessage) (Generator, error) {
	var c GeneratorConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return nil, fmt.Errorf("generator config error: %s", err)
	}

	switch c.Generator {
	case HibernateTypeName:
		return NewHibernate(db, config)
	case SphinxTypeName:
		//		return NewSphinx(db, config)
	default:
		return nil, fmt.Errorf("unknown generator: %s", c.Generator)
	}
	return nil, nil
}

func (c *Config) connect() (*sql.DB, error) {
	db, err := sql.Open("postgres", c.Src)
	if err != nil {
		return nil, err
	}

	return db, nil
}
