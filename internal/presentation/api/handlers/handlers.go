package handlers

// Handlers aggregates every HTTP handler. Wire populates it via wire.Struct.
// Register a new resource by adding its handler field here.
type Handlers struct {
	HealthcheckHandler *HealthcheckHandler
	UserAuthHandler    *UserAuthHandler
	TaskHandler        *TaskHandler
}
