package env

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"delivery/internal/logger"
	"github.com/sirupsen/logrus"
)

var (
	Addr                   string
	Port                   string
	HTTPPort               string
	RepoListFilePath       string
	defaultPrivateKeyFile  string
	WorkDirectory          string
	LogLevel               string
	KustomizePath          string
	YQPath                 string
	YamlfmtPath            string
	DefaultCommitUserName  string
	DefaultCommitUserEmail string
	ForceClone             bool
	AllowEmptyCommit       bool
)
var log *logrus.Logger

func init() {
	if _, err := os.Stat(".server.env"); err == nil {
		err = godotenv.Load(".server.env")
		if err != nil {
			panic(err)
		}
	}
	Addr = os.Getenv("DELIVERY_ADDR")
	Port = os.Getenv("DELIVERY_PORT")
	HTTPPort = os.Getenv("DELIVERY_HTTP_PORT")
	RepoListFilePath = os.Getenv("DELIVERY_REPO_LIST_FILE_PATH")
	defaultPrivateKeyFile = os.Getenv("DELIVERY_DEFAULT_PRIVATE_KEY_FILE")
	WorkDirectory = os.Getenv("DELIVERY_WORK_DIRECTORY")
	LogLevel = os.Getenv("DELIVERY_LOG_LEVEL")
	log = logger.New(LogLevel)
	KustomizePath = os.Getenv("DELIVERY_KUSTOMIZE_PATH")
	YQPath = os.Getenv("DELIVERY_YQ_PATH")
	YamlfmtPath = os.Getenv("DELIVERY_YAMLFMT_PATH")
	DefaultCommitUserName = os.Getenv("DELIVERY_DEFAULT_COMMIT_USER_NAME")
	DefaultCommitUserEmail = os.Getenv("DELIVERY_DEFAULT_COMMIT_USER_EMAIL")
	if strings.ToLower(os.Getenv("DELIVERY_FORCE_CLONE")) == "true" {
		ForceClone = true
	}
	if strings.ToLower(os.Getenv("DELIVERY_ALLOW_EMPTY_COMMIT")) == "true" {
		AllowEmptyCommit = true
	}
	setDefaultValue()
}

// setDefaultValue 는 환경변수에서 지정되지 않은 변수값의 기본값을 지정합니다.
func setDefaultValue() {
	if Addr == "" {
		Addr = "0.0.0.0"
	}
	if Port == "" {
		Port = "12010"
	}
	if HTTPPort == "" {
		HTTPPort = "12011"
	}
	if RepoListFilePath == "" {
		RepoListFilePath = "list.yaml"
	}
	if KustomizePath == "" {
		KustomizePath = "/usr/bin/kustomize"
	}
	if YQPath == "" {
		YQPath = "/usr/bin/yq"
	}
	if YamlfmtPath == "" {
		YamlfmtPath = "/usr/bin/yamlfmt"
	}
	if DefaultCommitUserName == "" {
		DefaultCommitUserName = "Administrator"
	}
	if DefaultCommitUserEmail == "" {
		DefaultCommitUserEmail = "admin@example.com"
	}
}

// GetDefaultPrivateKeyFile 은 config.yaml 파일의 repo 정보에 ssh.private-key-file 이 없을 경우,
// $HOME/.ssh/id_rsa, $HOME/.ssh/id_25519 순으로 파일이 있는지를 체크하고, 있다면 해당 경로를 리턴합니다.
func GetDefaultPrivateKeyFile() string {
	if defaultPrivateKeyFile == "" {
		rsa := filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		ed25519 := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")

		log.Infoln("ssh.private-key-file is not specified. Try the ~/.ssh/id_rsa file as the default file")
		if _, err := os.Stat(rsa); err == nil {
			log.Infoln("found the ~/.ssh/id_rsa file. Set as the default private key file")
			defaultPrivateKeyFile = rsa
		} else {
			log.Infoln("the ~/.ssh/id_rsa file was not found. Try the ~/.ssh/id_25519 file as the default file")
			if _, err := os.Stat(ed25519); err == nil {
				log.Infoln("found the ~/.ssh/id_ed25519 file. Set as the default file")
				defaultPrivateKeyFile = ed25519
			}
			log.Warnln("the ~/.ssh/id_25519 file was not found")
		}
	}
	return defaultPrivateKeyFile
}
