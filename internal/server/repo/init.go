package repo

import (
	"delivery/internal/server/env"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"gopkg.in/yaml.v3"
)

func init() {
	parse()
	setAuth()
	cloneOptions := &git.CloneOptions{
		Depth:    1,
		Progress: os.Stdout,
	}
	for i := range repos {
		repos[i].clone(cloneOptions)
	}
}

// parse 는 config file 을 파싱해서 repos 슬라이스의 인덱스에 해당하는 값들을 저장하는 함수 입니다.
func parse() {
	filePath, err := filepath.Abs(env.RepoListFilePath)
	if err != nil {
		log.Panicln(err.Error())
	}
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		log.Panicln(err.Error())
	}
	var Repos []*Repo
	err = yaml.Unmarshal(yamlFile, &Repos)
	if err != nil {
		log.Panicln(err.Error())
	}
	repos = make([]*Repo, 0, len(Repos))
	for i := range Repos {
		Repos[i].ParsedUrl = *getParsedUrl(Repos[i].Url)
		var repo *Repo
		repo = Repos[i]
		repos = append(repos, repo)
	}
}

// GetParsedUrl 은 인자로 받은 path 를 정규화하고 반환하는 함수 입니다.
func getParsedUrl(url string) *string {
	pattern, err := regexp.Compile("(?:http://(.*?)/)?(?:https://(.*?)/)?(?:git@(.*):)?(?:(.*).git)?(.*)?")
	if err != nil {
		log.Panicln(err.Error())
	}
	matches := pattern.FindAllStringSubmatch(url, -1)
	var builder strings.Builder
	for i := range matches {
		for j := 1; j < len(matches[i]); j++ {
			if j == 4 {
				_, err := builder.WriteRune('/')
				if err != nil {
					log.Panicf(err.Error())
				}
			}
			_, err := builder.WriteString(matches[i][j])
			if err != nil {
				log.Panicf("invalid git repository address. path: %s", url)
			}
		}
	}
	parsedUrl := builder.String()
	return &parsedUrl
}

// setAuth 는 repository 를 clone 하기 위한 인증 정보를 세팅합니다.
func setAuth() {
	for i := range repos {
		var repo *Repo
		repo = repos[i]
		if !strings.Contains(repo.Url, "git@") {
			if repo.HttpAuth != nil {
				if repo.HttpAuth.Username != "" && repo.HttpAuth.Password != "" {
					repo.authMethod = &http.BasicAuth{
						Username: repo.HttpAuth.Username,
						Password: repo.HttpAuth.Password,
					}
				} else {
					log.Panicf("http auth username and password are both required. repository: %s", repo.Url)
				}
			}
		} else {
			if repo.SshAuth == nil {
				repo.SshAuth = &sshAuth{}
			}
			if repo.SshAuth.PrivateKeyFile == "" {
				if privateKeyFile := env.GetDefaultPrivateKeyFile(); privateKeyFile == "" {
					log.Panicln("couldn't find the ssh private key file to access the git repo")
				} else {
					repo.SshAuth.PrivateKeyFile = privateKeyFile
				}
				log.Infof("privateKeyFile: %s", repo.SshAuth.PrivateKeyFile)
			} else {
				_, err := os.Stat(repo.SshAuth.PrivateKeyFile)
				if err != nil {
					log.Panicln(err.Error())
				}
			}
			var err error
			repo.authMethod, err = ssh.NewPublicKeysFromFile("git", repo.SshAuth.PrivateKeyFile, repo.SshAuth.Password)
			if err != nil {
				log.Panicln(err.Error())
			}
		}
	}
}
