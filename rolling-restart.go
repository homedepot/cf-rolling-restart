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

	"regexp"
	"strconv"

	"github.com/cloudfoundry/cli/plugin"
	"github.com/fatih/color"
)

// Basic variables for cf-rolling-restart.
var (
	Version    = "1.1.1"
	GitCommit  = "HEAD"
	BuildStamp = "UNKNOWN"

	maxRestartWaitCycles = 120
	printLine            = fmt.Println
	printFormatted       = fmt.Printf
	spinner              = NewSpinner(os.Stdout)
	exit                 = os.Exit
	printRedBold         = color.New(color.FgRed, color.Bold).Println
	successfulExit       = 0
	failureExit          = 1
)

// Instance provides basicinformation for a CF application which includes
// the current state as well as uptime and last updated time.
type Instance struct {
	State  string `json:"state"`
	Uptime int    `json:"uptime"`
	Since  int    `json:"since"`
}

// Instances is grouping of CF Instance for an application.
type Instances map[string]Instance

// RollingRestart provides basic structure required by CF CLI Plugins.
type RollingRestart struct {
	Version plugin.VersionType
}

// GetMetadata returns the pertinent metadata for the CF CLI Plugin architecture.
func (c *RollingRestart) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:    "cf-rolling-restart",
		Version: c.Version,
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

// Run executes the main code for Rolling Restart, exposes all required actions for a plugin.
func (c *RollingRestart) Run(conn plugin.CliConnection, args []string) {
	if args[0] != "rolling-restart" && args[0] != "rrs" {
		return
	}

	var exitCode int
	if exitCode = execute(conn, args); exitCode != 0 {
		exit(exitCode)
	}
}

func execute(conn plugin.CliConnection, args []string) (exitCode int) {
	var appName string
	var appGUID string
	var instances Instances
	var instanceIDs []string
	var restarted bool
	var err error

	if appName, err = setFlagsAndReturnAppName(args); err != nil {
		printError(err.Error())
		return failureExit
	}

	if err = validateCLISession(conn); err != nil {
		printError(err.Error())
		return failureExit
	}

	if appGUID, err = getappGUID(conn, appName); err != nil {
		printError(err.Error())
		return failureExit
	}

	if instances, err = getInstances(conn, appGUID); err != nil {
		printFormatted("Failed to get the instance information for %s.\n", appName)
		printError(err.Error())
		return failureExit
	}

	if instanceIDs = getKeysFor(instances); len(instanceIDs) < 2 {
		printFormatted("Only found a single instance of %s, scaling up to two instances.\n", appName)

		if err = scaleApplication(conn, appName, 2); err != nil {
			printFormatted("Failed to scale %s to two instances.\n", appName)
			printError(err.Error())
			return failureExit
		}

		if _, err = checkInstanceStatus(conn, appGUID, "1"); err != nil {
			printFormatted("Failed to get the instance information for %s.\n", appName)
			printError(err.Error())
			return failureExit
		}

		printFormatted("Finished scaling %s to two instances.\n", appName)
	}

	printFormatted("Beginning restart of app instances for %s.\n", appName)

	for _, instanceID := range instanceIDs {
		if err = restartInstance(conn, appName, instanceID); err != nil {
			printFormatted("Failed to restart instance %s.\n", instanceID)
			printError(err.Error())
			return failureExit
		}

		if restarted, err = checkInstanceStatus(conn, appGUID, instanceID); err != nil {
			printFormatted("Failed to get the instance information for %s.\n", appName)
			printError(err.Error())
			return failureExit
		}

		if restarted == false {
			printError(fmt.Sprintf("Application did not restart within %d Second(s), failing out. Check your current application state.\n", maxRestartWaitCycles))
			return failureExit
		}
	}

	if len(instanceIDs) == 1 {
		printFormatted("Scaling %s back down to one instance.\n", appName)
		scaleApplication(conn, appName, 1)
	}

	printFormatted("Finished restart of app instances for %s.\n", appName)

	return successfulExit
}

