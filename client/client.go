package main

import (
	"context"
	"fmt"
	"log"
	proto "mutual-exclusion/grpc"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const RELEASED = 0
const WANTED = 1
const HELD = 2

type CriticalService struct {
	proto.UnimplementedCriticalServiceServer
	id          int64
	time        int64
	node_amount int64
	state       int64
	node_map    map[int64]proto.CriticalServiceClient
}

func main() {
	server := &CriticalService{
		id:          0,
		time:        0,
		node_amount: 1,
		state:       0,
		node_map:    make(map[int64]proto.CriticalServiceClient),
	}
	server.start_server()
}

// starts the server, and find correct id
func (s *CriticalService) start_server() {
	grpc_server := grpc.NewServer()

	var listener net.Listener
	var err error

	for {
		//as long as it cant connect
		port := fmt.Sprintf(":%d", 8080+s.id)
		listener, err = net.Listen("tcp", port)
		if err == nil {
			break
		}
		s.id++
		//create clients
		conn, err := grpc.NewClient("localhost"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal(err)
		}
		s.node_map[s.id] = proto.NewCriticalServiceClient(conn)
	}
	s.node_amount = s.id + 1
	//update everything?
	for _, val := range s.node_map {
		val.UpdateNodeCount(context.Background(), &proto.ConnectionAmount{NodeAmount: s.node_amount})
	}
	fmt.Printf("id: %d\n", s.id)

	//creates the server
	proto.RegisterCriticalServiceServer(grpc_server, s)

	err = grpc_server.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *CriticalService) UpdateNodeCount(ctx context.Context, in *proto.ConnectionAmount) (*proto.Empty, error) {
	for i := range in.NodeAmount - s.node_amount {
		port := fmt.Sprintf(":%d", 8080+s.node_amount+i)
		conn, err := grpc.NewClient("localhost"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatal(err)
		}
		s.node_map[s.node_amount+i] = proto.NewCriticalServiceClient(conn)
	}
	//increment own node amount to the correct amount
	s.node_amount = in.NodeAmount //in.NodeAmount -> from proto file even though its node_amount
	return &proto.Empty{}, nil
}

func (s *CriticalService) RequestAccess(ctx context.Context, in *proto.AccessRequest) (*proto.AccessGrant, error) {
	if s.state == HELD || s.state == WANTED && s.time < in.Time {
		for s.state != RELEASED {
			// nada
		}
	}
	return &proto.AccessGrant{}, nil
}
