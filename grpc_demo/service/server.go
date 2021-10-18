package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "grpc_test/grpc_demo/user"
)

var (
	etcdAddrs = []string{"127.0.0.1:49166", "127.0.0.1:49168", "127.0.0.1:49170"}
	srvAddr   = "127.0.0.1"
)

var (
	port = 8899
)

var mu sync.RWMutex

// Users 用户列表
var Users map[int32]*pb.User

type UserService struct {
	pb.UnimplementedUserServiceServer

	Host string
}

func NewUserService() *UserService {
	return &UserService{
		Host: fmt.Sprintf("%s:%d", srvAddr, port),
	}
}

// GetUser 获取一个用户
func (u *UserService) GetUser(ctx context.Context, request *pb.GetUserRequest) (*pb.User, error) {
	mu.Lock()
	defer mu.Unlock()
	if user, ok := Users[request.GetUserId()]; ok {
		return user, nil
	}
	return nil, errors.New("get nil")
}

// GetAllUser 获取所有用户
func (u *UserService) GetAllUser(empty *emptypb.Empty, stream pb.UserService_GetAllUserServer) error {
	mu.Lock()
	defer mu.Unlock()
	for _, user := range Users {
		if err := stream.Send(user); err != nil {
			return err
		}
	}
	return nil
}

// GetUserList 批量获取用户
func (u *UserService) GetUserList(stream pb.UserService_GetUserListServer) error {
	mu.Lock()
	defer mu.Unlock()
	for {
		request, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if err = stream.Send(Users[request.GetUserId()]); err != nil {
			return err
		}
	}
}

// AddUser 添加一个用户
func (u *UserService) AddUser(ctx context.Context, user *pb.User) (*pb.AddUserResponse, error) {
	mu.Lock()
	defer mu.Unlock()
	Users[user.GetUserId()] = user
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
	mu.Lock()
	for _, user := range users {
		Users[user.GetUserId()] = user
	}
	mu.Unlock()
	return stream.SendAndClose(&pb.AddUserResponse{
		Result: true,
	})
}

func init() {
	Users = map[int32]*pb.User{
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
	}
}

func Start() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	grpcServer := grpc.NewServer()
	userService := NewUserService()
	pb.RegisterUserServiceServer(grpcServer, NewUserService())
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}
	etcdRegister, err := NewRegister(etcdAddrs, BasePath, ServerPath, 5)
	if err != nil {
		log.Fatalf("failed to new etcdRegister: %v\n", err)
	}
	if err = etcdRegister.Register(context.Background(), userService, 5); err != nil {
		log.Fatalf("failed to register userService: %v\n", err)
	}
	log.Printf("server listening at %v", lis.Addr())
	go func() {
		<-ctx.Done()
		log.Println("server exit")
		etcdRegister.StopServe()
		grpcServer.Stop()
	}()
	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalln("grpcServer serve fail")
	}
	time.Sleep(1 * time.Second)
}
