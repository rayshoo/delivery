package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"delivery/internal/client/env"
	"delivery/internal/client/notify"
	"delivery/internal/client/slack"
	pb "delivery/api/gen"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc/credentials"

	"gopkg.in/yaml.v3"

	"delivery/internal/logger"

	"google.golang.org/grpc"
)

type stream interface {
	Recv() (*pb.DeployResponse, error)
	grpc.ClientStream
}

var version string
var log = logger.New(env.LogLevel)

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v" {
			fmt.Printf("deploy client version: %s\n", version)
			return
		}
	}

	timeout, err := strconv.Atoi(env.Timeout)
	if err != nil {
		log.Fatalln("DELIVERY_TIMEOUT must be of numeric type")
	}
	project, branch, tag, commit := env.CommitProject, env.CommitBranch, env.CommitTag, env.CommitShortSha
	if project == "" || (branch == "" && tag == "") || commit == "" {
		log.Fatalln("DELIVERY_COMMIT_PROJECT, DELIVERY_COMMIT_BRANCH, DELIVERY_COMMIT_SHORT_SHA||DELIVERY_COMMIT_TAG env required")
	}

	specs := parseDeploySpecs()
	validateSpecs(specs)
	for i := range specs {
		log.Debugf("spec[%d]: %s", i, specs[i])
	}

	notifySpec, wg := parseNotifySpec()

	commitMessage := buildCommitMessage(project, branch, tag, commit)
	data := pb.DeployRequest{DeploySpecs: specs, CommitSpec: &pb.CommitSpec{CommitMessage: commitMessage}}
	if env.CommitUserEmail != "" {
		data.CommitSpec.CommitUserEmail = &env.CommitUserEmail
	}
	if env.CommitUserName != "" {
		data.CommitSpec.CommitMessage = fmt.Sprintf("%s\n- User: %s", commitMessage, env.CommitUserName)
		data.CommitSpec.CommitUserName = &env.CommitUserName
	}

	conn := dialServer(timeout)
	defer func() { _ = conn.Close() }()

	rpcCtx, rpcCancel := context.WithCancel(context.Background())
	defer rpcCancel()

	c := pb.NewDeployClient(conn)
	var s stream
	s, err = c.Deploy(rpcCtx, &data)
	if err != nil {
		log.Fatalln(err.Error())
	}

	for {
		result, err := s.Recv()
		if err != nil {
			if err == io.EOF {
				if env.NotifySpecFile != "" || env.NotifySpec != "" {
					for i := range notifySpec.Notify.Slack {
						wg.Add(1)
						go func(i int) {
							defer wg.Done()
							err := slack.SendMessage(&notifySpec.Notify.Slack[i].Url, &notifySpec.Notify.Slack[i].Token, &notifySpec.Notify.Slack[i].Data)
							if err != nil {
								log.Errorln(err.Error())
								log.Errorln("failed to send slack notify")
							}
						}(i)
					}
				}
				break
			} else {
				log.Fatalln(err.Error())
			}
		}
		fmt.Println(result.GetMessage())
	}
	wg.Wait()
}

func unmarshalJSONOrYAML(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		log.Traceln(err.Error())
		log.Warnln("not a json file. considers it a yaml file and attempts to convert")
		var yamlContent interface{}
		if err := yaml.Unmarshal(data, &yamlContent); err != nil {
			return fmt.Errorf("failed to unmarshal as yaml: %w", err)
		}
		jsonContent, err := json.Marshal(yamlContent)
		if err != nil {
			return fmt.Errorf("failed to convert yaml to json: %w", err)
		}
		if err := json.Unmarshal(jsonContent, v); err != nil {
			return fmt.Errorf("failed to parse converted json: %w", err)
		}
	}
	return nil
}

func readSpecFile(filePath string) []byte {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Errorln(err.Error())
		log.Fatalf("the %s provided is malformed", filePath)
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		log.Errorln(err.Error())
		log.Fatalf("the %s cannot be read", filePath)
	}
	return content
}

func parseDeploySpecs() []*pb.DeploySpec {
	var specs []*pb.DeploySpec
	if env.SpecsFile != "" {
		content := readSpecFile(env.SpecsFile)
		if err := unmarshalJSONOrYAML(content, &specs); err != nil {
			log.Errorln(err.Error())
			log.Fatalln("failed to parse spec file into deploy spec object")
		}
	} else {
		if err := json.Unmarshal([]byte(env.Specs), &specs); err != nil {
			log.Errorln(err.Error())
			log.Fatalln("failed to parse json into deploy spec object")
		}
	}
	return specs
}

