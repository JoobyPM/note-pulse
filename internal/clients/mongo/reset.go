package mongo

// reset clears the singleton without going through Shutdown (helper for tests).
func reset() {
	mu.Lock()
	defer mu.Unlock()
	client = nil
	db = nil
	initErr = nil
}
