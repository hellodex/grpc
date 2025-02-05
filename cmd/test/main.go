package main

import (
	"context"
	"crypto/x509"
	"flag"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
	"io"
	"log"
	"os"
	"time"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

var token = ""

func main() {
	tokenPtr := flag.String("token", "", "Token for authentication")
	addressPtr := flag.String("address", "solana-yellowstone-grpc.publicnode.com:443", "gRPC server address")

	flag.Parse()

	token = *tokenPtr
	address := *addressPtr
	log.SetOutput(os.Stdout)
	conn := grpc_connect(address, false)
	defer conn.Close()
	go stdERR()
	go stdAccountIndexes()
	grpc_subscribe(conn)
}

var kacp = keepalive.ClientParameters{
	Time:                10 * time.Second,
	Timeout:             time.Second,
	PermitWithoutStream: true,
}

func grpc_connect(address string, plaintext bool) *grpc.ClientConn {
	var opts []grpc.DialOption
	if plaintext {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		pool, _ := x509.SystemCertPool()
		creds := credentials.NewClientTLSFromCert(pool, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	}

	opts = append(opts, grpc.WithKeepaliveParams(kacp))

	log.Println("Starting grpc client, connecting to", address)
	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	return conn
}

func ptr[T any](v T) *T {
	return &v
}

func grpc_subscribe(conn *grpc.ClientConn) {
	var err error
	client := pb.NewGeyserClient(conn)

	subscription := pb.SubscribeRequest{
		Transactions: map[string]*pb.SubscribeRequestFilterTransactions{
			"txn": {
				Vote:            ptr(false),
				Failed:          ptr(false),
				Signature:       nil,
				AccountInclude:  []string{},
				AccountExclude:  []string{},
				AccountRequired: []string{},
			},
		},
		Commitment: pb.CommitmentLevel_PROCESSED.Enum(),
		Ping:       nil,
	}

	ctx := context.Background()
	if token != "" {
		md := metadata.New(map[string]string{"x-token": token})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	stream, err := client.Subscribe(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}
	err = stream.Send(&subscription)
	if err != nil {
		log.Fatalf("%v", err)
	}
	var i uint = 0
	var lastSlot uint64
	var lastTimestamp int64
	var initialized bool

	for {
		i += 1
		resp, err := stream.Recv()
		currentTimestamp := time.Now().UnixMilli()

		if err == io.EOF {
			return
		}
		if err != nil {
			log.Fatalf("Error occurred in receiving update: %v", err)
		}

		tx := resp.GetTransaction()
		resp.GetTransaction()
		if tx == nil {
			log.Println("Received non-transaction message, continuing...")
			log.Printf("CurrentTimestamp: %v %v", currentTimestamp, resp)
			continue
		}

		currentSlot := tx.GetSlot()

		if !initialized {
			lastSlot = currentSlot
			lastTimestamp = currentTimestamp
			initialized = true
			log.Printf("Initial slot: %v at Timestamp: %v", lastSlot, lastTimestamp)
			continue
		}

		if currentSlot > lastSlot {
			timeDiff := currentTimestamp - lastTimestamp
			log.Printf("New slot: %v, Previous slot: %v, CurrentTimestamp: %v",
				currentSlot,
				lastSlot,
				currentTimestamp)
			log.Printf("Time gap since last : %d ms", timeDiff)

			lastSlot = currentSlot
			lastTimestamp = currentTimestamp
		}

		//if data, err := json.Marshal(tx); err == nil {
		//	msg <- data
		//}
		txsChan <- tx
	}
}

var msg = make(chan []byte, 1024)
var txsChan = make(chan *pb.SubscribeUpdateTransaction)

func stdERR() {
	for d := range msg {
		os.Stderr.WriteString(string(d) + "\n")

	}
}

type RawTransaction struct {
	Slot        uint64
	Transaction *solana.Transaction
	Meta        *rpc.TransactionMeta
	Index       uint32
	BlockTime   time.Time
}

// 输出账户索引超限的交易 (需要请求动态地址表的交易)
func stdAccountIndexes() {
	//solana.PublicKeyFromBytes(message.AccountKeys[0])
	for tx := range txsChan {

		//交易
		tran := tx.Transaction.GetTransaction()

		message := tran.Message
		//交易Tx
		signature := base58.Encode(tran.Signatures[0])
		////最大账户索引
		//accountsLen := len(tran.Message.AccountKeys) - 1
		//交易内联指令
		//innerInsAll := tx.GetTransaction().Meta.InnerInstructions
		//innMap := make(map[uint32][]*pb.InnerInstruction)
		//for _, in := range innerInsAll {
		//	innMap[in.Index] = in.Instructions
		//}
		log.Printf("tx:%d,%d, %s, accounts: %s", tx.Slot, tx.GetTransaction().Index, signature, solana.PublicKeyFromBytes(message.AccountKeys[0]).String())
		//交易指令
		/*
			for i, in := range tran.Message.Instructions {
				ProgramIdIndex := in.ProgramIdIndex
				//交易指令ProgramId索引 大于最大账户索引则输出
				if ProgramIdIndex > uint32(accountsLen) {
					// Printf 第几个交易指令 ProgramIdIndex 最大账户索引 交易Tx
					log.Printf("Ins ProgramIdIndex %d: %d>%d, %s ", i, ProgramIdIndex, accountsLen, signature)
				}
				//交易指令accounts索引  大于最大账户索引则输出
			inFOR:
				for _, u := range in.Accounts {
					if u > uint8(accountsLen) {
						// Printf 第几个交易指令 索引列表 最大账户索引 交易Tx
						log.Printf("Ins accountIndex %d:  %v>%d, %s ", i, in.Accounts, accountsLen, base58.Encode(tran.Signatures[0]))
						break inFOR
					}
				}
				innerIns, exists := innMap[uint32(i)]
				if exists {

					for iin, insi := range innerIns {
						//交易内联指令ProgramId索引 大于最大账户索引则输出
						iProgramIdIndex := insi.ProgramIdIndex
						if iProgramIdIndex >= uint32(accountsLen) {
							// Printf 第几个交易指令 内联指令列表第几个  ProgramIdIndex 最大账户索引 交易Tx
							log.Printf("Ins innProgramIdIndex %d:%d %d>%d, %s ", i, iin, ProgramIdIndex, accountsLen, signature)
						}
						//交易内联指令accounts索引  大于最大账户索引则输出
					insiFOR:
						for _, u := range insi.Accounts {
							if u >= uint8(accountsLen) {
								// Printf 第几个交易指令 内联指令列表第几个 索引列表 最大账户索引 交易Tx
								log.Printf("Ins innAccountIndex %d:%d  %v>%d, %s ", i, iin, insi.Accounts, accountsLen, base58.Encode(tran.Signatures[0]))
								break insiFOR
							}
						}
					}
				}
			}
		*/

	}
}
