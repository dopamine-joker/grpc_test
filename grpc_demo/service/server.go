package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	pb "grpc_test/grpc_demo/user"
)

var (
	port = 8899
)

type UserService struct {
	pb.UnimplementedUserServiceServer

	mu sync.RWMutex

	// 用户列表
	Users map[int32]*pb.User
}

// GetUser 获取一个用户
func (u *UserService) GetUser(ctx context.Context, request *pb.GetUserRequest) (*pb.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if user, ok := u.Users[request.GetUserId()]; ok {
		return user, nil
	}
	return nil, errors.New("get nil")
}

// GetAllUser 获取所有用户
func (u *UserService) GetAllUser(empty *emptypb.Empty, stream pb.UserService_GetAllUserServer) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, user := range u.Users {
		if err := stream.Send(user); err != nil {
			return err
		}
	}
	return nil
}

// GetUserList 批量获取用户
func (u *UserService) GetUserList(stream pb.UserService_GetUserListServer) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	for {
		request, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err = stream.Send(u.Users[request.GetUserId()]); err != nil {
			return err
		}
	}
}

// AddUser 添加一个用户
func (u *UserService) AddUser(ctx context.Context, user *pb.User) (*pb.AddUserResponse, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Users[user.GetUserId()] = user
	return &pb.AddUserResponse{
		Result: true,
	}, nil
}

// AddUserList 批量添加用户
func (u *UserService) AddUserList(stream pb.UserService_AddUserListServer) error {
	var users []*pb.User
	for {
		user, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		users = append(users, user)
	}
	u.mu.Lock()
	for _, user := range users {
		u.Users[user.GetUserId()] = user
	}
	u.mu.Unlock()
	return stream.SendAndClose(&pb.AddUserResponse{
		Result: true,
	})
}

func main() {
	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, &UserService{
		Users: map[int32]*pb.User{
			1: {
				UserId: 1,
				Name:   "zhangsan",
			},
			2: {
				UserId: 2,
				Name:   "lisi",
			},
			3: {
				UserId: 3,
				Name:   "wangwu",
			},
		},
	})
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}
	log.Printf("server listening at %v", lis.Addr())
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalln("grpcServer serve fail")
	}
}
