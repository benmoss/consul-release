package main

import (
	"confab"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/agent"
	"github.com/pivotal-golang/clock"
)

type stringSlice []string

func (ss *stringSlice) String() string {
	return fmt.Sprintf("%s", *ss)
}

func (ss *stringSlice) Set(value string) error {
	*ss = append(*ss, value)

	return nil
}

var (
	nodeType        string
	agentPath       string
	consulConfigDir string
	pidFile         string
	expectedMembers stringSlice
	encryptionKeys  stringSlice

	stdout = log.New(os.Stdout, "", 0)
	stderr = log.New(os.Stderr, "", 0)
)

func main() {
	flagSet := flag.NewFlagSet("flags", flag.ContinueOnError)
	flagSet.StringVar(&nodeType, "node-type", "", "client or server")
	flagSet.StringVar(&agentPath, "agent-path", "", "path to the on-filesystem consul `executable`")
	flagSet.StringVar(&consulConfigDir, "consul-config-dir", "", "path to consul configuration `directory`")
	flagSet.StringVar(&pidFile, "pid-file", "", "path to consul PID `file`")
	flagSet.Var(&expectedMembers, "expected-member", "address `list` of the expected members")
	flagSet.Var(&encryptionKeys, "encryption-key", "`key` used to encrypt consul traffic")

	if len(os.Args) < 2 {
		printUsageAndExit("invalid number of arguments", flagSet)
	}

	command := os.Args[1]
	if !validCommand(command) {
		printUsageAndExit(fmt.Sprintf("invalid COMMAND %q", command), flagSet)
	}

	flagSet.Parse(os.Args[2:])

	path, err := exec.LookPath(agentPath)
	if err != nil {
		printUsageAndExit(fmt.Sprintf("\"agent-path\" %q cannot be found", agentPath), flagSet)
	}

	if len(pidFile) == 0 {
		printUsageAndExit("\"pid-file\" cannot be empty", flagSet)
	}

	_, err = os.Stat(consulConfigDir)
	if err != nil {
		printUsageAndExit(fmt.Sprintf("\"consul-config-dir\" %q could not be found", consulConfigDir), flagSet)
	}

	if len(expectedMembers) == 0 {
		printUsageAndExit("at least one \"expected-member\" must be provided", flagSet)
	}

	agentRunner := confab.AgentRunner{
		Path:      path,
		PIDFile:   pidFile,
		ConfigDir: consulConfigDir,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}
	consulAPIClient, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	agentClient := confab.AgentClient{
		ExpectedMembers: expectedMembers,
		ConsulAPIAgent:  consulAPIClient.Agent(),
		ConsulRPCClient: nil,
	}

	controller := confab.Controller{
		AgentRunner:    &agentRunner,
		AgentClient:    &agentClient,
		MaxRetries:     10,
		SyncRetryDelay: 1 * time.Second,
		SyncRetryClock: clock.NewClock(),
		EncryptKeys:    encryptionKeys,
		SSLDisabled:    false,
		Logger:         nil,
	}

	switch nodeType {
	case "client":
		err = controller.BootAgent()
		if err != nil {
			panic(err)
		}
	case "server":
		err = controller.BootAgent()
		if err != nil {
			panic(err)
		}
		rpcClient, err := agent.NewRPCClient("localhost:8400")
		if err != nil {
			panic(err)
		}
		agentClient.ConsulRPCClient = &confab.RPCClient{
			*rpcClient,
		}

		err = controller.ConfigureServer()
		if err != nil {
			panic(err)
		}
	default:
		panic("unhandled default case")
	}
}

func printUsageAndExit(message string, flagSet *flag.FlagSet) {
	stderr.Printf("%s\n\n", message)
	stderr.Println("usage: confab COMMAND OPTIONS\n")
	stderr.Println("COMMAND: \"start\" or \"stop\"")
	stderr.Println("\nOPTIONS:")
	flagSet.PrintDefaults()
	stderr.Println()
	os.Exit(1)
}

func validCommand(command string) bool {
	for _, c := range []string{"start", "stop"} {
		if command == c {
			return true
		}
	}

	return false
}