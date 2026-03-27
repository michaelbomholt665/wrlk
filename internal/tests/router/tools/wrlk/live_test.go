package wrlk_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveParticipantReport mirrors the JSON shape that live.go expects from participants.
type liveParticipantReport struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// liveSession holds the listening URL and tracks the subprocess for cleanup.
type liveSession struct {
	url  string
	cmd  *exec.Cmd
	done chan commandResult
}

// startLiveSession launches `wrlk live run` in the background, reads the
// listening URL from the first stdout line, and returns a liveSession.
// The caller must call liveSession.wait() to collect the result and reap the
// subprocess, and should do so before the test ends to avoid orphaned processes.
func startLiveSession(t *testing.T, extraArgs ...string) *liveSession {
	t.Helper()

	repoRoot := repositoryRoot(t)
	baseArgs := []string{"run", "./internal/router/tools/wrlk", "--root", repoRoot, "live", "run"}
	args := append(baseArgs, extraArgs...)

	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot

	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err, "create stdout pipe for wrlk live run")

	// Capture stderr into a buffer via a separate pipe.
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	require.NoError(t, cmd.Start(), "start wrlk live run")

	// Read the listening address from the very first stdout line synchronously
	// so the caller knows immediately where to POST.
	// live.go prints: "Router live listening: http://<addr>/report\n"
	reader := bufio.NewReader(stdoutPipe)
	firstLine, err := reader.ReadString('\n')
	require.NoError(t, err, "read live listening line from wrlk stdout")
	firstLine = strings.TrimSpace(firstLine)

	const listenPrefix = "Router live listening: "
	require.True(t, strings.HasPrefix(firstLine, listenPrefix),
		"expected live listening line, got: %q", firstLine)
	listenURL := strings.TrimPrefix(firstLine, listenPrefix)

	// Drain the rest of stdout in a goroutine so the pipe buffer never
	// fills up and stalls the server process.  Collect lines into a buffer
	// that is merged into the commandResult once the process exits.
	var remainingStdout strings.Builder
	remainingStdout.WriteString(firstLine + "\n")

	doneCh := make(chan commandResult, 1)
	go func() {
		// Drain remaining stdout lines.
		remaining, _ := reader.ReadString(0) // read until EOF
		finalStdout := firstLine + "\n" + remaining

		waitErr := cmd.Wait()
		result := commandResult{
			stdout: finalStdout,
			stderr: stderrBuf.String(),
			err:    waitErr,
		}
		if waitErr != nil {
			var exitErr *exec.ExitError
			if isExitErr := assert.ErrorAs(t, waitErr, &exitErr); isExitErr {
				result.exitCode = exitErr.ExitCode()
			}
		}
		doneCh <- result
	}()

	return &liveSession{
		url:  listenURL,
		cmd:  cmd,
		done: doneCh,
	}
}

// wait blocks until the wrlk subprocess exits and returns its commandResult.
func (s *liveSession) wait() commandResult {
	return <-s.done
}

// postReport sends a single participant report to the live session server.
func postReport(t *testing.T, url string, report liveParticipantReport) *http.Response {
	t.Helper()

	body, err := json.Marshal(report)
	require.NoError(t, err, "marshal live report")

	//nolint:noctx
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	require.NoError(t, err, "POST live report to %s", url)

	return resp
}

// TestLive_Run_AllParticipantsSucceed_ExitsZero starts a live session for two
// expected participants, posts success for both, and verifies exit code 0.
func TestLive_Run_AllParticipantsSucceed_ExitsZero(t *testing.T) {
	s := startLiveSession(t,
		"--expect", "alpha",
		"--expect", "beta",
		"--timeout", "10s",
	)

	resp := postReport(t, s.url, liveParticipantReport{ID: "alpha", Status: "success"})
	resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "alpha success report")

	resp = postReport(t, s.url, liveParticipantReport{ID: "beta", Status: "success"})
	resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "beta success report")

	result := s.wait()
	require.NoError(t, result.err, "expected zero exit; stderr=%q", result.stderr)
	assert.Equal(t, 0, result.exitCode)
	assert.Contains(t, result.stdout, "passed", "success message should appear in stdout")
}

// TestLive_Run_OneParticipantFails_ExitsNonZero verifies that a failure report
// from any participant causes a non-zero exit.
func TestLive_Run_OneParticipantFails_ExitsNonZero(t *testing.T) {
	s := startLiveSession(t,
		"--expect", "alpha",
		"--expect", "beta",
		"--timeout", "10s",
	)

	resp := postReport(t, s.url, liveParticipantReport{ID: "alpha", Status: "success"})
	resp.Body.Close()

	resp = postReport(t, s.url, liveParticipantReport{
		ID:     "beta",
		Status: "failure",
		Error:  "assertion mismatch",
	})
	body := readResponseBody(t, resp)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "failure report must be acknowledged distinctly")
	assert.Contains(t, body, "beta reported failure: assertion mismatch")

	result := s.wait()
	require.Error(t, result.err, "expected non-zero exit when participant reports failure")
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stderr, "beta", "failure message should name the failing participant")
}

