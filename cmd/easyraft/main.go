package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bingoohuang/easyraft"
	"github.com/bingoohuang/easyraft/discovery"
	"github.com/bingoohuang/easyraft/fsm"
	"github.com/gin-gonic/gin"
)

func GetEnvInt(name string, defaultValue int) int {
	if s := os.Getenv(name); s != "" {
		if a, err := strconv.Atoi(s); err != nil {
			return defaultValue
		} else {
			return a
		}
	}

	return defaultValue
}

func createDiscoveryMethod() discovery.DiscoveryMethod {
	switch s := os.Getenv("ER_DISCOVERY"); strings.ToLower(s) {
	case "k8s":
		return discovery.NewKubernetesDiscovery("", nil, "")
	case "mdns", "":
		return discovery.NewMDNSDiscovery()
	default:
		return discovery.NewStaticDiscovery(strings.Split(s, ","))
	}
}

func RandPort(defaultPort int) int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return defaultPort
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func IsPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", +port))
	if err != nil {
		return false
	}

	ln.Close()
	return true
}

func FindFreePort(port int) int {
	if IsPortFree(port) {
		return port
	}

	return RandPort(port)
}

func main() {
	// raft details
	raftPort := GetEnvInt("EASYRAFT_PORT", RandPort(15000))
	discoveryPort := FindFreePort(GetEnvInt("EASYRAFT_DISCOVERY_PORT", raftPort+1))
	httpPort := FindFreePort(GetEnvInt("EASYRAFT_HTTP_PORT", discoveryPort+1))
	discoveryMethod := createDiscoveryMethod()
	fsmServices := []fsm.FSMService{fsm.NewInMemoryMapService()}

	log.Printf("Start new raft node with raftPort:%d, discoveryPort:%d, httpPort:%d, discoveryMethod:%s",
		raftPort, discoveryPort, httpPort, discoveryMethod.Name())
	// EasyRaft Node
	node, err := easyraft.NewNode(raftPort, discoveryPort, fsmServices, discoveryMethod)
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	stoppedCh, err := node.Start()
	if err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}
	defer node.Stop()

	go func() {
		// http server
		r := gin.Default()
		r.GET("/raft", createRaftHandler(node))
		r.POST("/put", createPutHandler(node))
		r.GET("/get", createGetHandler(node))

		if err := r.Run(fmt.Sprintf(":%d", httpPort)); err != nil {
			log.Fatalf("failed to run %d, error: %v", httpPort, err)
		}
	}()

	// wait for interruption/termination
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigs
		done <- true
	}()
	<-done
	<-stoppedCh
}

type RaftNode struct {
	ServerID  string
	Address   string
	RaftState string
}

func createRaftHandler(node *easyraft.Node) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		var result []RaftNode
		for _, server := range node.Raft.GetConfiguration().Configuration().Servers {
			if rsp, err := easyraft.GetPeerDetails(string(server.Address)); err != nil {
				log.Printf("GetPeerDetails error: %v", err)
			} else {
				result = append(result, RaftNode{
					ServerID:  rsp.ServerId,
					Address:   string(server.Address),
					RaftState: rsp.RaftState,
				})
			}
		}
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"Nodes":     result,
			"Discovery": node.DiscoveryMethod.Name(),
		})
	}
}
func createPutHandler(node *easyraft.Node) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		mapName, _ := ctx.GetQuery("map")
		key, _ := ctx.GetQuery("key")
		value, _ := ctx.GetQuery("value")
		result, err := node.RaftApply(fsm.MapPutRequest{MapName: mapName, Key: key, Value: value}, time.Second)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.JSON(http.StatusOK, result)
	}
}

func createGetHandler(node *easyraft.Node) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		mapName, _ := ctx.GetQuery("map")
		key, _ := ctx.GetQuery("key")
		result, err := node.RaftApply(fsm.MapGetRequest{MapName: mapName, Key: key}, time.Second)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		ctx.JSON(http.StatusOK, result)
	}
}
