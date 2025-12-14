package main

type Account struct {
	Id     string  `json:"id"`
	Name   string  `json:"name"`
	Orders []Order `json:"orders"`
}
