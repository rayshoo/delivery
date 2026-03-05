package command

import (
	"bytes"
	"context"
	"delivery/internal/server/env"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// command 는 프로그램을 실행하고 결과를 반환하는 함수 입니다.
func command(ctx context.Context, stdin *bytes.Buffer, directory *string, envVars map[string]string, name string, args ...string) (*string, *string, *int) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdoutBuilder, stderrBuilder := new(strings.Builder), new(strings.Builder)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	if directory != nil {
		cmd.Dir = *directory
	}
	if envVars != nil {
		cmd.Env = os.Environ()
		for k, v := range envVars {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.Stdout = stdoutBuilder
	cmd.Stderr = stderrBuilder
	if err := cmd.Start(); err != nil {
		stdout := ""
		stderr := err.Error()
		rc := 1
		return &stdout, &stderr, &rc
	}
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stdout := stdoutBuilder.String()
			stderr := stderrBuilder.String()
			rc := exitErr.ExitCode()
			return &stdout, &stderr, &rc
		}
	}
	stdout := stdoutBuilder.String()
	stderr := stderrBuilder.String()
	rc := 0
	return &stdout, &stderr, &rc
}

func checkResult(stdout, stderr *string, rc *int, errMsg string) error {
	if log.Level > 5 {
		fmt.Printf("\nStdout:\n%s", *stdout)
		fmt.Printf("\nStderr:\n%s", *stderr)
		fmt.Printf("\nrc: %d\n\n", *rc)
	}
	if rc == nil {
		return fmt.Errorf(errMsg)
	}
	if *rc != 0 && *stderr != "" {
		log.Debugln(*stderr)
		return fmt.Errorf(errMsg)
	}
	return nil
}

func Kustomize(ctx context.Context, args *[]string, path *string) error {
	if path != nil {
		if _, err := os.Stat(*path); err != nil {
			return err
		}
	}
	stdout, stderr, rc := command(ctx, nil, path, nil, env.KustomizePath, *args...)
	return checkResult(stdout, stderr, rc, "kustomize command failed")
}

// isYqLiteral 은 value 가 yq expression 에서 리터럴(숫자, bool, null)로 사용 가능한지 판별합니다.
func isYqLiteral(value string) bool {
	if value == "true" || value == "false" || value == "null" {
		return true
	}
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	return false
}

func PlainUpdate(ctx context.Context, key *string, value *string, file *string) error {
	if file != nil {
		if _, err := os.Stat(*file); err != nil {
			return err
		}
	}

	var expr string
	var envVars map[string]string
	if isYqLiteral(*value) {
		expr = fmt.Sprintf(`%s = %s`, *key, *value)
	} else {
		expr = fmt.Sprintf(`%s = strenv(YQ_VALUE)`, *key)
		envVars = map[string]string{"YQ_VALUE": *value}
	}

	args := []string{"-i", expr, *file}
	stdout, stderr, rc := command(ctx, nil, nil, envVars, env.YQPath, args...)
	if err := checkResult(stdout, stderr, rc, fmt.Sprintf("yq command failed on %s", *file)); err != nil {
		return err
	}

	args = []string{
		"-formatter",
		"indentless_arrays=true",
		*file,
	}
	stdout, stderr, rc = command(ctx, nil, nil, nil, env.YamlfmtPath, args...)
	return checkResult(stdout, stderr, rc, fmt.Sprintf("yamlfmt command failed on %s", *file))
}
