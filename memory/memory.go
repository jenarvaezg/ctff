package memory

import (
	"container/list"
	"session"
	"sync"
	"time"
)

var pder = &Provider{list: list.New()}