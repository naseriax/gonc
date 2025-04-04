package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Endpoint struct {
	Ip           string
	Name         string
	Username     string
	Password     string
	PrivKeyPath  string
	Port         string
	SshOut       io.Reader
	SshIn        io.WriteCloser
	Timeout      int
	Client       *ssh.Client
	Session      *ssh.Session
	Capabilities string
}

func publicKeyFile(file string) ssh.AuthMethod {
	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

// Connect connects to the specified server and opens a session (Filling the Client and Session fields in SshAgent struct).
func (s *Endpoint) Connect() error {
	if err := validateNode(s); err != nil {
		return err
	}

	var err error
	config := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(s.Timeout) * time.Second,
	}

	config.User = s.Username

	authMethods := []ssh.AuthMethod{
		ssh.Password(s.Password),
	}

	if s.PrivKeyPath != "" {
		authMethods = append(authMethods, publicKeyFile(s.PrivKeyPath))
	}

	config.Auth = authMethods

	s.Client, err = ssh.Dial("tcp", fmt.Sprintf("%v:%v", s.Ip, s.Port), config)
	if err != nil {
		return fmt.Errorf("%v:%v - %v", s.Ip, s.Port, err.Error())
	}

	if err := s.cliLogin(); err != nil {
		return err
	}

	helloPayload := `
	<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  		<capabilities>
    		<capability>urn:ietf:params:netconf:base:1.0</capability>
  		</capabilities>
	</hello>
	]]>]]>`

	s.Run(helloPayload)

	return nil
}

func (s *Endpoint) cliLogin() error {
	var err error

	s.Session, err = s.Client.NewSession()
	if err != nil {
		return fmt.Errorf("%v:%v - failure on Client.NewSession() - details: %v", s.Ip, s.Port, err.Error())
	}

	err = s.Session.RequestSubsystem("netconf")
	if err != nil {
		log.Fatalf("Failed to request netconf subsystem: %v", err)
	}

	s.SshIn, err = s.Session.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to get stdin: %v", err)
	}
	s.SshOut, err = s.Session.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to get stdout: %v", err)
	}

	helloMsg := `<?xml version="1.0" encoding="UTF-8"?>
	<hello xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
	  <capabilities>
		<capability>urn:ietf:params:netconf:base:1.0</capability>
	  </capabilities>
	</hello>]]>]]>`

	_, err = s.SshIn.Write([]byte(helloMsg))
	if err != nil {
		log.Fatalf("Failed to send hello message: %v", err)
	}

	var responseBuf bytes.Buffer
	buf := make([]byte, 1024)
	delimiter := []byte("]]>]]>")
	for {
		n, err := s.SshOut.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatalf("Failed to read response: %v", err)
		}
		if n > 0 {
			responseBuf.Write(buf[:n])
			if bytes.Contains(responseBuf.Bytes(), delimiter) {
				break
			}
		}

		if err == io.EOF {
			break
		}
	}

	s.Capabilities = responseBuf.String()

	return nil

}

// Run executes the given cli command on the opened session.
func (s *Endpoint) Run(arg string) (string, error) {

	if !strings.Contains(arg, "]]>]]>") {
		arg = arg + "]]>]]>"
	}

	_, err := s.SshIn.Write([]byte(arg))
	if err != nil {
		log.Fatalf("Failed to send the rpc message: %v", err)
	}

	var responseBuf bytes.Buffer
	buf := make([]byte, 1024)
	delimiter := []byte("]]>]]>")
	for {
		n, err := s.SshOut.Read(buf)
		if err != nil && err != io.EOF {
			log.Fatalf("Failed to read response: %v", err)
		}
		if n > 0 {
			responseBuf.Write(buf[:n])
			if bytes.Contains(responseBuf.Bytes(), delimiter) {
				break
			}
		}
		if err == io.EOF {
			break
		}
	}

	return responseBuf.String(), nil
}

// Disconnect closes the ssh sessoin.
func (s *Endpoint) Disconnect() {

	closePayload := `<rpc message-id="103" xmlns="urn:ietf:params:xml:ns:netconf:base:1.0">
  		<close-session/>
	</rpc>]]>]]>`
	s.Run(closePayload)
	s.Session.Close()
	s.Client.Close()
}

func validateIpAddress(ip string) error {
	ipSegments := strings.Split(ip, ".")
	if len(ipSegments) != 4 {
		return fmt.Errorf("provided ip: %v - ip address is not formatted properly", ip)
	}
	for _, seg := range ipSegments {
		num, err := strconv.Atoi(seg)
		if err != nil {
			return fmt.Errorf("provided ip: %v - ip address includes wrong values: %v", ip, seg)
		} else {
			if num < 0 || num > 255 {
				return fmt.Errorf("provided ip: %v - ip address includes wrong values: %v", ip, seg)
			}
		}
	}

	return nil
}

func validateNode(s *Endpoint) error {
	s.Timeout = 30
	if err := validateIpAddress(s.Ip); err != nil {
		return err
	}
	if _, err := strconv.Atoi(s.Port); err != nil {
		log.Printf("provided port: %v - wrong port number, defaulting to 22", s.Port)
		s.Port = "22"
	}

	return nil
}
