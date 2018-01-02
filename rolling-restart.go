package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cloudfoundry/cli/plugin"
	"github.com/fatih/color"
)

var (
	maxRestartWaitCycles = 120
	printLine            = fmt.Println
	printFormatted       = fmt.Printf
	spinner              = NewSpinner(os.Stdout)
)

type Instance struct {
	State  string `json:"state"`
	Uptime int    `json:"uptime"`
	Since  int    `json:"since"`
}

type Instances map[string]Instance

type RollingRestart struct{}

func (c *RollingRestart) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "RollingRestartPlugin",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 0,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "rolling-restart",
				HelpText: "Restart instances of your application one at a time for zero downtime.",
				Alias:    "rrs",
				UsageDetails: plugin.Usage{
					Usage: "cf rolling-restart [--max-cycles #] APP_NAME",
				},
			},
		},
	}
}

func (c *RollingRestart) Run(conn plugin.CliConnection, args []string) {
	if args[0] != "rolling-restart" && args[0] != "rrs" {
		return
	}

	var appName string
	var appGuid string
	var instances Instances
	var instanceIds []string
	var isRunning bool
	var restarted bool
	var err error

	if appName, err = setFlagsAndReturnAppName(args); err != nil {
		printErrorAndExit(err.Error())
	}

	if err = validateCLISession(conn); err != nil {
		printErrorAndExit(err.Error())
	}

	if appGuid, err = getAppGuid(conn, appName); err != nil {
		printErrorAndExit(err.Error())
	}

	if instances, err = getInstances(conn, appGuid); err != nil {
		printFormatted("Failed to get the instance information for %s.\n", appName)
		printErrorAndExit(err.Error())
	}

	if instanceIds = getKeysFor(instances); len(instanceIds) < 2 {
		printErrorAndExit("There are too few instances to ensure zero-downtime, use `cf restart APP_NAME` if you are OK with downtime.")
	}

	printFormatted("Beginning restart of app instances for %s.\n", appName)

	for _, instanceId := range instanceIds {
		if err = restartInstance(conn, appName, instanceId); err != nil {
			printFormatted("Failed to restart instance %s.\n", instanceId)
			printErrorAndExit(err.Error())
		}

		printFormatted("Checking status of instance %s.\n", instanceId)

		isRunning = false
		for i := 0; i < maxRestartWaitCycles; i++ {
			spinner.Next()

			if isRunning, err = isInstanceRunning(conn, appGuid, instanceId); err != nil {
				printFormatted("Failed to get the instance information for %s.\n", appName)
				printErrorAndExit(err.Error())
			}

			if isRunning {
				spinner.Done()
				restarted = true
				break
			}

			time.Sleep(time.Second)
		}

		if restarted == false {
			printErrorAndExit(fmt.Sprintf("Application did not restart within %d Second(s), failing out. Check your current application state.\n", maxRestartWaitCycles))
		}
	}

	printFormatted("Finished restart of app instances for %s.", appName)
}

func printErrorAndExit(message string) {
	color.New(color.FgRed, color.Bold).Println("FAILED")
	printLine(message)
	os.Exit(1)
}

func setFlagsAndReturnAppName(args []string) (string, error) {
	rrsFlags := flag.NewFlagSet("rolling-restart", flag.ExitOnError)
	maxCycles := rrsFlags.Int("max-cycles", maxRestartWaitCycles, "Maximum number of cycles to wait when checking for restart status. (Optional)")
	rrsFlags.Parse(args[1:])

	if !rrsFlags.Parsed() {
		return "", errors.New("Failed parsing command line arguements.")
	}

	maxRestartWaitCycles = *maxCycles
	remainingArgs := rrsFlags.Args()

	if len(remainingArgs) == 0 {
		return "", errors.New("An application name was not provided. Usage: cf rolling-restart APP_NAME")
	}

	if len(remainingArgs) > 1 {
		return "", errors.New("Only a single app name is currently supported, please try again.")
	}

	return remainingArgs[0], nil
}

func validateCLISession(conn plugin.CliConnection) error {
	var loggedIn bool
	var hasOrg bool
	var hasSpace bool
	var err error

	if loggedIn, err = conn.IsLoggedIn(); err != nil {
		return err
	}

	if !loggedIn {
		return errors.New("You are not logged in, please log in and try again.")
	}

	if hasOrg, err = conn.HasOrganization(); err != nil {
		return err
	}

	if !hasOrg {
		return errors.New("The logged in user does not have an Org set, please select an Org and Space and try again.")
	}

	if hasSpace, err = conn.HasSpace(); err != nil {
		return err
	}

	if !hasSpace {
		return errors.New("The logged in user does not have a Space set, please select a Space and try again.")
	}

	return nil
}

func isInstanceRunning(conn plugin.CliConnection, appGuid string, instanceId string) (bool, error) {
	var instanceStatuses Instances
	var err error

	if instanceStatuses, err = getInstances(conn, appGuid); err != nil {
		return false, err
	}

	instance := instanceStatuses[instanceId]
	running := instance.State == "RUNNING" && instance.Uptime < 10
	return running, nil
}

func restartInstance(conn plugin.CliConnection, appName string, instanceId string) error {
	_, err := conn.CliCommand("restart-app-instance", appName, instanceId)
	return err
}

func getAppGuid(conn plugin.CliConnection, appName string) (string, error) {
	var appGuid []string
	var err error

	if appGuid, err = conn.CliCommandWithoutTerminalOutput("app", appName, "--guid"); err != nil {
		return "", err
	}

	return appGuid[0], nil
}

func getInstances(conn plugin.CliConnection, appGuid string) (Instances, error) {
	var instances Instances

	instancesCurlUrl := fmt.Sprintf("/v2/apps/%s/instances", appGuid)
	instanceJson, curlErr := conn.CliCommandWithoutTerminalOutput("curl", "-X", "GET", instancesCurlUrl)
	if curlErr != nil {
		return nil, curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(instanceJson, "")), &instances)
	if unmarshallErr != nil {
		return nil, unmarshallErr
	}

	return instances, nil
}

func getKeysFor(m map[string]Instance) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func main() {
	plugin.Start(new(RollingRestart))
}
