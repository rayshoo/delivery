package grpc

import (
	pb "delivery/api/gen"
	"delivery/internal/server/command"
	"delivery/internal/server/env"
	s "delivery/internal/server/grpc/stream"
	r "delivery/internal/server/repo"
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing"
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
						s.LoggingAndSendMessage(stream, fmt.Sprintf("try updating the %s file", file), "info")
						s.LoggingAndSendMessage(stream, fmt.Sprintf("path: %s, update: %s=%s", file, yq.Key, yq.Value), "info")
						err := command.PlainUpdate(ctx, &yq.Key, &yq.Value, &file)
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