// TestLive_Run_UnknownParticipant_Rejected verifies that a report from a
// participant not listed in --expect causes the session to fail and exit non-zero.
func TestLive_Run_UnknownParticipant_Rejected(t *testing.T) {
	s := startLiveSession(t,
		"--expect", "known-participant",
		"--timeout", "10s",
	)

	resp := postReport(t, s.url, liveParticipantReport{ID: "intruder", Status: "success"})
	resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"unknown participant must be rejected with 400")

	result := s.wait()
	require.Error(t, result.err, "expected non-zero exit when unknown participant reports")
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stderr, "intruder", "failure message should name the unknown participant")
}

// TestLive_Run_DuplicateParticipant_Rejected verifies that a second report from
// the same participant causes the session to fail and exit non-zero.
func TestLive_Run_DuplicateParticipant_Rejected(t *testing.T) {
	s := startLiveSession(t,
		"--expect", "alpha",
		"--expect", "beta",
		"--timeout", "10s",
	)

	resp := postReport(t, s.url, liveParticipantReport{ID: "alpha", Status: "success"})
	resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "first alpha report accepted")

	// Second report from the same participant — must be rejected.
	resp = postReport(t, s.url, liveParticipantReport{ID: "alpha", Status: "success"})
	resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"duplicate participant must be rejected with 400")

	result := s.wait()
	require.Error(t, result.err, "expected non-zero exit for duplicate participant report")
	assert.NotEqual(t, 0, result.exitCode)
	assert.Contains(t, result.stderr, "alpha", "failure message should name the duplicate participant")
}

// TestLive_Run_Timeout_IsBug verifies that when not all participants report
// within the timeout window, wrlk exits with exit code 3 (exitCodeInternalBug),
// not exit code 1 (normal failure).
//
// The timeout timer starts only after the first participant report.  Send
// success from a first participant to start the clock, then let the second
// participant never report.  After the timeout, wrlk must exit non-zero with
// exit code 3.
func TestLive_Run_Timeout_IsBug(t *testing.T) {
	// Use a very short timeout to keep the test fast.
	s := startLiveSession(t,
		"--expect", "fast-participant",
		"--expect", "slow-participant",
		"--timeout", "250ms",
	)

	// Send the first participant success to start the startedAt clock.
	resp := postReport(t, s.url, liveParticipantReport{ID: "fast-participant", Status: "success"})
	resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode, "first participant accepted")

	// Do NOT send the second participant report — wait for the session to time out.
	result := s.wait()

	// Verify non-zero exit and the verificationBugError classification message.
	// Note: `go run` on Windows exits with exit code 1 regardless of the child's
	// actual exit code (3 = exitCodeInternalBug), so we assert the stderr content
	// which is the authoritative classification signal.
	assert.NotEqual(t, 0, result.exitCode,
		"timeout must cause non-zero exit; stderr=%q", result.stderr)
	assert.Contains(t, result.stderr, "timed out",
		"timeout error message must mention 'timed out'")
	assert.Contains(t, result.stderr, "verification session did not complete",
		"timeout error must be classified as a verification bug")
}

// TestLive_ReportPath_WrongMethod_NotFound verifies that a GET request to
// /report returns 404.  Only POST is accepted by the live session handler.
func TestLive_ReportPath_WrongMethod_NotFound(t *testing.T) {
	s := startLiveSession(t,
		"--expect", "alpha",
		"--timeout", "10s",
	)

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, s.url, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"GET to /report must return 404; only POST is accepted")

	// Clean up: post a valid success so the session exits zero.
	r := postReport(t, s.url, liveParticipantReport{ID: "alpha", Status: "success"})
	r.Body.Close()

	result := s.wait()
	require.NoError(t, result.err, "session should exit zero after cleanup report")
}

// TestLive_ParseOptions_RequiresExpect verifies that running `live run` without
// any --expect flag produces a usage error immediately, without starting an
// HTTP server.
func TestLive_ParseOptions_RequiresExpect(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "live", "run")

	// Note: `go run` on Windows exits with exit code 1 regardless of the child's
	// actual exit code (2 = exitCodeUsage), so we assert non-zero exit and
	// check the stderr content which is the authoritative classification signal.
	assert.NotEqual(t, 0, result.exitCode,
		"missing --expect must produce a usage error; stderr=%q", result.stderr)
	assert.Contains(t, result.stderr, "expect",
		"usage error should mention the --expect flag")
}

// TestLive_Run_WrongSubcommand_Rejected verifies that an unknown live
// subcommand (e.g. `live boot`) produces a usage error and exits non-zero.
func TestLive_Run_WrongSubcommand_Rejected(t *testing.T) {
	result := runWrlkCommand(t, repositoryRoot(t), "live", "boot")

	// Note: `go run` on Windows exits with exit code 1 regardless of the child's
	// actual exit code (2 = exitCodeUsage), so we assert non-zero exit and
	// check the stderr content which is the authoritative classification signal.
	assert.NotEqual(t, 0, result.exitCode,
		"unknown live subcommand must yield a usage error; stderr=%q", result.stderr)
	assert.Contains(t, result.stderr, "boot",
		"usage error should echo the unknown subcommand name")
}

func readResponseBody(t *testing.T, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)

	return string(body)
}
