package sessions

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	output []byte
	err    error
	calls  []mockCall
}

type mockCall struct {
	name string
	args []string
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, mockCall{name: name, args: args})
	return m.output, m.err
}

func TestParseOpenCodeSessions(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    int
		wantErr bool
	}{
		{
			name:    "valid JSON array",
			input:   []byte(`[{"id":"ses_1","title":"test","updated":1000,"created":900,"projectId":"proj1","directory":"/tmp"}]`),
			want:    1,
			wantErr: false,
		},
		{
			name:    "multiple sessions",
			input:   []byte(`[{"id":"ses_1","title":"first","updated":1000,"created":900,"projectId":"proj1","directory":"/tmp"},{"id":"ses_2","title":"second","updated":2000,"created":1900,"projectId":"proj1","directory":"/tmp"}]`),
			want:    2,
			wantErr: false,
		},
		{
			name:    "empty array",
			input:   []byte(`[]`),
			want:    0,
			wantErr: false,
		},
		{
			name:    "RTK prefix before JSON",
			input:   []byte("RTK: compressed 42 lines\n[{\"id\":\"ses_1\",\"title\":\"test\",\"updated\":1000,\"created\":900,\"projectId\":\"proj1\",\"directory\":\"/tmp\"}]"),
			want:    1,
			wantErr: false,
		},
		{
			name:    "whitespace before JSON",
			input:   []byte("  \n  [{\"id\":\"ses_1\",\"title\":\"test\",\"updated\":1000,\"created\":900,\"projectId\":\"proj1\",\"directory\":\"/tmp\"}]"),
			want:    1,
			wantErr: false,
		},
		{
			name:    "no JSON array found",
			input:   []byte("no json here"),
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   []byte("[{invalid json}]"),
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   []byte(""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions, err := ParseOpenCodeSessions(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, sessions, tt.want)
		})
	}
}

func TestParseOpenCodeSessionsFields(t *testing.T) {
	input := []byte(`[{"id":"ses_abc123","title":"My Session","updated":1700000000,"created":1699000000,"projectId":"proj_xyz","directory":"/home/user/project"}]`)

	sessions, err := ParseOpenCodeSessions(input)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	s := sessions[0]
	assert.Equal(t, "ses_abc123", s.ID)
	assert.Equal(t, "My Session", s.Title)
	assert.Equal(t, int64(1700000000), s.Updated)
	assert.Equal(t, int64(1699000000), s.Created)
	assert.Equal(t, "proj_xyz", s.ProjectID)
	assert.Equal(t, "/home/user/project", s.Directory)
}

func TestFindMostRecentSession(t *testing.T) {
	tests := []struct {
		name     string
		sessions []OpenCodeSession
		wantID   string
		wantNil  bool
	}{
		{
			name:     "empty list",
			sessions: []OpenCodeSession{},
			wantNil:  true,
		},
		{
			name: "single session",
			sessions: []OpenCodeSession{
				{ID: "ses_1", Updated: 1000},
			},
			wantID: "ses_1",
		},
		{
			name: "most recent is last",
			sessions: []OpenCodeSession{
				{ID: "ses_1", Updated: 1000},
				{ID: "ses_2", Updated: 2000},
				{ID: "ses_3", Updated: 3000},
			},
			wantID: "ses_3",
		},
		{
			name: "most recent is first",
			sessions: []OpenCodeSession{
				{ID: "ses_3", Updated: 3000},
				{ID: "ses_1", Updated: 1000},
				{ID: "ses_2", Updated: 2000},
			},
			wantID: "ses_3",
		},
		{
			name: "most recent is middle",
			sessions: []OpenCodeSession{
				{ID: "ses_1", Updated: 1000},
				{ID: "ses_3", Updated: 3000},
				{ID: "ses_2", Updated: 2000},
			},
			wantID: "ses_3",
		},
		{
			name: "equal timestamps picks first",
			sessions: []OpenCodeSession{
				{ID: "ses_1", Updated: 1000},
				{ID: "ses_2", Updated: 1000},
			},
			wantID: "ses_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindMostRecentSession(tt.sessions)
			if tt.wantNil {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, tt.wantID, result.ID)
		})
	}
}

func TestDiscoverOpenCodeSessions(t *testing.T) {
	validJSON := `[{"id":"ses_1","title":"test","updated":1000,"created":900,"projectId":"proj1","directory":"/tmp"}]`

	tests := []struct {
		name       string
		session    *Session
		mockOutput []byte
		mockErr    error
		wantCount  int
		wantErr    bool
		wantCmd    string
	}{
		{
			name:       "devcontainer worker discovers via docker exec",
			session:    &Session{Runtime: "devcontainer", Container: "yak-worker-test"},
			mockOutput: []byte(validJSON),
			wantCount:  1,
			wantCmd:    "docker",
		},
		{
			name:       "native worker discovers via opencode --dir",
			session:    &Session{Runtime: "native", CWD: "/home/user/project"},
			mockOutput: []byte(validJSON),
			wantCount:  1,
			wantCmd:    "opencode",
		},
		{
			name:       "command failure returns error",
			session:    &Session{Runtime: "devcontainer", Container: "yak-worker-test"},
			mockOutput: []byte("container not running"),
			mockErr:    fmt.Errorf("exit status 1"),
			wantErr:    true,
		},
		{
			name:       "invalid JSON output returns error",
			session:    &Session{Runtime: "native", CWD: "/tmp"},
			mockOutput: []byte("not json"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{output: tt.mockOutput, err: tt.mockErr}
			sessions, err := DiscoverOpenCodeSessions(runner, tt.session)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, sessions, tt.wantCount)
			assert.Equal(t, tt.wantCmd, runner.calls[0].name)
		})
	}
}

