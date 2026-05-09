package analysis

import "sync"

// OrderManager is a standard Singleton holder
type OrderManager struct {
	items []string
}

var Instance *OrderManager
var lock = &sync.Mutex{}

func GetManager() *OrderManager {
	if Instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if Instance == nil {
			Instance = &OrderManager{}
		}
	}
	return Instance
}

// WidgetFactory acts as a creator class
type WidgetFactory struct{}

func (f *WidgetFactory) CreateBasicWidget() string { return "widget" }
func (f *WidgetFactory) NewFancyWidget() string { return "fancy_widget" }

// EventListener enables subscription
type EventListener interface {
	OnEvent()
}

type Dispatcher struct {
	listeners []EventListener
}

func (d *Dispatcher) Subscribe(l EventListener) {
	d.listeners = append(d.listeners, l)
}

func (d *Dispatcher) NotifyAll() {
	for _, l := range d.listeners {
		l.OnEvent()
	}
}
