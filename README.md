# grpc_test
grpc_test例子，学习用

分别service和client部分
为了方便使用，直接`go build .`生成二进制文件
然后使用type选项来指定启动client还是service

使用前先配置etcd集群，用作服务发现
