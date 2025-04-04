package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	IP       string
	Port     string
	Username string
	Password string
	Path     string
	File     string
	Output   string
	Key      string
	Timeout  int
}

func main() {
	config := Config{}
	flag.StringVar(&config.IP, "ip", "", "IP address of the NETCONF device (required)")
	flag.StringVar(&config.Port, "port", "830", "Port number for NETCONF connection")
	flag.StringVar(&config.Username, "username", "admin", "Username for authentication")
	flag.StringVar(&config.Password, "password", "", "Password for authentication (required)")
	flag.StringVar(&config.File, "file", "", "Path to XML file containing NETCONF RPC payload")
	flag.StringVar(&config.Output, "output", "", "Path to output file for NETCONF response (optional)")
	flag.StringVar(&config.Key, "key", "", "ssh key file(optional)")
	flag.IntVar(&config.Timeout, "timeout", 30, "Connection timeout in seconds")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nA NETCONF client to interact with network devices\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Using inline path, output to console\n")
		fmt.Fprintf(os.Stderr, "  %s -ip 192.168.1.1 -username admin -password secret -path '<get-config><source><running/></source></get-config>'\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Using XML file, output to file\n")
		fmt.Fprintf(os.Stderr, "  %s -ip 192.168.1.1 -username admin -password secret -file rpc.xml -output response.xml\n", os.Args[0])
	}

	flag.Parse()

	if err := validateConfig(config); err != nil {
		fmt.Printf("Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	if err := runNetconfClient(config); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func validateConfig(config Config) error {
	if config.IP == "" || config.Password == "" {
		return fmt.Errorf("IP address and password are required")
	}
	if config.Path != "" && config.File != "" {
		return fmt.Errorf("cannot specify both -path and -file; choose one")
	}
	if config.Path == "" && config.File == "" {
		return fmt.Errorf("either -path or -file must be specified")
	}
	return nil
}

func runNetconfClient(config Config) error {

	ncEndPoint := Endpoint{
		Ip:          config.IP,
		Username:    config.Username,
		Password:    config.Password,
		PrivKeyPath: config.Key,
		Timeout:     10,
		Port:        config.Port,
	}

	if err := ncEndPoint.Connect(); err != nil {
		log.Fatalln(err)
	}
	defer ncEndPoint.Disconnect()

	err := os.WriteFile(config.IP+"_capabilities.xml", []byte(formatXML(ncEndPoint.Capabilities, true)), 0644)
	if err != nil {
		return fmt.Errorf("failed to write response to file %s: %v", config.Output, err)
	}

	formattedResponse := ""

	rpc, err := getRPCPayload(config)
	if err != nil {
		return fmt.Errorf("failed to get RPC payload: %v", err)
	}

	reply, err := ncEndPoint.Run(rpc)
	if err != nil {
		return fmt.Errorf("failed to execute NETCONF RPC: %v", err)
	}

	formattedResponse = formatXML(reply, true)

	if config.Output != "" {
		err = os.WriteFile(config.Output, []byte(formattedResponse), 0644)
		if err != nil {
			return fmt.Errorf("failed to write response to file %s: %v", config.Output, err)
		}
		fmt.Printf("Response written to %s\n", config.Output)
	} else {
		fmt.Println("NETCONF Response:")
		fmt.Println(formattedResponse)
	}

	return nil
}

func removeEmptyLines(s string) string {
	lines := strings.Split(s, "\n")
	var b strings.Builder
	b.Grow(len(s))

	first := true
	for _, line := range lines {
		if line != "" {
			if !first {
				b.WriteString("\n")
			}
			b.WriteString(line)
			first = false
		}
	}
	return b.String()
}

func getRPCPayload(config Config) (string, error) {
	if config.File != "" {
		data, err := os.ReadFile(config.File)
		if err != nil {
			return "", fmt.Errorf("failed to read XML file %s: %v", config.File, err)
		}
		return removeEmptyLines(string(data)), nil
	}
	return config.Path, nil
}

func formatXML(data string, removeBlankLines bool) string {
	var b strings.Builder
	if removeBlankLines {
		b.Grow(len(data))
	} else {
		b.Grow(len(data) + len(data)/2)
	}

	indent := 0
	for i := range len(data) {
		switch data[i] {
		case '<':
			if i+1 < len(data) && data[i+1] == '/' {
				indent--
				if !removeBlankLines {
					b.WriteString("\n")
					b.WriteString(indentString(indent))
				}
				b.WriteByte('<')
			} else {
				if !removeBlankLines {
					b.WriteString("\n")
					b.WriteString(indentString(indent))
				}
				b.WriteByte('<')
				indent++
			}
		case '>':
			b.WriteByte('>')
			if !removeBlankLines && i+1 < len(data) && data[i+1] == '<' {
				b.WriteString("\n")
				b.WriteString(indentString(indent))
			}
		default:
			b.WriteByte(data[i])
		}
	}
	return b.String()
}

func indentString(level int) string {
	const indentUnit = "  "
	return strings.Repeat(indentUnit, level)
}
