package data

import (
	"sync"
	"time"
)

type Updates struct {
	Updated time.Time     `json:"-"`
	Mu      *sync.RWMutex `json:"-"`

	Updates []*Update `json:"updates"`
}

type Update struct {
	Type    string `json:"info_type"`
	Message string `json:"message"`
}
