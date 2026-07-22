package tracker

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"os"
	"time"

	"github.com/m4kyu/harvester/internal/peer"
	"github.com/m4kyu/harvester/internal/torrent"
	tr "github.com/m4kyu/harvester/internal/torrent"
)

type ConnectionID uint64

type UDPClient struct {
	id   ConnectionID
	conn *net.UDPConn
}

func peersListUDP(torrent tr.Torrent) ([]peer.Peer, error) {
	raw, err := url.Parse(torrent.Announce)
	if err != nil {
		return nil, err
	}

	addr, err := net.ResolveUDPAddr("udp", raw.Host)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	fmt.Println("Successful connection")

	client := UDPClient{
		conn: conn,
	}

	err = client.connect()
	if err != nil {
		return nil, err
	}

	fmt.Println("Successful connection")
	return client.announce(torrent)
}

func (client *UDPClient) announce(t torrent.Torrent) ([]peer.Peer, error) {
	transID := newTransID()

	port := uint16(6969)
	key := uint32(0)           // Just for now
	peerID := make([]byte, 20) // TODO: make a persistent peer id
	_, err := crand.Read(peerID)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, 98)
	binary.BigEndian.PutUint64(payload[0:8], uint64(client.id))
	binary.BigEndian.PutUint32(payload[8:12], 1) // action announce
	binary.BigEndian.PutUint32(payload[12:16], transID)
	copy(payload[16:36], t.InfoHash[:])
	copy(payload[36:56], peerID)
	binary.BigEndian.PutUint64(payload[56:64], 0)                  // Downloaded
	binary.BigEndian.PutUint64(payload[64:72], uint64(t.Info.Len)) // Left
	binary.BigEndian.PutUint64(payload[72:80], 0)                  // Uploaded
	binary.BigEndian.PutUint32(payload[80:84], 0)                  // 0: none; 1: completed; 2: started; 3: stopped
	binary.BigEndian.PutUint32(payload[84:88], 0)                  // IP address
	binary.BigEndian.PutUint32(payload[88:92], key)                // Key
	binary.BigEndian.PutUint32(payload[92:96], ^uint32(0))         // Num want
	binary.BigEndian.PutUint16(payload[96:98], port)

	resp, err := sendAndRead(client.conn, payload)
	if err != nil {
		return nil, err
	}

	if len(resp) < 20 {
		return nil, fmt.Errorf("resp len is less than 20 bytes")
	}

	if binary.BigEndian.Uint32(resp[0:4]) != 1 { // Check action
		return nil, fmt.Errorf("expected action 1, got: %d", binary.BigEndian.Uint32(resp[0:4]))
	}

	if binary.BigEndian.Uint32(resp[4:8]) != transID {
		return nil, fmt.Errorf("transaction id doesnt match")
	}

	respSize := len(resp) - 20
	if respSize%6 != 0 {
		return nil, fmt.Errorf("invalid peers list len")
	}

	peersCount := respSize / 6 // Actual number of peers
	peersCount = min(peersCount, int(binary.BigEndian.Uint32(resp[16:20])))
	peers := make([]peer.Peer, peersCount)

	fmt.Println("Size: ", len(resp))
	offset := 20
	for i := range peersCount {
		peers[i].IP = net.IP(resp[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16(resp[offset+4 : offset+6])
		//		fmt.Println("IP: ", peers[i].IP)

		offset += 6
	}

	fmt.Println("Seeders: ", binary.BigEndian.Uint32(resp[16:20]))
	fmt.Println("ACtual: ", peersCount)

	return peers, nil
}

func (client *UDPClient) connect() error {
	transID := newTransID()

	payload := make([]byte, 16)
	binary.BigEndian.PutUint64(payload[0:8], 0x41727101980) // Magic
	binary.BigEndian.PutUint32(payload[8:12], 0)            // Connect action
	binary.BigEndian.PutUint32(payload[12:16], transID)     // Connect action

	resp, err := sendAndRead(client.conn, payload)
	if err != nil {
		return err
	}

	if len(resp) < 16 {
		return fmt.Errorf("resp is less than 16 bytes: %v", resp)
	}

	respTransID := binary.BigEndian.Uint32(resp[4:8])
	if respTransID != transID {
		return fmt.Errorf("transaction id doesnt match")
	}

	client.id = ConnectionID(binary.BigEndian.Uint64(resp[8:]))
	return nil
}

func newTransID() uint32 {
	return uint32(rand.Int31())
}

func sendAndRead(conn *net.UDPConn, data []byte) ([]byte, error) {
	counter := 0 // Max 8

	_, err := conn.Write(data)
	if err != nil {
		return nil, err
	}

	for counter < 8 {
		waitTime := 15 * int(math.Pow(2, float64(counter)))
		buffer := make([]byte, 8*1024)

		fmt.Println("Wait time: ", waitTime)
		conn.SetReadDeadline(time.Now().Add(time.Duration(waitTime) * time.Second))
		defer conn.SetDeadline(time.Time{})

		n, err := conn.Read(buffer)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				counter++
				continue
			}

			return nil, err
		}

		return buffer[:n], nil
	}

	return nil, fmt.Errorf("reached wait limit")
}
