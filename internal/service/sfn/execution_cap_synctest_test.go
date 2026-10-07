package sfn

import (
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

type executionCapCase struct {
	name           string
	waitSeconds    int
	definitionTime string
	wantStatus     ExecutionStatus
	wantError      string
	wantElapsed    time.Duration
}

const executionCapUnder = executionTimeoutCap - time.Second

var executionCapCases = []executionCapCase{
	{
		name:        "wait beyond cap fails with States.Runtime",
		waitSeconds: 600,
		wantStatus:  ExecutionStatusFailed,
		wantError:   errorStatesRuntime,
		wantElapsed: executionTimeoutCap,
	},
	{
		name:        "wait just under cap succeeds",
		waitSeconds: int(executionCapUnder / time.Second),
		wantStatus:  ExecutionStatusSucceeded,
		wantElapsed: executionCapUnder,
	},
	{
		name:           "definition timeout above cap does not replace it",
		waitSeconds:    600,
		definitionTime: `"TimeoutSeconds": 3600,`,
		wantStatus:     ExecutionStatusFailed,
		wantError:      errorStatesRuntime,
		wantElapsed:    executionTimeoutCap,
	},
	{
		name:           "definition timeout below cap reports States.Timeout",
		waitSeconds:    600,
		definitionTime: `"TimeoutSeconds": 60,`,
		wantStatus:     ExecutionStatusFailed,
		wantError:      errorStatesTimeout,
		wantElapsed:    time.Minute,
	},
}

func TestRunExecution_TimeoutCap(t *testing.T) {
	t.Parallel()

	for _, tt := range executionCapCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				runExecutionCapCase(t, &tt)
			})
		})
	}
}

// runExecutionCapCase runs a Wait-only state machine inside the current
// synctest bubble and checks how the execution ends.
func runExecutionCapCase(t *testing.T, tt *executionCapCase) {
	t.Helper()

	definition := fmt.Sprintf(`{
		%s
		"StartAt": "Hold",
		"States": {
			"Hold": {"Type": "Wait", "Seconds": %d, "End": true}
		}
	}`, tt.definitionTime, tt.waitSeconds)

	store := NewMemoryStorage()
	started := startBubbleExecution(t, store, definition)

	exec := awaitExecutionTerminal(t, store, started.ExecutionArn)
	if exec.Status != tt.wantStatus || exec.Error != tt.wantError {
		t.Fatalf("execution: got status %q error %q (cause: %s), want %q %q", exec.Status, exec.Error, exec.Cause, tt.wantStatus, tt.wantError)
	}

	if got := executionElapsed(t, exec); got != tt.wantElapsed {
		t.Fatalf("execution duration: got %v, want %v", got, tt.wantElapsed)
	}

	if tt.wantError == errorStatesRuntime && !strings.Contains(exec.Cause, executionTimeoutCap.String()) {
		t.Fatalf("cause %q does not mention the %s cap", exec.Cause, executionTimeoutCap)
	}
}
