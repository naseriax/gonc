package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
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
	Filter   string
	Timeout  int
}

func main() {
	defer customPanicHandler()

	config := Config{}
	flag.StringVar(&config.IP, "ip", "", "IP address of the NETCONF device (required)")
	flag.StringVar(&config.Port, "port", "830", "Port number for NETCONF connection")
	flag.StringVar(&config.Username, "username", "admin", "Username for authentication")
	flag.StringVar(&config.Password, "password", "", "Password for authentication (required)")
	flag.StringVar(&config.File, "file", "", "Path to XML file containing NETCONF RPC payload")
	flag.StringVar(&config.Output, "output", "", "Path to output file for NETCONF response (optional)")
	flag.StringVar(&config.Filter, "filter", "", "start-with xpath filtering only for the last element")
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

	output, err := runNetconfClient(config)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if config.Filter != "" {
		output = enhancedFilter(output, config.Filter)
	}

	if config.Output != "" {
		err = os.WriteFile(config.Output, []byte(output), 0644)
		if err != nil {
			fmt.Printf("failed to write response to file %s: %v\n", config.Output, err)
		}
		fmt.Printf("Response written to %s\n", config.Output)
	} else {
		fmt.Println("NETCONF Response:")
		fmt.Println(output)
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

func runNetconfClient(config Config) (string, error) {

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

	err := os.WriteFile(config.IP+"_capabilities.xml", []byte(formatXML(ncEndPoint.Capabilities)), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write response to file %s: %v", config.Output, err)
	}

	formattedResponse := ""

	rpc, err := getRPCPayload(config)
	if err != nil {
		return "", fmt.Errorf("failed to get RPC payload: %v", err)
	}

	reply, err := ncEndPoint.Run(rpc)
	if err != nil {
		return "", fmt.Errorf("failed to execute NETCONF RPC: %v", err)
	}

	formattedResponse = formatXML(reply)

	return formattedResponse, nil
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

func formatXML(data string) string {
	var b strings.Builder
	b.Grow(len(data))

	indent := 0
	for i := range len(data) {
		switch data[i] {
		case '<':
			if i+1 < len(data) && data[i+1] == '/' {
				indent--
				b.WriteByte('<')
			} else {
				b.WriteByte('<')
				indent++
			}
		case '>':
			b.WriteByte('>')
		default:
			b.WriteByte(data[i])
		}
	}

	lines := strings.Split(b.String(), "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func removePaths(stack []byte) []byte {
	lines := bytes.Split(stack, []byte("\n"))
	for i, line := range lines {
		if idx := bytes.LastIndex(line, []byte("/go/")); idx != -1 {
			lines[i] = line[idx+4:]
		} else if idx := bytes.Index(line, []byte(":")); idx != -1 {
			lines[i] = line[idx:]
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func customPanicHandler() {
	if r := recover(); r != nil {
		// Get the stack trace
		stack := debug.Stack()

		// Remove file paths from the stack trace
		sanitizedStack := removePaths(stack)

		// Log or print the sanitized stack trace
		fmt.Printf("Panic: %v\n%s", r, sanitizedStack)

		// Optionally, exit the program
		os.Exit(1)
	}
}

func parseXPathFilter(filter string) (predicate []string, path []string, err error) {

	filter = strings.Trim(filter, "/ ")
	startIdx := strings.Index(filter, "[")
	if startIdx == -1 {
		return nil, nil, fmt.Errorf("no predicate found in filter")
	}

	pathStr := filter[:startIdx]
	predicateStr := filter[startIdx:]
	path = strings.Split(pathStr, "/")
	if len(path) == 0 {
		return nil, nil, fmt.Errorf("empty path")
	}

	predicateStr = strings.Trim(predicateStr, "[]")
	if strings.HasPrefix(predicateStr, "start-with(") {
		argsStr := strings.TrimPrefix(predicateStr, "start-with(")
		argsStr = strings.TrimSuffix(argsStr, ")")
		predicate = strings.SplitN(argsStr, ",", 2)
		if len(predicate) != 2 {
			return nil, nil, fmt.Errorf("invalid start-with predicate: %s", predicateStr)
		}
		predicate[1] = strings.Trim(predicate[1], "'\"")
	} else {
		predicate = []string{predicateStr}
	}

	return predicate, path, nil
}

func enhancedFilter(xmlData, filter string) string {

	// filter := "/rpc-reply/data/terminal-device/logical-channels/channel[start-with(index,'10115')]"

	predicate, path, err := parseXPathFilter(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return ""
	}
	targetElement := path[len(path)-1]
	predicatePrefix := predicate[1]
	var output bytes.Buffer
	var currentChannel bytes.Buffer
	inChannel := false
	keepChannel := false
	depth := 0
	stack := []xml.StartElement{}

	decoder := xml.NewDecoder(strings.NewReader(xmlData))

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == targetElement && !inChannel {
				inChannel = true
				depth = 1
				currentChannel.Reset()
				currentChannel.WriteString(xmlMarshalStartElement(t))
			} else if inChannel {
				depth++
				currentChannel.WriteString(xmlMarshalStartElement(t))
				if t.Name.Local == "index" {
					nextToken, _ := decoder.Token()
					if charData, ok := nextToken.(xml.CharData); ok {
						indexValue := string(charData)
						if strings.HasPrefix(indexValue, predicatePrefix) {
							keepChannel = true
						}
						currentChannel.WriteString(indexValue)
					}
				}
			} else {
				output.WriteString(xmlMarshalStartElement(t))
				stack = append(stack, t)
			}

		case xml.EndElement:
			if inChannel {
				currentChannel.WriteString(fmt.Sprintf("</%s>", t.Name.Local))
				depth--
				if depth == 0 {
					inChannel = false
					if keepChannel {
						output.Write(currentChannel.Bytes())
						output.WriteString("\n")
					}
					keepChannel = false
				}
			} else {
				if len(stack) > 0 {
					output.WriteString(fmt.Sprintf("</%s>\n", t.Name.Local))
					stack = stack[:len(stack)-1]
				}
			}

		case xml.CharData:
			if inChannel {
				currentChannel.WriteString(string(t))
			} else {
				output.WriteString(string(t))
			}
		}
	}

	for i := len(stack) - 1; i >= 0; i-- {
		output.WriteString(fmt.Sprintf("</%s>\n", stack[i].Name.Local))
	}

	return formatXML(output.String())

}

func xmlMarshalStartElement(se xml.StartElement) string {
	var attrs string
	for _, attr := range se.Attr {
		attrs += fmt.Sprintf(` %s="%s"`, attr.Name.Local, attr.Value)
	}
	return fmt.Sprintf("<%s%s>", se.Name.Local, attrs)
}
