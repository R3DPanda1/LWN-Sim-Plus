package udp

import (
	"errors"
	"net"
)

func ConnectTo(BridgeAddress string) (*net.UDPConn, error) {

	addressRS, err := net.ResolveUDPAddr("udp", BridgeAddress)
	if err != nil {
		return nil, err
	}

	return net.DialUDP("udp", nil, addressRS) //udp4

}

func SendDataUDP(connection *net.UDPConn, data []byte) (int, error) {
	if connection == nil {
		return 0, errors.New("UDP connection is nil")
	}
	return connection.Write(data)

}
