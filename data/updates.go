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
	// Type defines what color the message is rendered in. Supported are:
	// - "info": Light blue background.
	// - "warn": Yellow background.
	// - "danger": Dark red background.
	// - "success": Dark green background.
	// - every other value will be rendered with dark grey background.
	// For more details, see https://getbootstrap.com/docs/5.3/components/alerts/
	Type string `json:"info_type"`

	Message string `json:"message"`
}
