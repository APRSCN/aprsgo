package model

import "github.com/APRSCN/aprsgo/internal/historydb"

// ReturnHistory provides a struct to return history of server
type ReturnHistory struct {
	Msg    string                `json:"msg"`
	System []historydb.DataPoint `json:"system"`
}
