package stream

import (
	pb "delivery/api/gen"
	"fmt"
	"strings"
)

// LoggingAndSendMessage 는 서버 로깅과 grpc stream 응답으로 메세지를 보내는 함수입니다.
// level 인수가 없을 경우, 에러 레벨 로깅을 수행합니다.
func LoggingAndSendMessage(stream Stream, message string, level ...string) {
	if level != nil && len(level) != 0 {
		switch level[0] {
		case "panic":
			log.Panicln(message)
		case "fatal":
			log.Fatalln(message)
		case "debug":
			log.Debugln(message)
		case "error":
			log.Errorln(message)
		case "warn":
			log.Warnln(message)
		case "info":
			log.Infoln(message)
		case "trace":
			log.Traceln(message)
		default:
			log.Errorln(message)
		}
	} else {
		level = []string{"error"}
		log.Errorln(message)
	}
	message = fmt.Sprintf("[%s] %s", strings.ToUpper(level[0]), message)
	if err := stream.Send(&pb.DeployResponse{Message: &message}); err != nil {
		fmt.Println(err.Error())
	}
}