func scaleApplication(conn plugin.CliConnection, appName string, numberOfInstances int) error {
	_, err := conn.CliCommand("scale", appName, "-i", strconv.Itoa(numberOfInstances))
	return err
}

func checkInstanceStatus(conn plugin.CliConnection, appGUID string, instanceID string) (bool, error) {
	printFormatted("Checking status of instance %s.\n", instanceID)

	var isRunning bool
	var restarted bool
	var err error

	for i := 0; i < maxRestartWaitCycles; i++ {
		spinner.Next()

		if isRunning, err = isInstanceRunning(conn, appGUID, instanceID); err != nil {
			return false, err
		}

		if isRunning {
			spinner.Done()
			restarted = true
			break
		}

		time.Sleep(time.Second)
	}
	return restarted, nil
}

func printError(message string) {
	printRedBold("FAILED")
	printLine(message)
}

func setFlagsAndReturnAppName(args []string) (string, error) {
	rrsFlags := flag.NewFlagSet("rolling-restart", flag.ExitOnError)
	maxCycles := rrsFlags.Int("max-cycles", maxRestartWaitCycles, "Maximum number of cycles to wait when checking for restart status. (Optional)")
	rrsFlags.Parse(args[1:])

	if !rrsFlags.Parsed() {
		return "", errors.New("Failed parsing command line arguments.")
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

func isInstanceRunning(conn plugin.CliConnection, appGUID string, instanceID string) (bool, error) {
	var instanceStatuses Instances
	var err error

	if instanceStatuses, err = getInstances(conn, appGUID); err != nil {
		return false, err
	}

	instance := instanceStatuses[instanceID]
	running := instance.State == "RUNNING" && instance.Uptime < 10
	return running, nil
}

func restartInstance(conn plugin.CliConnection, appName string, instanceID string) error {
	_, err := conn.CliCommand("restart-app-instance", appName, instanceID)
	return err
}

func getappGUID(conn plugin.CliConnection, appName string) (string, error) {
	var appGUID []string
	var err error

	if appGUID, err = conn.CliCommandWithoutTerminalOutput("app", appName, "--guid"); err != nil {
		return "", err
	}

	return appGUID[0], nil
}

func getInstances(conn plugin.CliConnection, appGUID string) (Instances, error) {
	var instances Instances

	instancesCurlURL := fmt.Sprintf("/v2/apps/%s/instances", appGUID)
	instanceJSON, curlErr := conn.CliCommandWithoutTerminalOutput("curl", "-X", "GET", instancesCurlURL)
	if curlErr != nil {
		return nil, curlErr
	}

	unmarshallErr := json.Unmarshal([]byte(strings.Join(instanceJSON, "")), &instances)
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

var versionRegexp = regexp.MustCompile(`^v?([0-9]+).([0-9]+).([0-9]+)$`)

func main() {

	submatches := versionRegexp.FindAllStringSubmatch(Version, -1)
	if len(submatches) == 0 || len(submatches[0]) != 4 {
		printError("unable to parse version `" + Version + "`")
		exit(failureExit)
	}
	major, err := strconv.Atoi(submatches[0][1])
	if err != nil {
		printError("unable to parse major version `" + Version + "`")
		exit(failureExit)
	}
	minor, err := strconv.Atoi(submatches[0][2])
	if err != nil {
		printError("unable to parse minor version `" + Version + "`")
		exit(failureExit)
	}
	build, err := strconv.Atoi(submatches[0][3])
	if err != nil {
		printError("unable to parse build version `" + Version + "`")
		exit(failureExit)
	}

	rollingRestart := &RollingRestart{
		plugin.VersionType{
			Major: major,
			Minor: minor,
			Build: build,
		},
	}

	plugin.Start(rollingRestart)
}
