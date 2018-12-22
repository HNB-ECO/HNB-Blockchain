package grpc

import (
	"HNB/config"
	"HNB/logging"
	"google.golang.org/grpc"
	"HNB/access/grpc/proto"
	"net"
	"fmt"
	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"golang.org/x/net/context"
)


var GRPCLog logging.LogModule

const (
	LOGTABLE_GRPC string = "grpc"
)

func GetTLSServerConfig(keyPath string,certPath string) ([]grpc.ServerOption, error) {
	GRPCLog.Infof(LOGTABLE_GRPC, "keyPath:%s, certPath:%s", keyPath, certPath)
	cert, err := tls.LoadX509KeyPair(certPath,
		keyPath)
	if err != nil {
		GRPCLog.Errorf(LOGTABLE_GRPC, "get consensus key and cert error:[%v]", err)
		return nil, err
	}


	cf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,}
	creds := credentials.NewTLS(cf)

	opts := []grpc.ServerOption{grpc.Creds(creds)}
	return opts, nil
}

func StartGRPCServer(){
	//port := strconv.FormatUint(uint64(config.Config.GPRCPort), 10)

	GRPCLog = logging.GetLogIns()
	GRPCLog.Infof(LOGTABLE_GRPC, "grpc start port:%s " ,config.Config.GPRCPort)

	var opts []grpc.ServerOption
	opts = append(opts, grpc.MaxMsgSize(200*1024*1024))
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Config.GPRCPort))
	if err != nil {
		panic(err)
	}

	if config.Config.IsServerTLS == true{
		opts, err = GetTLSServerConfig(config.Config.TlsKeyPath, config.Config.TlsCertPath)
		if err != nil {
			panic(err)
		}
	}

	grpcServer := grpc.NewServer(opts...)

	proto.RegisterMsgGRPCServer(grpcServer, &serverHandle{})
	go func(){
		err = grpcServer.Serve(lis)
		if err != nil{
			panic(err)
		}
	}()
}

type serverHandle struct {

}

func (s *serverHandle)Chat(ctx context.Context, mr *proto.MsgReq) (*proto.MsgRes, error){
	//开始处理msg信息
	fmt.Printf("recv msg:%v\n", mr.Msg)

	return &proto.MsgRes{}, nil
}

