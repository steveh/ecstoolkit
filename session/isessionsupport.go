package session

// ISessionSupport defines the interface for session support.
// It combines the functionalities of ISessionTypeSupport and ISessionSubTypeSupport.
type ISessionSupport interface {
	ISessionTypeSupport
	ISessionSubTypeSupport
}
