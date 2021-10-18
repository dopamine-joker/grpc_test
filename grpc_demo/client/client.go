package client

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/resolver"
	"google.golang.org/protobuf/types/known/emptypb"
	pb "grpc_test/grpc_demo/user"
	"io"
	"log"
)

var (
	etcdAddrs = []string{"127.0.0.1:49166", "127.0.0.1:49168", "127.0.0.1:49170"}
)

func getUser(client pb.UserServiceClient, userId int32) *pb.User {
	user, err := client.GetUser(context.Background(), &pb.GetUserRequest{
		UserId: userId,
	})

	if err != nil {
		log.Fatalf("get user fail, userId: %d, err : %v\n", userId, err)
	}
	return user
}

func getAllUser(client pb.UserServiceClient) []*pb.User {
	var users []*pb.User
	stream, err := client.GetAllUser(context.Background(), &emptypb.Empty{})
	if err != nil {
		log.Fatalf("client.GetAllUser err %v", err)
	}
	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Failed to receive user: %v", err)
		}
		users = append(users, user)
	}
	return users
}

func getUserList(client pb.UserServiceClient, userIds []int32) []*pb.User {
	var users []*pb.User
	stream, err := client.GetUserList(context.Background())
	if err != nil {
		log.Fatalf("client.GetUserList err %v", err)
	}
	go func() {
		for _, id := range userIds {
			request := &pb.GetUserRequest{
				UserId: id,
			}
			if err = stream.Send(request); err != nil {
				log.Fatalf("%v.Send(%v) = %v", stream, request, err)
			}
		}
		if err = stream.CloseSend(); err != nil {
			log.Fatalf("%v.CloseSend err %v", stream, err)
		}
	}()
	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Failed to receive user: %v", err)
		}
		users = append(users, user)
	}
	return users
}

func AddUser(client pb.UserServiceClient, user *pb.User) bool {
	response, err := client.AddUser(context.Background(), user)
	if err != nil {
		log.Fatalf("client.AddUser err %v", err)
	}
	return response.GetResult()
}

func AddUserList(client pb.UserServiceClient, users []*pb.User) bool {
	stream, err := client.AddUserList(context.Background())
	if err != nil {
		log.Fatalf("client.AddUserList err %v", err)
	}
	for _, user := range users {
		if err = stream.Send(user); err != nil {
			log.Fatalf("%v.Send(%v) = %v", stream, user, err)
		}
	}
	response, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("%v.CloseAndRecv err %v", stream, err)
	}
	return response.GetResult()
}

func Start() {
	r, err := NewResolver(etcdAddrs, BasePath, ServerPath, 5, 5)
	if err != nil {
		log.Fatalf("NewResolver err %v\n", err)
	}
	resolver.Register(r)
	conn, err := grpc.Dial(fmt.Sprintf("%s://author/%s/%s", r.Scheme(), BasePath, ServerPath), grpc.WithBalancerName(roundrobin.Name), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("fail to dial: %v\n", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	client := pb.NewUserServiceClient(conn)
	fmt.Println(getUser(client, 1))
	fmt.Println(getUserList(client, []int32{1, 2}))
	fmt.Println(getAllUser(client))
	fmt.Println(AddUser(client, &pb.User{
		UserId: 3,
		Name:   "user3",
	}))
	userList := []*pb.User{{UserId: 4, Name: "user4"}, {UserId: 5, Name: "user5"}}
	fmt.Println(AddUserList(client, userList))
}
