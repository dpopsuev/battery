package session

import "time"

// Session holds the state of an interactive conversation.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Driver    string    `json:"driver,omitempty"`
	Model     string    `json:"model"`
	Mode      string    `json:"mode,omitempty"`
	WorkDir   string    `json:"work_dir"`
	Workspace string    `json:"workspace,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	History   *History  `json:"history"`
}

// New creates a new session.
func New(id, model, workDir string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Model:     model,
		WorkDir:   workDir,
		CreatedAt: now,
		UpdatedAt: now,
		History:   NewHistory(0),
	}
}

// Append adds an entry to the session history.
func (s *Session) Append(entry Entry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	s.History.Append(entry)
	s.UpdatedAt = time.Now()
}

// Entries returns the conversation history.
func (s *Session) Entries() []Entry {
	return s.History.Entries()
}

// TotalTokens returns the approximate total token count.
func (s *Session) TotalTokens() int {
	return s.History.TotalTokens()
}
