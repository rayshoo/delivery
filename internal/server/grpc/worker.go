package grpc

import (
	pb "delivery/api/gen"
	"delivery/internal/server/command"
	"delivery/internal/server/env"
	s "delivery/internal/server/grpc/stream"
	r "delivery/internal/server/repo"
	"fmt"
	"math"
	"path/filepath"
	"strconv"

	"github.com/go-git/go-git/v5/plumbing"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var Worker chan work

func init() {
	Worker = make(chan work, 1)
	go working()
}

// working 은 worker 채널에 들어온 work 를 순차적으로 처리하고 결과를 리턴 해주는 함수입니다.
func working() {
	for {
		w := <-Worker
		var complete bool

		complete = deploy(w.stream, w.commitSpec, w.specs)

		*w.ch <- complete
		close(*w.ch)
	}
}

// deploy 는 specs 의 값에 따라 kubernetes manifest 를 관리하는 repository 의 manifest 이미지 이름 혹은 태그를 변경, 커밋, 푸시하고 결과를 반환하는 함수입니다.
func deploy(stream s.Stream, commitSpec *pb.CommitSpec, specs []*pb.DeploySpec) bool {
	ctx := stream.Context()

	complete := true
	for _, spec := range specs {
		repoUrl := spec.GetUrl()
		s.LoggingAndSendMessage(stream, fmt.Sprintf("try updating the %s repository", repoUrl), "info")
		repo := r.GetRepo(&spec.Url)
		if repo == nil {
			complete = false
			s.LoggingAndSendMessage(stream, fmt.Sprintf("no matching '%s' repo was found", repoUrl), "error")
			s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' repo", repoUrl), "warn")
			continue
		}
		if err := repo.FetchRepo(); err != nil {
			complete = false
			log.Errorln(err.Error())
			s.LoggingAndSendMessage(stream, fmt.Sprintf("failed to fetch '%s' repo", repoUrl), "error")
			s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' repo", repoUrl), "warn")
			continue
		}
		if spec.Updates == nil {
			continue
		}
	Branch:
		for _, update := range spec.Updates {
			branch := update.GetBranch()
			s.LoggingAndSendMessage(stream, fmt.Sprintf("try updating to the %s branch of the %s repository.", branch, repoUrl), "info")
			worktree, err := repo.GetWorktree()
			if err != nil {
				complete = false
				log.Errorln(err.Error())
				s.LoggingAndSendMessage(stream, "failed to get worktree", "error")
				s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
				continue
			}
			err = repo.CheckoutRepo(worktree, update.Branch)
			if err != nil {
				complete = false
				log.Errorln(err.Error())
				s.LoggingAndSendMessage(stream, fmt.Sprintf("failed checkout to '%s' branch", branch), "error")
				s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
				continue
			}
			var totalStatusCount int
			for _, p := range update.Paths {
				if p.Kustomize != nil {
					for _, img := range p.Kustomize.Images {
						updateSpec := fmt.Sprintf("%s=%s:%s", img.Name, img.GetNewName(), img.GetNewTag())
						args := []string{"edit", "set", "image", updateSpec}
						path := repo.GetRealPath(&p.Path)
						s.LoggingAndSendMessage(stream, "try updating the kustomization.yaml file", "info")
						s.LoggingAndSendMessage(stream, fmt.Sprintf("path: %s, update: %s", filepath.Join(*path, "kustomization.yaml"), updateSpec), "info")
						err = command.Kustomize(ctx, &args, path)
						if err != nil {
							complete = false
							log.Errorln(err.Error())
							s.LoggingAndSendMessage(stream, fmt.Sprintf("the kustomize command to modify the %s file failed.", filepath.Join(*path, "kustomization.yaml")), "error")
							s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
							continue Branch
						}
						s.LoggingAndSendMessage(stream, fmt.Sprintf("the kustomize command to modify the %s image was successful", img.Name), "info")
					}
				}
				if p.Yq != nil {
					for _, yq := range p.Yq {
						file := filepath.Join(*repo.GetRealPath(&p.Path), yq.File)
						value, kind, err := yqValue(yq)
						if err != nil {
							complete = false
							log.Errorln(err.Error())
							s.LoggingAndSendMessage(stream, fmt.Sprintf("the yq value for %s is not usable: %s", file, err.Error()), "error")
							s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
							continue Branch
						}
						s.LoggingAndSendMessage(stream, fmt.Sprintf("try updating the %s file", file), "info")
						s.LoggingAndSendMessage(stream, fmt.Sprintf("path: %s, update: %s=%s", file, yq.Key, value), "info")
						err = command.PlainUpdate(ctx, &yq.Key, &value, kind, &file)
						if err != nil {
							complete = false
							log.Errorln(err.Error())
							s.LoggingAndSendMessage(stream, fmt.Sprintf("the yq command to modify the %s failed", file), "error")
							s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
							continue Branch
						}
						s.LoggingAndSendMessage(stream, fmt.Sprintf("the yq command to modify the %s was successful", file), "info")
					}
				}
				_, err = worktree.Add(p.Path)
				if err != nil {
					complete = false
					log.Errorln(err.Error())
					s.LoggingAndSendMessage(stream, fmt.Sprintf("failed to add changes on %s branch stage", branch), "error")
					s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
					continue Branch
				}
				status, err := worktree.Status()
				if err != nil {
					complete = false
					log.Errorln(err.Error())
					s.LoggingAndSendMessage(stream, fmt.Sprintf("failed to get %s branch status", branch), "error")
					s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
					continue Branch
				}
				totalStatusCount = totalStatusCount + len(status)
			}
			var commitHash *plumbing.Hash
			if totalStatusCount != 0 || env.AllowEmptyCommit {
				var name, email *string
				if isValidString(commitSpec.CommitUserName) {
					name = commitSpec.CommitUserName
				} else {
					name = &env.DefaultCommitUserName
				}
				if isValidString(commitSpec.CommitUserEmail) {
					email = commitSpec.CommitUserEmail
				} else {
					email = &env.DefaultCommitUserEmail
				}
				commitHash, err = repo.CommitRepo(worktree, name, email, &commitSpec.CommitMessage)
			}
			if err != nil {
				complete = false
				log.Errorln(err.Error())
				s.LoggingAndSendMessage(stream, fmt.Sprintf("failed to commit changes to branch %s", branch), "error")
				s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
				continue
			}
			err = repo.PushRepo(commitHash, update.Branch)
			if err != nil {
				complete = false
				log.Errorln(err.Error())
				s.LoggingAndSendMessage(stream, fmt.Sprintf("failed to push commit to branch %s", branch), "error")
				s.LoggingAndSendMessage(stream, fmt.Sprintf("skip the current '%s' branch", branch), "warn")
				continue
			}
			s.LoggingAndSendMessage(stream, fmt.Sprintf("the push to commit updates to branch %s in repository %s was successful", branch, repoUrl), "info")
		}
	}
	return complete
}

func isValidString(p *string) bool {
	if p == nil || *p == "" {
		return false
	}
	return true
}

// yqValue 는 Yq 요청에서 대입할 값과 그 해석 방식을 꺼냅니다.
//
// 보내는 쪽이 google.protobuf.Value 로 타입을 명시하므로, 서버는 값의
// 생김새를 추측하지 않습니다.
func yqValue(y *pb.Yq) (string, command.ValueKind, error) {
	v := y.GetValue()
	if v == nil {
		return "", command.KindString, fmt.Errorf("value of %s is empty", y.GetKey())
	}
	switch k := v.GetKind().(type) {
	case *structpb.Value_StringValue:
		return k.StringValue, command.KindString, nil
	case *structpb.Value_NumberValue:
		// 정수로 떨어지면 1.0 이 아니라 1 로 씁니다.
		if !math.IsInf(k.NumberValue, 0) && k.NumberValue == math.Trunc(k.NumberValue) {
			return strconv.FormatInt(int64(k.NumberValue), 10), command.KindLiteral, nil
		}
		return strconv.FormatFloat(k.NumberValue, 'f', -1, 64), command.KindLiteral, nil
	case *structpb.Value_BoolValue:
		return strconv.FormatBool(k.BoolValue), command.KindLiteral, nil
	case *structpb.Value_NullValue:
		return "null", command.KindLiteral, nil
	case *structpb.Value_StructValue, *structpb.Value_ListValue:
		b, err := protojson.Marshal(v)
		if err != nil {
			return "", command.KindString, err
		}
		return string(b), command.KindJSON, nil
	default:
		return "", command.KindString, fmt.Errorf("unknown value kind for %s", y.GetKey())
	}
}
