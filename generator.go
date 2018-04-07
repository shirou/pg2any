package main

type Generator interface {
	GetType() string
	Build(InspectResult) error
}
