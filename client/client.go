package main

import (
	"context"
	"fmt"
	"log"
	proto "mutual-exclusion/grpc"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const RELEASED = 0
const WANTED = 1
const HELD = 2

type CriticalService struct {
	proto.UnimplementedCriticalServiceServer
	mutex       sync.Mutex
	id          int64
	time        int64
	req_time    int64
	node_amount int64
	state       int64
	node_map    map[int64]proto.CriticalServiceClient
	queue       []int64
	granted     int64
}

func main() {
	server := &CriticalService{
		id:          0,
		time:        0,
		node_amount: 1,
		state:       0,
		node_map:    make(map[int64]proto.CriticalServiceClient),
		queue:       make([]int64, 0),
	}
	server.start_server()
}

// starts the server, and find correct id
func (s *CriticalService) start_server() {
	grpc_server := grpc.NewServer()

	var listener net.Listener
	var err error

	for {
		//as long as it can't connect keep increasing port number by 1
		port := fmt.Sprintf(":%d", 8080+s.id)
		listener, err = net.Listen("tcp", port)
		if err == nil {
			break
		}
		//create clients
		conn, err := grpc.NewClient("localhost"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal(err)
		}
		s.node_map[s.id] = proto.NewCriticalServiceClient(conn)
		s.id++
	}
	// Setup log writing to file
	f, err := os.OpenFile(fmt.Sprintf("log%d.txt", s.id), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	s.node_amount = s.id + 1
	s.time = s.id
	//Inform other nodes about connection
	for _, val := range s.node_map {
		val.UpdateNodeCount(context.Background(), &proto.ConnectionAmount{NodeAmount: s.node_amount})
	}
	fmt.Printf("id: %d\n", s.id)

	//create the server
	proto.RegisterCriticalServiceServer(grpc_server, s)

	go s.process()
	go s.shutdown_logger(grpc_server)
	err = grpc_server.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *CriticalService) process() {
	for {
		s.mutex.Lock()
		s.state = WANTED
		s.time += 1
		s.req_time = s.time
		s.mutex.Unlock()
		log.Printf("%d is requesting access with time %d\n", s.id, s.req_time)
		time.Sleep(200 * time.Millisecond) // Done to simulate operations in CS and to slow down logger
		var requests_sent int64 = 0
		for _, node := range s.node_map {
			s.mutex.Lock()
			s.time += 1
			s.mutex.Unlock()
			_, err := node.RequestAccess(context.Background(), &proto.AccessRequest{Time: s.req_time, Id: s.id})
			// If the request failed, don't increment the counter
			if err == nil {
				requests_sent += 1
			}
		}

		// Await access grants from all sent requests.
		for s.granted < requests_sent {
		}
		// All other nodes have approved access to CS
		s.state = HELD
		log.Printf("%d got access to CS. Request time %d, access time %d\n", s.id, s.req_time, s.time)
		s.mutex.Lock()
		s.state = RELEASED
		s.granted = 0
		log.Printf("%d released access to CS\n", s.id)
		for _, id := range s.queue {
			s.time += 1
			go s.node_map[id].GrantAccess(context.Background(), &proto.AccessGrant{Id: s.id, Time: s.time})
		}
		s.queue = s.queue[:0]
		s.mutex.Unlock()
	}
}

func (s *CriticalService) shutdown_logger(grpc_server *grpc.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	<-stop
	log.Printf("Node stopped with time %d\n", s.time)
	log.Println("---------------------------------------------------------------")
	grpc_server.GracefulStop()
}

func (s *CriticalService) UpdateNodeCount(ctx context.Context, in *proto.ConnectionAmount) (*proto.Empty, error) {
	s.mutex.Lock()
	for i := int64(0); i < in.NodeAmount-s.node_amount; i++ {
		port := fmt.Sprintf(":%d", 8080+s.node_amount+i)
		conn, err := grpc.NewClient("localhost"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal(err)
		}
		s.node_map[s.node_amount+i] = proto.NewCriticalServiceClient(conn)
	}
	//increment own node amount to the correct amount
	s.node_amount = in.NodeAmount //in.NodeAmount -> from proto file even though its node_amount
	s.mutex.Unlock()
	return &proto.Empty{}, nil
}

func (s *CriticalService) RequestAccess(ctx context.Context, in *proto.AccessRequest) (*proto.Empty, error) {
	s.mutex.Lock()
	if in.Time > s.time {
		s.time = in.Time
	}
	s.time += 1
	log.Printf("%d received access request from %d with request time %d and own request time %d", s.id, in.Id, in.Time, s.req_time)
	if s.state == HELD || s.state == WANTED && (s.req_time < in.Time || s.req_time == in.Time && s.id < in.Id) {
		log.Printf("%d is adding %d to queue", s.id, in.Id)
		s.queue = append(s.queue, in.Id)
	} else {
		log.Printf("%d is giving %d access", s.id, in.Id)
		s.time += 1
		go s.node_map[in.Id].GrantAccess(context.Background(), &proto.AccessGrant{Id: s.id, Time: s.time})
	}
	s.mutex.Unlock()
	return &proto.Empty{}, nil
}

func (s *CriticalService) GrantAccess(ctx context.Context, in *proto.AccessGrant) (*proto.Empty, error) {
	log.Printf("%d access allowed by %d", s.id, in.Id)
	s.mutex.Lock()
	if in.Time > s.time {
		s.time = in.Time
	}
	s.time += 1
	s.granted += 1
	s.mutex.Unlock()
	return &proto.Empty{}, nil
}
