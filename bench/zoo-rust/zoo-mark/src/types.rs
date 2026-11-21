// The structures of the cells that name a field in the rule of the sink (docs/DESIGN-field-sinks.md; the same as the
// Go zoo's mark/types.go). Both the analysis and the harness read this file.
use std::collections::HashMap;

/// A statement whose text is dangerous and whose bind parameters are not.
pub struct Stmt {
    pub sql: String,
    pub params: HashMap<String, String>,
}

impl Stmt {
    pub fn new(sql: &str) -> Stmt {
        Stmt { sql: sql.to_string(), params: HashMap::new() }
    }
}

/// The request of a service that makes a task, and `Task` the task: an optional struct that holds an optional
/// oneof (an enum) whose variant holds the request of the HTTP call (the shape of a generated API).
pub struct CreateTask {
    pub parent: String,
    pub task: Option<Box<Task>>,
}

pub struct Task {
    pub name: String,
    pub message_type: Option<TaskMessage>,
}

pub enum TaskMessage {
    HttpRequest(HttpRequest),
}

/// The request: the URL is the sink (the body is not).
pub struct HttpRequest {
    pub url: String,
    pub body: String,
}
