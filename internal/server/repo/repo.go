package repo

import (
	"delivery/internal/server/env"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5/config"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Clone 은 fileSystem 에 repository 를 clone 합니다.
func (r *Repo) clone(cloneOptions *git.CloneOptions) {
	workDirectory := env.WorkDirectory

	cloneDir := filepath.Join(workDirectory, r.Name)
	log.Infof("trying clone %s to %s", r.Url, cloneDir)
	if _, err := os.Stat(cloneDir); err == nil {
		log.Warnf("directory %s is already exists.", cloneDir)
		forceClone := env.ForceClone
		if forceClone {
			log.Warnf("force delete option is enabled. original %s will be destroy", cloneDir)
			if err = os.RemoveAll(cloneDir); err != nil {
				log.Panicln(err.Error())
			}
		} else {
			log.Panicf("can`t overwrite already exist directory %s. try to remove directory manualy or set GSM_FORCE_CLONE=true env", cloneDir)
		}
	}
	cloneOptions.URL = r.Url
	cloneOptions.Auth = r.authMethod
	var err error
	r.Repository, err = git.PlainClone(cloneDir, false, cloneOptions)
	if err != nil {
		log.Panicln(err.Error())
	}
}

// GetRealPath 는 fileSystem 에서의 repo 경로를 반환합니다.
func (r *Repo) GetRealPath(path *string) *string {
	realPath := filepath.Join(env.WorkDirectory, r.Name, *path)
	return &realPath
}

// GetRepo 는 인자로 들어온 url 를 정규화하고 repos 인덱스 값의 parsedUrl 와 일치하는 값을 반환합니다.
func GetRepo(url *string) *Repo {
	parsedUrl := getParsedUrl(*url)
	for i := range repos {
		if repos[i].ParsedUrl == *parsedUrl {
			return repos[i]
		}
	}
	return nil
}

func (r *Repo) FetchRepo() error {
	// git fetch --depth=1
	if err := r.Repository.Fetch(&git.FetchOptions{
		Depth: 1,
		Auth:  r.authMethod,
	}); err != nil {
		if err.Error() == "already up-to-date" {
			log.Warnln("the most recent commit has already been fetched")
			return nil
		}
		return err
	}
	return nil
}

func (r *Repo) GetHash(revision plumbing.Revision) (*plumbing.Hash, error) {
	hash, err := r.Repository.ResolveRevision(revision)
	if err != nil {
		return nil, err
	}
	return hash, nil
}

func (r *Repo) GetWorktree() (*git.Worktree, error) {
	return r.Repository.Worktree()
}

func (r *Repo) CheckoutRepo(worktree *git.Worktree, branch *string) error {
	err := worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("refs/remotes/origin/%s", *branch)),
		Force:  true,
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *Repo) ResetRepo(worktree *git.Worktree, to *plumbing.Hash) error {
	// git reset --hard <to>
	if err := worktree.Reset(&git.ResetOptions{
		Commit: *to,
		Mode:   git.HardReset,
	}); err != nil {
		return fmt.Errorf("failed to reset repository %s", to)
	}
	return nil
}

func (r *Repo) CommitRepo(workTree *git.Worktree, commitUserName *string, commitUserEmail *string, commitMessage *string) (*plumbing.Hash, error) {
	commit, err := workTree.Commit(*commitMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  *commitUserName,
			Email: *commitUserEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, err
	}
	return &commit, nil
}

func (r *Repo) PushRepo(commitHash *plumbing.Hash, remoteBranch *string) error {
	refSpec := config.RefSpec(fmt.Sprintf("%s:refs/heads/%s", commitHash, *remoteBranch))
	err := r.Repository.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
		Auth:     r.authMethod,
	})
	if err != nil {
		return err
	}
	return nil
}
