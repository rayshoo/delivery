package command

import (
	"bytes"
	"context"
	"delivery/internal/server/env"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// ValueKind 는 yq 에 대입할 값을 어떻게 해석할지 나타냅니다.
//
// 보내는 쪽이 타입을 명시하므로 서버는 값의 생김새를 추측하지 않습니다.
type ValueKind int

const (
	// KindString 은 문자열로 대입합니다. "2" 가 숫자로 바뀌지 않습니다.
	KindString ValueKind = iota
	// KindLiteral 은 숫자·bool·null 로 대입합니다.
	KindLiteral
	// KindJSON 은 값을 JSON 으로 파싱해 구조(map/list)로 대입합니다.
	KindJSON
)

// yqExpr 은 대입 expression 과 함께 넘길 환경변수를 만듭니다.
//
// 값을 expression 에 그대로 이어 붙이면 여러 줄 YAML 이나 공백이 든 값에서
// yq 렉서가 깨집니다. 문자열은 환경변수로 넘겨 strenv 로 읽습니다.
func yqExpr(key, value string, kind ValueKind) (string, map[string]string) {
	switch kind {
	case KindLiteral:
		return fmt.Sprintf(`%s = %s`, key, value), nil
	case KindJSON:
		// from_json 은 flow 스타일로 들어가므로 하위 노드까지 블록으로 되돌립니다.
		return fmt.Sprintf(`%s = (strenv(YQ_VALUE) | from_json) | (%s | ..) style=""`, key, key),
			map[string]string{"YQ_VALUE": value}
	default:
		return fmt.Sprintf(`%s = strenv(YQ_VALUE)`, key), map[string]string{"YQ_VALUE": value}
	}
}

func PlainUpdate(ctx context.Context, key *string, value *string, kind ValueKind, file *string) error {
	if file != nil {
		if _, err := os.Stat(*file); err != nil {
			return err
		}
	}

	expr, envVars := yqExpr(*key, *value, kind)
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
