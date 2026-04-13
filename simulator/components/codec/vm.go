package codec

import (
	"sync"

	"github.com/dop251/goja"
)

// VMPool manages a pool of goja VMs for reuse
type VMPool struct {
	pool chan *goja.Runtime
	size int
	mu   sync.Mutex
}

// NewVMPool creates a new VM pool with the specified size.
// All VMs are pre-created and callers block until one is available.
func NewVMPool(size int) *VMPool {
	if size <= 0 {
		size = 10
	}

	p := &VMPool{
		pool: make(chan *goja.Runtime, size),
		size: size,
	}
	for i := 0; i < size; i++ {
		p.pool <- p.createVM()
	}
	return p
}

// Get retrieves a VM from the pool, blocking until one is available.
func (p *VMPool) Get() *goja.Runtime {
	return <-p.pool
}

// Put returns a VM to the pool after clearing its state.
func (p *VMPool) Put(vm *goja.Runtime) {
	if vm == nil {
		return
	}
	p.clearVM(vm)
	p.pool <- vm
}

// createVM creates a new goja Runtime instance
func (p *VMPool) createVM() *goja.Runtime {
	vm := goja.New()

	// Set up basic JavaScript environment
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	// Enable console.log for debugging
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		// In production, you might want to use a proper logger
		// For now, we'll just ignore console.log calls
		return goja.Undefined()
	})
	vm.Set("console", console)

	return vm
}

// clearVM resets the VM state to prepare it for reuse
func (p *VMPool) clearVM(vm *goja.Runtime) {
	// Remove custom properties that might have been set
	// Note: goja doesn't have a built-in way to completely reset,
	// so we manually remove known custom properties

	// Remove state helper functions
	vm.Set("getState", goja.Undefined())
	vm.Set("setState", goja.Undefined())

	// Remove device helper functions
	vm.Set("getSendInterval", goja.Undefined())
	vm.Set("setSendInterval", goja.Undefined())
	vm.Set("log", goja.Undefined())

	// Remove conversion helpers
	vm.Set("hexToBytes", goja.Undefined())
	vm.Set("base64ToBytes", goja.Undefined())

	// Remove codec functions
	vm.Set("OnUplink", goja.Undefined())
	vm.Set("OnDownlink", goja.Undefined())
}

// Close closes the pool and releases all VMs
func (p *VMPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Drain the pool
	close(p.pool)
	for range p.pool {
		// VMs will be garbage collected
	}
}

// Size returns the maximum size of the pool
func (p *VMPool) Size() int {
	return p.size
}

// Available returns the number of VMs currently available in the pool
func (p *VMPool) Available() int {
	return len(p.pool)
}
