package session

// SessionRegistry stores the mapping of session types to their implementations.
var SessionRegistry = map[string]ISessionPlugin{}

func init() {
	SessionRegistry = make(map[string]ISessionPlugin)
}

// Register adds a session plugin to the registry.
func Register(session ISessionPlugin) {
	SessionRegistry[session.Name()] = session
}
