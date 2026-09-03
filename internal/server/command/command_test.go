package command

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"delivery/internal/server/env"
)

// 여러 줄 값을 expression 에 그대로 이어 붙이면 yq 렉서가 깨진다.
// (`1:23: lexer: invalid input text "op: replace\n  pa..."`)
// kustomization.yaml 의 patches[].patch 처럼 JSON6902 블록을 통째로 넣는
// 경우가 실제로 있어 회귀를 막는다.
func TestPlainUpdateMultilineValue(t *testing.T) {
	yq, err := exec.LookPath("yq")
	if err != nil {
		t.Skip("yq 없음")
	}
	yamlfmt, err := exec.LookPath("yamlfmt")
	if err != nil {
		t.Skip("yamlfmt 없음")
	}
	env.YQPath, env.YamlfmtPath = yq, yamlfmt

	file := filepath.Join(t.TempDir(), "kustomization.yaml")
	if err := os.WriteFile(file, []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
patches:
- patch: |-
    - op: replace
      path: /spec/replicas
      value: 1
  target:
    kind: Deployment
    name: inspection
`), 0o600); err != nil {
		t.Fatal(err)
	}

	key := ".patches[0].patch"
	value := "- op: replace\n  path: /spec/replicas\n  value: 2\n" +
		"- op: add\n  path: /spec/template/spec/containers/0/env/-\n  value:\n    name: TIME_KO\n    value: 점검 시간"

	if err := PlainUpdate(context.Background(), &key, &value, KindString, &file); err != nil {
		t.Fatalf("PlainUpdate 실패: %v", err)
	}

	out, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{"value: 2", "TIME_KO", "점검 시간", "name: inspection"} {
		if !strings.Contains(got, want) {
			t.Errorf("결과에 %q 가 없다:\n%s", want, got)
		}
	}
}

// 숫자 리터럴은 expression 에 그대로 들어가 문자열이 아닌 값으로 대입된다.
func TestPlainUpdateLiteralStaysNumber(t *testing.T) {
	yq, err := exec.LookPath("yq")
	if err != nil {
		t.Skip("yq 없음")
	}
	yamlfmt, err := exec.LookPath("yamlfmt")
	if err != nil {
		t.Skip("yamlfmt 없음")
	}
	env.YQPath, env.YamlfmtPath = yq, yamlfmt

	file := filepath.Join(t.TempDir(), "deploy.yaml")
	if err := os.WriteFile(file, []byte("spec:\n  replicas: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	key, value := ".spec.replicas", "3"
	if err := PlainUpdate(context.Background(), &key, &value, KindLiteral, &file); err != nil {
		t.Fatalf("PlainUpdate 실패: %v", err)
	}

	out, _ := os.ReadFile(file)
	if !strings.Contains(string(out), "replicas: 3") {
		t.Errorf("숫자로 대입되지 않았다:\n%s", out)
	}
}

// 타입을 명시하면 값의 생김새와 무관하게 그대로 대입돼야 한다.
// "2" 를 이미지 태그로 쓰는 경우처럼, 숫자로 보이지만 문자열이어야 하는
// 값이 있다. 예전에는 서버가 생김새로 추측해서 이걸 숫자로 만들었다.
func TestPlainUpdateKinds(t *testing.T) {
	yq, err := exec.LookPath("yq")
	if err != nil {
		t.Skip("yq 없음")
	}
	yamlfmt, err := exec.LookPath("yamlfmt")
	if err != nil {
		t.Skip("yamlfmt 없음")
	}
	env.YQPath, env.YamlfmtPath = yq, yamlfmt

	cases := []struct {
		name  string
		kind  ValueKind
		value string
		want  string
	}{
		{"숫자꼴 문자열은 문자열로", KindString, "2", `tag: "2"`},
		{"bool 꼴 문자열도 문자열로", KindString, "true", `tag: "true"`},
		{"공백이 든 문자열", KindString, "a b c", "tag: a b c"},
		{"숫자 리터럴", KindLiteral, "3", "tag: 3"},
		{"bool 리터럴", KindLiteral, "true", "tag: true"},
		{"null 리터럴", KindLiteral, "null", "tag: null"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "v.yaml")
			if err := os.WriteFile(file, []byte("tag: seed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			key := ".tag"
			if err := PlainUpdate(context.Background(), &key, &c.value, c.kind, &file); err != nil {
				t.Fatalf("실패: %v", err)
			}
			out, _ := os.ReadFile(file)
			if !strings.Contains(string(out), c.want) {
				t.Errorf("%q 를 기대했으나:\n%s", c.want, out)
			}
		})
	}
}

// 구조 값은 JSON 으로 받아 블록 스타일 매핑으로 들어가야 한다.
// from_json 은 flow 스타일로 넣으므로 하위 노드까지 되돌리는지 본다.
func TestPlainUpdateKindJSON(t *testing.T) {
	yq, err := exec.LookPath("yq")
	if err != nil {
		t.Skip("yq 없음")
	}
	yamlfmt, err := exec.LookPath("yamlfmt")
	if err != nil {
		t.Skip("yamlfmt 없음")
	}
	env.YQPath, env.YamlfmtPath = yq, yamlfmt

	file := filepath.Join(t.TempDir(), "s.yaml")
	if err := os.WriteFile(file, []byte("spec: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	key := ".spec.env"
	value := `[{"name":"TIME_KO","value":"점검 시간"},{"name":"PORT","value":"8080"}]`
	if err := PlainUpdate(context.Background(), &key, &value, KindJSON, &file); err != nil {
		t.Fatalf("실패: %v", err)
	}
	out, _ := os.ReadFile(file)
	got := string(out)
	for _, want := range []string{"TIME_KO", "점검 시간", "PORT"} {
		if !strings.Contains(got, want) {
			t.Errorf("결과에 %q 가 없다:\n%s", want, got)
		}
	}
	if strings.Contains(got, "[{") {
		t.Errorf("flow 스타일로 들어갔다:\n%s", got)
	}
}
