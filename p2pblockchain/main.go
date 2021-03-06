package main

import (

    "bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	golog "github.com/ipfs/go-log"
	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	gologging "github.com/whyrusleeping/go-logging"

)

type Block struct {
    Index     int
    TimeStamp string
    BPM       int
    Hash      string
    PrevHash  string
}

//Blockchain

var Blockchain []Block

var mutext = sync.Mutex{}


func isBlockValid(newBlock, oldBlock Block) bool {

	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}

func calculateHash(block Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp + strconv.Itoa(block.BPM) + block.PrevHash
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// create a new block using previous block's hash
func generateBlock(oldBlock Block, BPM int) Block {

	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)

	return newBlock
}


func makeBasicHost(listenPort int,  secio bool, randseed int64) (host.Host, error) {

    var r io.Reader
    if randseed == 0 {
        r = rand.Reader
    } else {
        r = mrand.New(mrand.NewSource(randseed))
    }

    priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)

    if err!=nil {
        return nil, err
    }

    opts := []libp2p.Option{
                libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)),
                libp2p.Identify(priv)
            }

    if !secio {
        opts = append(opts, libp2p.NoEncryption())
    }

    basicHost, err := libp2p.New(context.Background(), opts...)

    if err!=nil {
        return nil, err
    }

    hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s",basicHost.ID().Pretty()))

    addr := basicHost.Addrs()[0]
    fullAddr := addr.Encapsulate(hostAddr)
    log.Printf("I am %s\n",fullAddr)

    if secio {
        log.Printf("Now run \"go run main.go -l %d -d %s -secio\" on a different terminal\n", listenPort+1, fullAddr)
    } else {
        log.Printf("Now run \"go run main.go -l %d -d %s\" on a different terminal\n", listenPort+1, fullAddr)
    }

    return basicHost, nil
}

func handleStream(s net.Stream) {

    log.Println("Got a new Stream!!")

    rw := bufio.NewReaderWriter(bufio.NewReader(s), bufio.NewWriter(s))

    go readData(rw)
    go writeData(rw)

}

func readData(rw *bufio.ReadWriter) {

    for {

        str, err := rw.ReadString('\n')
        if err!=nil {
            log.Fatal(err)
        }

        if str == "" {
            return
        }

        if str != "\n" {

            chain := make([]Block, 0)
            if err := json.Unmarshal([]byte(str), &chain); err!=nil {
                log.Fatal(err)
            }

            mutex.Lock()
            if len(chain) > len(Blockchain) {
                Blockchain = chain
                bytes, err := json.MarshalIndent(Blockchain, "", "  ")
                if err!= nil {
                    log.Fatal(err)
                }
                fmt.Printf("\x1b[32m%s\x1b[0m",string(bytes))
            }
            mutex.Unlock()
        }



    }
}

func writeData(rw *bufio.ReadWriter) {


    go func() {
        for {
            time.Sleep(5 * time.Second)
            mutex.Lock()
            bytes, err := json.Marshal(Blockchain)
            if err!=nil {
                log.Println(err)
            }
            mutex.Unlock()

            mutex.Lock()
            rw.WriteString(fmt.Sprintf("%s\n",string(bytes)))
            rw.Flush()
            mutex.Unlock()
        }
    }()

    stdReader := bufio.NewReader(os.Stdin)

    for {

        fmt.Print("> ")
        sendData, err := stdReader.ReadString("\n")
        if err!=nil {
            log.Fatal(err)
        }

        sendData = strings.Replace(sendData, "\n", "", -1)
        bpm, err := strconv.Atoi(sendData)
        if err!=nil {
            log.Fatal(err)
        }

        newBlock := generateBlock(Blockchain[len(Blockchain)-1],bpm)

        if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
            mutex.Lock()
            Blockchain = append(Blockchain, newBlock)
            mutex.Unlock()
        }

        bytes, err := json.Marshal(Blockchain)
        if err!=nil {
            log.Println(err)
        }

        spew.Dump(Blockchain)

        mutex.Lock()
        rw.WriteString(fmt.Sprintf("%s\n",string(bytes)))
        rw.Flush()
        mutex.Unlock()

    }
}

