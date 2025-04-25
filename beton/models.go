package main

import "github.com/golang-jwt/jwt/v5"

type Measurement struct {
	Time      string  `json:"time"`
	Parameter string  `json:"parameter"`
	Value     float64 `json:"value"`
}

type Parameter struct {
	Name  string  `json:"parameter_name"`
	Min   float64 `json:"min_threshold"`
	Max   float64 `json:"max_threshold"`
	Units string  `json:"units"`
}
type User struct {
	ID     int    `json:"id_user"`
	Login  string `json:"login"`
	Rights string `json:"rights"`
}
type Claims struct {
	Login  string `json:"login"`
	Rights string `json:"rights"`
	jwt.RegisteredClaims
}
