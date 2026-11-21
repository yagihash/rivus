package mark

// The structures that the sinks of the cells read a part of (docs/DESIGN-field-sinks.md): the shapes of
// the requests of a generated API.

// Stmt is a statement whose text is dangerous and whose bind parameters are not (spanner.Statement).
type Stmt struct {
	SQL    string
	Params map[string]string
}

// CreateTask is the request of a service that makes a task, and Task the task: a pointer to a struct that
// holds a pointer to a struct that holds an interface (a oneof) whose value is a wrapper that holds the
// request of the HTTP call (cloudtasks).
type CreateTask struct {
	Parent string
	Task   *Task
}

// Task has the oneof MessageType.
type Task struct {
	Name        string
	MessageType isTaskMessage
}

type isTaskMessage interface{ isTaskMessage() }

// TaskHTTPRequest is the wrapper of the HTTP request in the oneof.
type TaskHTTPRequest struct{ HTTPRequest *HTTPRequest }

func (*TaskHTTPRequest) isTaskMessage() {}

// HTTPRequest is the request: the URL is the sink (the body is not).
type HTTPRequest struct {
	URL  string
	Body string
}
