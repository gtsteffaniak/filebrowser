package files

import "sync"

// ResourceMetadataHooks couples optional application metadata to physical API
// mutations. Callbacks run under the existing DAV mutation gate and must not
// acquire that gate again. Registration is intended for application startup.
type ResourceMetadataHooks struct {
	Preflight func() error
	Delete    func(string) error
	Move      func(string, string) error
	Copy      func(string, string) error
}

var resourceMetadataRegistry struct {
	sync.RWMutex
	hooks ResourceMetadataHooks
}

func RegisterResourceMetadataHooks(hooks ResourceMetadataHooks) {
	resourceMetadataRegistry.Lock()
	defer resourceMetadataRegistry.Unlock()
	resourceMetadataRegistry.hooks = hooks
}
func resourceMetadataHooks() ResourceMetadataHooks {
	resourceMetadataRegistry.RLock()
	defer resourceMetadataRegistry.RUnlock()
	return resourceMetadataRegistry.hooks
}
func (h ResourceMetadataHooks) before() error {
	if h.Preflight != nil {
		return h.Preflight()
	}
	return nil
}
func (h ResourceMetadataHooks) deleted(key string) error {
	if h.Delete != nil {
		return h.Delete(key)
	}
	return nil
}
func (h ResourceMetadataHooks) moved(oldKey, newKey string) error {
	if h.Move != nil {
		return h.Move(oldKey, newKey)
	}
	return nil
}
func (h ResourceMetadataHooks) copied(oldKey, newKey string) error {
	if h.Copy != nil {
		return h.Copy(oldKey, newKey)
	}
	return nil
}
