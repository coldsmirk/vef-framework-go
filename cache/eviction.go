package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
)

// EvictionPolicy defines the eviction strategy for cache when it reaches max size.
type EvictionPolicy int

const (
	// EvictionPolicyNone disables eviction tracking (used for unlimited caches).
	EvictionPolicyNone EvictionPolicy = iota
	// EvictionPolicyLRU evicts least recently used entries when cache is full.
	EvictionPolicyLRU
	// EvictionPolicyLFU evicts least frequently used entries when cache is full.
	EvictionPolicyLFU
	// EvictionPolicyFIFO evicts oldest entries when cache is full.
	EvictionPolicyFIFO
)

// EvictionHandler defines the interface for handling cache eviction.
type EvictionHandler interface {
	// OnAccess is called when an entry is accessed.
	OnAccess(key string)
	// OnInsert is called when a new entry is inserted.
	OnInsert(key string)
	// OnEvict is called when an entry is evicted.
	OnEvict(key string)
	// SelectEvictionCandidate selects a key to evict, returns empty string if no candidate.
	SelectEvictionCandidate() string
	// Reset clears all eviction tracking data.
	Reset()
}

// NoOpEvictionHandler is used when eviction policy is EvictionPolicyNone.
type NoOpEvictionHandler struct{}

func NewNoOpEvictionHandler() *NoOpEvictionHandler {
	return new(NoOpEvictionHandler)
}

func (*NoOpEvictionHandler) OnAccess(_ string)               {}
func (*NoOpEvictionHandler) OnInsert(_ string)               {}
func (*NoOpEvictionHandler) OnEvict(_ string)                {}
func (*NoOpEvictionHandler) SelectEvictionCandidate() string { return "" }
func (*NoOpEvictionHandler) Reset()                          {}

// LruHandler implements Least Recently Used eviction policy.
type LruHandler struct {
	mu         sync.RWMutex
	accessList *list.List
	accessMap  map[string]*list.Element
}

// NewLruHandler creates a new LRU eviction handler.
func NewLruHandler() *LruHandler {
	return &LruHandler{
		accessList: list.New(),
		accessMap:  make(map[string]*list.Element),
	}
}

func (h *LruHandler) OnAccess(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if elem, exists := h.accessMap[key]; exists {
		// Move to front (most recently used)
		h.accessList.MoveToFront(elem)
	} else {
		// Add to front
		elem := h.accessList.PushFront(key)
		h.accessMap[key] = elem
	}
}

func (h *LruHandler) OnInsert(key string) {
	// Treat insert as access
	h.OnAccess(key)
}

func (h *LruHandler) OnEvict(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if elem, exists := h.accessMap[key]; exists {
		h.accessList.Remove(elem)
		delete(h.accessMap, key)
	}
}

func (h *LruHandler) SelectEvictionCandidate() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return least recently used (back of list)
	elem := h.accessList.Back()
	if elem == nil {
		return ""
	}

	if key, ok := elem.Value.(string); ok {
		return key
	}

	return ""
}

func (h *LruHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.accessList = list.New()
	h.accessMap = make(map[string]*list.Element)
}

// FifoHandler implements First In First Out eviction policy.
type FifoHandler struct {
	mu         sync.RWMutex
	insertList *list.List
	insertMap  map[string]*list.Element
}

// NewFifoHandler creates a new FIFO eviction handler.
func NewFifoHandler() *FifoHandler {
	return &FifoHandler{
		insertList: list.New(),
		insertMap:  make(map[string]*list.Element),
	}
}

func (*FifoHandler) OnAccess(_ string) {
	// FIFO doesn't track access, only insertion order
}

func (h *FifoHandler) OnInsert(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.insertMap[key]; !exists {
		// Add to back (newest entries)
		elem := h.insertList.PushBack(key)
		h.insertMap[key] = elem
	}
}

func (h *FifoHandler) OnEvict(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if elem, exists := h.insertMap[key]; exists {
		h.insertList.Remove(elem)
		delete(h.insertMap, key)
	}
}

