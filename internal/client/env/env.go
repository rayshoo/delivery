package env

import (
	"os"

	"github.com/joho/godotenv"
)

var (
	Addr            string
	Port            string
	RootCert        string
	LogLevel        string
	CommitUserName  string
	CommitUserEmail string
	CommitMessage   string
	CommitProject   string
	CommitBranch    string
	CommitTag       string
	CommitShortSha  string
	Specs           string
	SpecsFile       string
	Timeout         string
	NotifySpec      string
	NotifySpecFile  string
	NotifyTimeout   string
)

func init() {
	if _, err := os.Stat(".client.env"); err == nil {
		err = godotenv.Load(".client.env")
		if err != nil {
			panic(err)
		}
	}
	Addr = os.Getenv("DELIVERY_SERVER_ADDR")
	Port = os.Getenv("DELIVERY_SERVER_PORT")
	RootCert = os.Getenv("DELIVERY_SERVER_ROOT_CERT")
	LogLevel = os.Getenv("DELIVERY_LOG_LEVEL")
	CommitUserName = os.Getenv("DELIVERY_COMMIT_USER_NAME")
	CommitUserEmail = os.Getenv("DELIVERY_COMMIT_USER_EMAIL")
	CommitMessage = os.Getenv("DELIVERY_COMMIT_MESSAGE")
	CommitProject = os.Getenv("DELIVERY_COMMIT_PROJECT")
	CommitBranch = os.Getenv("DELIVERY_COMMIT_BRANCH")
	CommitTag = os.Getenv("DELIVERY_COMMIT_TAG")
	CommitShortSha = os.Getenv("DELIVERY_COMMIT_SHORT_SHA")
	Specs = os.Getenv("DELIVERY_SPECS")
	SpecsFile = os.Getenv("DELIVERY_SPECS_FILE")
	Timeout = os.Getenv("DELIVERY_TIMEOUT")
	NotifySpec = os.Getenv("DELIVERY_NOTIFY_SPEC")
	NotifySpecFile = os.Getenv("DELIVERY_NOTIFY_SPEC_FILE")
	NotifyTimeout = os.Getenv("DELIVERY_NOTIFY_TIMEOUT")
	setDefaultValue()
}

// setDefaultValue 는 환경변수에서 지정되지 않은 변수값의 기본값을 지정합니다.
func setDefaultValue() {
	if Addr == "" {
		Addr = "127.0.0.1"
	}
	if Port == "" {
		Port = "12010"
	}
	if LogLevel == "" {
		LogLevel = "Info"
	}
	if Timeout == "" {
		Timeout = "10"
	}
	if NotifyTimeout == "" {
		NotifyTimeout = "5"
	}
}
