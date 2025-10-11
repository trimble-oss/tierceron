package common

import (
	"fmt"
	"strings"
	"time"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	pb "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/trcshtalksdk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// TalkBackFn transforms an incoming diagnostic request into a response (business logic hook).
type TalkBackFn func(*pb.DiagnosticRequest) *pb.DiagnosticResponse

// ProcessFn sends a diagnostic request (optionally broadcast) and returns the response.
type ProcessFn func(req *pb.DiagnosticRequest, broadcast bool) (*pb.DiagnosticResponse, error)

// StartTrashTalking contains the polling / broadcast loop previously in ttcore.
// It is extracted so alternate ttcore implementations can reuse the common behavior.
func StartTrashTalking(ctx *tccore.ConfigContext, shutdownChan chan bool, remoteServerName string, port int, ttbToken *string, isRemote bool, talkBack TalkBackFn, process ProcessFn) {
	if ctx == nil {
		return
	}
	ctx.Log.Printf("Trash talking...\n")

	// Broadcast goroutine
	go func() {
		if ctx.ChatBroadcastChan == nil {
			ctx.Log.Printf("No broadcast channel initialized for trcshtalk.\n")
			return
		}
		for msg := range *ctx.ChatBroadcastChan {
			if msg.Response == nil {
				ctx.Log.Printf("Invalid broadcast message in trcshtalk.\n")
				continue
			}
			req := &pb.DiagnosticRequest{MessageId: "", Data: []string{*msg.Response}}
			retry := 0
		retryBroadcast:
			if _, err := process(req, true); err != nil {
				ctx.Log.Printf("Error sending broadcast response: %s\n", err.Error())
				if retry < 5 {
					retry++
					time.Sleep(3 * time.Second)
					goto retryBroadcast
				}
				return
			}
		}
	}()

	for {
		select {
		case <-shutdownChan:
			return
		default:
		}
		// Pull a query
		emptyReq := &pb.DiagnosticRequest{}
		resp, err := process(emptyReq, false)
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
				time.Sleep(10 * time.Second)
			}
			continue
		}
		queryData := resp.GetResults()
		incoming := &pb.DiagnosticRequest{}
		if err := protojson.Unmarshal([]byte(queryData), incoming); err != nil {
			if !strings.Contains(err.Error(), "unexpected token") {
				ctx.Log.Printf("Error unmarshalling incoming diagnostic request: %s\n", err.Error())
			}
			continue
		}
		// Handle talkback asynchronously
		go func(r *pb.DiagnosticRequest) {
			tb := talkBack(r)
			reply := &pb.DiagnosticRequest{MessageId: r.MessageId, Data: []string{tb.Results}}
			retry := 0
		retryTalkback:
			if _, err := process(reply, false); err != nil {
				ctx.Log.Printf("Error sending talkback response: %s\n", err.Error())
				if retry < 5 {
					retry++
					time.Sleep(3 * time.Second)
					goto retryTalkback
				}
			}
		}(incoming)
	}
}

// TrcshTalkBack contains the original logic for assembling diagnostics into a response.
// Kept here for reuse; callers may supply an alternate implementation.
func TrcshTalkBack(ctx *tccore.ConfigContext, req *pb.DiagnosticRequest) *pb.DiagnosticResponse {
	cmds := req.GetDiagnostics()
	queries := []string{}
	tenantTest := req.GetQueryId() + ":"
	if contains(cmds, pb.Diagnostics_ALL) {
		ctx.Log.Println("Running all queries.")
		queries = append(queries, "healthcheck")
	} else {
		for _, q := range cmds {
			switch q {
			case pb.Diagnostics_HEALTH_CHECK:
				healthData := req.GetData()
				if len(healthData) == 1 && healthData[0] == "PROGRESS" {
					tenantTest = "PROGRESS"
				}
				ctx.Log.Println("Running healthcheck diagnostic.")
				queries = append(queries, "healthcheck")
			case pb.Diagnostics_TRCDB:
				ctx.Log.Println("Running trcdb diagnostic.")
				queries = append(queries, "trcdb")
				for i, r := range req.GetData() {
					if i == 0 && r == "PROGRESS" {
						tenantTest = "PROGRESS"
						break
					}
					if i == 0 {
						tenantTest = fmt.Sprintf("%s%s", tenantTest, r)
					} else {
						tenantTest = fmt.Sprintf("%s,%s", tenantTest, r)
					}
				}
			}
		}
	}
	name := "trcshtalk"
	*ctx.ChatSenderChan <- &tccore.ChatMsg{RoutingId: &req.MessageId, Name: &name, ChatId: &tenantTest, Query: &queries}

	results := ""
	finished := make(map[string]string)
	ctx.Log.Printf("Sent queries to kernel: %d\n", len(queries))
	for {
		event := <-*ctx.ChatReceiverChan
		ctx.Log.Println("TrcshTalk received message from kernel.")
		if len(finished) == len(queries) {
			ctx.Log.Println("Formatting responses.")
			for _, v := range finished {
				results += v + " "
			}
			ctx.Log.Printf("Sending response to chat: %s\n", results)
			return &pb.DiagnosticResponse{MessageId: *event.RoutingId, Results: results}
		}
		if event.Query != nil && len(*event.Query) == 1 && event.Response != nil && (*event).Response != nil {
			ctx.Log.Printf("Processing response from query: %s\n", *event.Query)
			finished[(*event.Query)[0]] = *event.Response
		}
		if len(finished) == len(queries) {
			ctx.Log.Println("Formatting responses.")
			for _, v := range finished {
				results += v + " "
			}
			ctx.Log.Printf("Sending response to chat: %s\n", results)
			return &pb.DiagnosticResponse{MessageId: *event.RoutingId, Results: results}
		}
	}
}

func contains[T comparable](arr []T, v T) bool {
	for _, e := range arr {
		if e == v {
			return true
		}
	}
	return false
}