func (h *FifoHandler) SelectEvictionCandidate() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return oldest (front of list)
	elem := h.insertList.Front()
	if elem == nil {
		return ""
	}

	if key, ok := elem.Value.(string); ok {
		return key
	}

	return ""
}

func (h *FifoHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.insertList = list.New()
	h.insertMap = make(map[string]*list.Element)
}

// lfuNode represents a node in the LFU frequency list.
type lfuNode struct {
	key         string
	frequency   int64
	insertOrder int64 // For tie-breaking
}

// lfuFreqBucket represents a bucket of entries with the same frequency.
type lfuFreqBucket struct {
	frequency int64
	entries   *list.List // List of *lfuNode
	nodeMap   map[string]*list.Element
}

// LfuHandler implements Least Frequently Used eviction policy using frequency buckets.
// This implementation achieves O(1) time complexity for all operations.
type LfuHandler struct {
	mu            sync.RWMutex
	freqBuckets   *list.List // List of *lfuFreqBucket, sorted by frequency
	bucketMap     map[int64]*list.Element
	keyToBucket   map[string]*list.Element // Maps key to its bucket element
	keyToNode     map[string]*lfuNode
	minFreq       int64
	insertCounter int64
}

// NewLfuHandler creates a new LFU eviction handler.
func NewLfuHandler() *LfuHandler {
	return &LfuHandler{
		freqBuckets: list.New(),
		bucketMap:   make(map[int64]*list.Element),
		keyToBucket: make(map[string]*list.Element),
		keyToNode:   make(map[string]*lfuNode),
		minFreq:     0,
	}
}

func (h *LfuHandler) OnAccess(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, exists := h.keyToNode[key]
	if !exists {
		return
	}

	// Increment frequency
	oldFreq := node.frequency
	newFreq := oldFreq + 1
	node.frequency = newFreq

	// Move node to new frequency bucket
	h.moveToFreqBucket(key, node, oldFreq, newFreq)

	// Update min frequency if needed
	if oldFreq == h.minFreq {
		// Check if old frequency bucket is now empty
		if bucketElem, exists := h.bucketMap[oldFreq]; exists {
			if bucket, ok := bucketElem.Value.(*lfuFreqBucket); ok && bucket.entries.Len() == 0 {
				h.minFreq = newFreq
			}
		}
	}
}

func (h *LfuHandler) OnInsert(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.keyToNode[key]; exists {
		return
	}

	// Create new node with frequency 1
	insertOrder := atomic.AddInt64(&h.insertCounter, 1)
	node := &lfuNode{
		key:         key,
		frequency:   1,
		insertOrder: insertOrder,
	}
	h.keyToNode[key] = node

	// Add to frequency 1 bucket
	h.addToFreqBucket(key, node, 1)

	// Set minimum frequency
	h.minFreq = 1
}

func (h *LfuHandler) OnEvict(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	node, exists := h.keyToNode[key]
	if !exists {
		return
	}

	// Remove from frequency bucket
	h.removeFromFreqBucket(key, node.frequency)

	// Clean up
	delete(h.keyToNode, key)
	delete(h.keyToBucket, key)

	// Recalculate min frequency if needed
	if node.frequency == h.minFreq {
		h.recalculateMinFreq()
	}
}

func (h *LfuHandler) SelectEvictionCandidate() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.keyToNode) == 0 {
		return ""
	}

	// Get the minimum frequency bucket
	bucketElem, exists := h.bucketMap[h.minFreq]
	if !exists || bucketElem == nil {
		return ""
	}

	bucket, ok := bucketElem.Value.(*lfuFreqBucket)
	if !ok || bucket.entries.Len() == 0 {
		return ""
	}

	// Return the first entry (oldest by insertion order due to FIFO within bucket)
	elem := bucket.entries.Front()
	if elem == nil {
		return ""
	}

	if node, ok := elem.Value.(*lfuNode); ok {
		return node.key
	}

	return ""
}