func TestDiscoverOpenCodeSessionsDockerArgs(t *testing.T) {
	runner := &mockRunner{output: []byte("[]")}
	session := &Session{Runtime: "devcontainer", Container: "yak-worker-api"}

	_, err := DiscoverOpenCodeSessions(runner, session)
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, "docker", call.name)
	assert.Equal(t, []string{"exec", "yak-worker-api", "opencode", "session", "list", "--format", "json"}, call.args)
}

func TestDiscoverOpenCodeSessionsNativeArgs(t *testing.T) {
	runner := &mockRunner{output: []byte("[]")}
	session := &Session{Runtime: "native", CWD: "/home/user/project"}

	_, err := DiscoverOpenCodeSessions(runner, session)
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, "opencode", call.name)
	assert.Equal(t, []string{"session", "list", "--format", "json", "--dir", "/home/user/project"}, call.args)
}

func TestSendMessage(t *testing.T) {
	tests := []struct {
		name       string
		session    *Session
		sessionID  string
		message    string
		format     string
		mockOutput []byte
		mockErr    error
		wantErr    bool
		wantCmd    string
	}{
		{
			name:       "devcontainer worker sends via docker exec",
			session:    &Session{Runtime: "devcontainer", Container: "yak-worker-test"},
			sessionID:  "ses_1",
			message:    "hello worker",
			format:     "",
			mockOutput: []byte("response text"),
			wantCmd:    "docker",
		},
		{
			name:       "native worker sends via opencode directly",
			session:    &Session{Runtime: "native", CWD: "/tmp"},
			sessionID:  "ses_2",
			message:    "do something",
			format:     "json",
			mockOutput: []byte(`{"result":"ok"}`),
			wantCmd:    "opencode",
		},
		{
			name:       "default format is omitted from args",
			session:    &Session{Runtime: "native", CWD: "/tmp"},
			sessionID:  "ses_3",
			message:    "test",
			format:     "default",
			mockOutput: []byte("ok"),
			wantCmd:    "opencode",
		},
		{
			name:       "command failure returns error",
			session:    &Session{Runtime: "native", CWD: "/tmp"},
			sessionID:  "ses_4",
			message:    "test",
			mockOutput: []byte("connection refused"),
			mockErr:    fmt.Errorf("connection refused"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockRunner{output: tt.mockOutput, err: tt.mockErr}
			result, err := SendMessage(runner, tt.session, tt.sessionID, tt.message, tt.format)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, string(tt.mockOutput), result.Output)
			assert.Equal(t, 0, result.ExitCode)
			assert.Equal(t, tt.wantCmd, runner.calls[0].name)
		})
	}
}

func TestSendMessageDockerArgs(t *testing.T) {
	runner := &mockRunner{output: []byte("ok")}
	session := &Session{Runtime: "devcontainer", Container: "yak-worker-api"}

	_, err := SendMessage(runner, session, "ses_1", "hello", "json")
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, "docker", call.name)
	assert.Equal(t, []string{"exec", "yak-worker-api", "opencode", "run", "--session", "ses_1", "--format", "json", "hello"}, call.args)
}

func TestSendMessageNativeArgs(t *testing.T) {
	runner := &mockRunner{output: []byte("ok")}
	session := &Session{Runtime: "native", CWD: "/home/user/project"}

	_, err := SendMessage(runner, session, "ses_2", "do work", "")
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, "opencode", call.name)
	assert.Equal(t, []string{"run", "--session", "ses_2", "--dir", "/home/user/project", "do work"}, call.args)
}

func TestSendMessageNativeWithFormatArgs(t *testing.T) {
	runner := &mockRunner{output: []byte("ok")}
	session := &Session{Runtime: "native", CWD: "/home/user/project"}

	_, err := SendMessage(runner, session, "ses_2", "do work", "json")
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, "opencode", call.name)
	assert.Equal(t, []string{"run", "--session", "ses_2", "--dir", "/home/user/project", "--format", "json", "do work"}, call.args)
}

func TestSendMessageNativeEmptyCWDOmitsDir(t *testing.T) {
	runner := &mockRunner{output: []byte("ok")}
	session := &Session{Runtime: "native", CWD: ""}

	_, err := SendMessage(runner, session, "ses_5", "test", "")
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, "opencode", call.name)
	// --dir should be omitted when CWD is empty
	assert.Equal(t, []string{"run", "--session", "ses_5", "test"}, call.args)
}

func TestSendMessageDefaultFormatOmitted(t *testing.T) {
	runner := &mockRunner{output: []byte("ok")}
	session := &Session{Runtime: "native", CWD: "/tmp"}

	_, err := SendMessage(runner, session, "ses_3", "test", "default")
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)

	call := runner.calls[0]
	assert.Equal(t, []string{"run", "--session", "ses_3", "--dir", "/tmp", "test"}, call.args)
}
