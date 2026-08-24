package process

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
)

// Running abstracts a started game process so the launching feature stays
// independent of exec.Command and platform stop/kill behavior.
type Running interface {
	PID() int
	Wait() (int, error)
	Stop() error
	Kill() error
}

// Launcher starts game processes with the given arguments, environment, and
// captured output.
type Launcher interface {
	Start(
		ctx context.Context,
		executable string,
		arguments []string,
		workingDirectory string,
		environment map[string]string,
		output io.Writer,
	) (Running, error)
}

// OSLauncher is the exec.Command-backed process launcher.
type OSLauncher struct{}

type runningProcess struct {
	command *exec.Cmd
}

func (OSLauncher) Start(
	ctx context.Context,
	executable string,
	arguments []string,
	workingDirectory string,
	environment map[string]string,
	output io.Writer,
) (Running, error) {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = workingDirectory
	command.Env = mergeEnvironment(os.Environ(), environment)
	command.Stdout = output
	command.Stderr = output

	if err := command.Start(); err != nil {
		slog.Warn("failed to start the game process", "error", err)
		return nil, err
	}

	slog.Info("game process started", "pid", command.Process.Pid)
	return &runningProcess{command: command}, nil
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		if key, value, ok := splitEnvironment(entry); ok {
			merged[key] = value
		}
	}
	for key, value := range overrides {
		merged[key] = value
	}
	environment := make([]string, 0, len(merged))
	for key, value := range merged {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func splitEnvironment(entry string) (string, string, bool) {
	for index := 0; index < len(entry); index++ {
		if entry[index] == '=' {
			return entry[:index], entry[index+1:], true
		}
	}
	return "", "", false
}

func (process *runningProcess) PID() int {
	return process.command.Process.Pid
}

func (process *runningProcess) Wait() (int, error) {
	err := process.command.Wait()
	if process.command.ProcessState != nil {
		return process.command.ProcessState.ExitCode(), err
	}
	return -1, err
}

func (process *runningProcess) Stop() error {
	if runtime.GOOS == "windows" {
		return process.command.Process.Kill()
	}
	return process.command.Process.Signal(os.Interrupt)
}

func (process *runningProcess) Kill() error {
	return process.command.Process.Kill()
}