func (h *LfuHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.freqBuckets = list.New()
	h.bucketMap = make(map[int64]*list.Element)
	h.keyToBucket = make(map[string]*list.Element)
	h.keyToNode = make(map[string]*lfuNode)
	h.minFreq = 0
	h.insertCounter = 0
}

// addToFreqBucket adds a node to the specified frequency bucket.
func (h *LfuHandler) addToFreqBucket(key string, node *lfuNode, freq int64) {
	bucketElem, exists := h.bucketMap[freq]

	var bucket *lfuFreqBucket

	if !exists {
		// Create new bucket
		bucket = &lfuFreqBucket{
			frequency: freq,
			entries:   list.New(),
			nodeMap:   make(map[string]*list.Element),
		}

		// Insert bucket in sorted order
		bucketElem = h.insertBucketSorted(bucket)
		h.bucketMap[freq] = bucketElem
	} else if b, ok := bucketElem.Value.(*lfuFreqBucket); ok {
		bucket = b
	}

	// Add node to bucket (at back for FIFO within same frequency)
	nodeElem := bucket.entries.PushBack(node)
	bucket.nodeMap[key] = nodeElem
	h.keyToBucket[key] = bucketElem
}

// removeFromFreqBucket removes a node from the specified frequency bucket.
func (h *LfuHandler) removeFromFreqBucket(key string, freq int64) {
	bucketElem, exists := h.bucketMap[freq]
	if !exists {
		return
	}

	bucket, ok := bucketElem.Value.(*lfuFreqBucket)
	if !ok {
		return
	}

	nodeElem, exists := bucket.nodeMap[key]
	if !exists {
		return
	}

	// Remove node from bucket
	bucket.entries.Remove(nodeElem)
	delete(bucket.nodeMap, key)

	// If bucket is empty, remove it
	if bucket.entries.Len() == 0 {
		h.freqBuckets.Remove(bucketElem)
		delete(h.bucketMap, freq)
	}
}

// moveToFreqBucket moves a node from one frequency bucket to another.
func (h *LfuHandler) moveToFreqBucket(key string, node *lfuNode, oldFreq, newFreq int64) {
	h.removeFromFreqBucket(key, oldFreq)
	h.addToFreqBucket(key, node, newFreq)
}

// insertBucketSorted inserts a bucket in sorted order by frequency.
func (h *LfuHandler) insertBucketSorted(bucket *lfuFreqBucket) *list.Element {
	// Find insertion point
	for elem := h.freqBuckets.Front(); elem != nil; elem = elem.Next() {
		existingBucket, ok := elem.Value.(*lfuFreqBucket)
		if ok && bucket.frequency < existingBucket.frequency {
			return h.freqBuckets.InsertBefore(bucket, elem)
		}
	}

	// Insert at end
	return h.freqBuckets.PushBack(bucket)
}

// recalculateMinFreq recalculates the minimum frequency from current buckets.
func (h *LfuHandler) recalculateMinFreq() {
	if h.freqBuckets.Len() == 0 {
		h.minFreq = 0

		return
	}

	// The first bucket has the minimum frequency
	elem := h.freqBuckets.Front()
	if elem != nil {
		if bucket, ok := elem.Value.(*lfuFreqBucket); ok {
			h.minFreq = bucket.frequency
		}
	}
}

// EvictionHandlerFactory creates eviction handlers based on policy.
type EvictionHandlerFactory struct{}

func (*EvictionHandlerFactory) CreateHandler(policy EvictionPolicy) EvictionHandler {
	switch policy {
	case EvictionPolicyLRU:
		return NewLruHandler()
	case EvictionPolicyLFU:
		return NewLfuHandler()
	case EvictionPolicyFIFO:
		return NewFifoHandler()
	case EvictionPolicyNone:
		fallthrough
	default:
		return NewNoOpEvictionHandler()
	}
}
