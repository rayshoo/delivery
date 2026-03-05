package repo

import (
	"delivery/internal/server/env"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"delivery/internal/logger"
)

var log = logger.New(env.LogLevel)

var repos []*Repo

type httpAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
type sshAuth struct {
	PrivateKeyFile string `yaml:"private-key-file"`
	Password       string `yaml:"password"`
}

type Repo struct {
	Name       string `yaml:"name"`
	Url        string `yaml:"url"`
	ParsedUrl  string
	HttpAuth   *httpAuth `yaml:"http,omitempty"`
	SshAuth    *sshAuth  `yaml:"ssh,omitempty"`
	Repository *git.Repository
	authMethod transport.AuthMethod
}
