package process

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"

	"github.com/waxlight/waxlight-launcher/internal/application"
)

type Launcher struct{}

type runningProcess struct {
	command *exec.Cmd
}

func (Launcher) Start(
	ctx context.Context,
	executable string,
	arguments []string,
	workingDirectory string,
	environment map[string]string,
	output io.Writer,
) (application.RunningProcess, error) {
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = workingDirectory
	command.Env = os.Environ()
	for key, value := range environment {
		command.Env = append(command.Env, key+"="+value)
	}
	command.Stdout = output
	command.Stderr = output

	if err := command.Start(); err != nil {
		slog.Warn("failed to start the game process", "error", err)
		return nil, err
	}

	slog.Info("game process started", "pid", command.Process.Pid)
	return &runningProcess{command: command}, nil
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