func validateSpecs(specs []*pb.DeploySpec) {
	head := "HEAD"
	for i, spec := range specs {
		if spec.GetUrl() == "" {
			log.Fatalf("project url of DELIVERY_SPECS[%d] required", i)
		}
		if spec.Updates == nil {
			continue
		}
		for j, update := range spec.Updates {
			if update.Branch == nil || update.GetBranch() == "" {
				update.Branch = &head
			}
			for k, p := range update.Paths {
				if p.GetPath() == "" {
					log.Fatalf("app path of DELIVERY_SPECS[%d].Updates[%d].Paths[%d].Path required", i, j, k)
				}
				if p.Kustomize != nil {
					for l, img := range p.Kustomize.Images {
						if img.Name == "" {
							log.Fatalf("name of image in DELIVERY_SPECS[%d].Updates[%d].Paths[%d].Kustomize.Images[%d].Name required", i, j, k, l)
						}
					}
				}
				for l, yq := range p.Yq {
					if yq.File == "" || yq.Key == "" || yq.Value == "" {
						log.Fatalf("all file, key, values of DELIVERY_SPECS[%d].Updates[%d].Paths[%d].Yq[%d] required", i, j, k, l)
					}
				}
			}
		}
	}
}

func parseNotifySpec() (notify.Spec, *sync.WaitGroup) {
	var notifySpec notify.Spec
	var wg sync.WaitGroup

	if env.NotifySpecFile == "" && env.NotifySpec == "" {
		return notify.Spec{
			Notify: &notify.Notify{
				Slack: make([]*notify.Slack, 0),
			},
		}, &wg
	}

	notifySpec = notify.Spec{}
	if env.NotifySpecFile != "" {
		content := readSpecFile(env.NotifySpecFile)
		if err := json.Unmarshal(content, &notifySpec); err != nil {
			log.Traceln(err.Error())
			log.Warnln("the notify spec file provided is not a json file. considers it a yaml file and attempts to unmarshal")
			if err = yaml.Unmarshal(content, &notifySpec); err != nil {
				log.Errorln(err.Error())
				log.Fatalln("failed to unmarshal content into yaml format")
			}
		}
	} else {
		if err := json.Unmarshal([]byte(env.NotifySpec), &notifySpec); err != nil {
			log.Errorln(err.Error())
			log.Fatalln("failed to parse json into notifySpec object")
		}
	}

	for i := range notifySpec.Notify.Slack {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			slackData := []byte(notifySpec.Notify.Slack[i].Data)
			var yamlContent interface{}
			if err := yaml.Unmarshal(slackData, &yamlContent); err != nil {
				log.Errorln(err.Error())
				log.Fatalln("failed to convert file to interface object")
			}
			if jsonContent, err := json.Marshal(yamlContent); err != nil {
				log.Errorln(err.Error())
				log.Fatalln("failed to convert interface object to json")
			} else {
				notifySpec.Notify.Slack[i].Data = string(jsonContent)
			}
			log.Debugf("notifySpec[%d]: %s", i, notifySpec.Notify.Slack[i])
		}(i)
	}
	wg.Wait()

	return notifySpec, &wg
}

func buildCommitMessage(project, branch, tag, commit string) string {
	if tag == "" {
		return fmt.Sprintf("%s\n- Repo: %s\n- Branch: %s\n- Commit: %s", env.CommitMessage, project, branch, commit)
	}
	return fmt.Sprintf("%s\n- Repo: %s\n- Tag: %s\n- Commit: %s", env.CommitMessage, project, tag, commit)
}

func dialServer(timeout int) *grpc.ClientConn {
	fullAddr := fmt.Sprintf("%s:%s", env.Addr, env.Port)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer dialCancel()

	if env.RootCert == "" {
		conn, err := grpc.DialContext(dialCtx, fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Panicln(err)
		}
		return conn
	}

	f, err := os.ReadFile(env.RootCert)
	if err != nil {
		log.Errorln(err.Error())
		log.Warnln("the certificate is in an incorrect format. Trying to connect with the insecure option")
		conn, err := grpc.DialContext(dialCtx, fullAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Panicln(err)
		}
		return conn
	}

	p := x509.NewCertPool()
	if !p.AppendCertsFromPEM(f) {
		log.Fatalln("failed to append root CAs")
	}
	tlsConfig := &tls.Config{
		RootCAs: p,
	}
	conn, err := grpc.DialContext(dialCtx, fullAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		log.Panicln(err)
	}
	return conn
}
