package resources

import (
	"sync"
)

type Resources struct {
	ExitGroup sync.WaitGroup `json:"-"`
}
