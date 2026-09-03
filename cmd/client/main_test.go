package main

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// 설정 파일에 쓴 타입이 그대로 보존되는지 확인한다. 예전에는 값이 전부
// 문자열이라, 따옴표를 값 안에 박아 넣어야 yq 가 문자열로 대입했다.
func TestUnmarshalSpecsPreservesTypes(t *testing.T) {
	content := []byte(`
- url: git@example.com:infra/platform.git
  updates:
  - branch: master
    paths:
    - path: app/prd
      yq:
      - file: kustomization.yaml
        key: .patches[0].patch
        value: |-
          - op: replace
            path: /spec/replicas
            value: 2
      - file: values.yaml
        key: .image.tag
        value: "2"
      - file: values.yaml
        key: .replicas
        value: 2
      - file: values.yaml
        key: .enabled
        value: true
`)
	specs, err := unmarshalSpecs(content)
	if err != nil {
		t.Fatalf("파싱 실패: %v", err)
	}
	yq := specs[0].Updates[0].Paths[0].Yq
	if len(yq) != 4 {
		t.Fatalf("yq 4개를 기대했으나 %d개", len(yq))
	}
	if _, ok := yq[0].GetValue().GetKind().(*structpb.Value_StringValue); !ok {
		t.Errorf("여러 줄 패치가 문자열이 아니다: %T", yq[0].GetValue().GetKind())
	}
	if v := yq[1].GetValue().GetStringValue(); v != "2" {
		t.Errorf(`"2" 가 문자열로 오지 않았다: %v`, yq[1].GetValue())
	}
	if _, ok := yq[2].GetValue().GetKind().(*structpb.Value_NumberValue); !ok {
		t.Errorf("2 가 숫자가 아니다: %T", yq[2].GetValue().GetKind())
	}
	if _, ok := yq[3].GetValue().GetKind().(*structpb.Value_BoolValue); !ok {
		t.Errorf("true 가 bool 이 아니다: %T", yq[3].GetValue().GetKind())
	}
}
