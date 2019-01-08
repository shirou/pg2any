package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
)

type Config struct {
	Src        string            `json:"src"`
	GenConfigs []json.RawMessage `json:"generators"`
	generators []Generator
	db         *sql.DB
	root       string
}

type GeneratorConfig struct {
	Generator string `json:"type"`
}

func NewConfig(filename string) (*Config, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "config read file")
	}
	var ret Config
	if err := json.Unmarshal(buf, &ret); err != nil {
		return nil, errors.Wrap(err, "json unmarshal")
	}

	db, err := ret.connect()
	if err != nil {
		return nil, errors.Wrap(err, "db connect")
	}

	ret.db = db
	ret.generators = make([]Generator, 0)

	root, err := filepath.Abs(filepath.Dir(filename))
	if err != nil {
		return nil, errors.Wrap(err, "config abs path")
	}

	for _, gc := range ret.GenConfigs {
		g, err := NewGenerator(db, root, gc)
		if err != nil {
			return nil, errors.Wrap(err, "NewGenerator")
		}
		ret.generators = append(ret.generators, g)
	}

	return &ret, nil
}

func NewGenerator(db *sql.DB, root string, config json.RawMessage) (Generator, error) {
	var c GeneratorConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return nil, fmt.Errorf("generator config error: %s", err)
	}

	switch c.Generator {
	case HibernateTypeName:
		return NewHibernate(db, root, config)
	case ProtoBufTypeName:
		return NewProtoBuf(db, root, config)
	case SphinxTypeName:
		return NewSphinx(db, root, config)
	case FakedataTypeName:
		return NewFakedata(db, root, config)
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
