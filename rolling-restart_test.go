package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/cloudfoundry/cli/cf/errors"
	"github.com/cloudfoundry/cli/plugin/pluginfakes"
	"github.com/stretchr/testify/require"
)

var (
	rr      *RollingRestart
	cliConn *pluginfakes.FakeCliConnection

	output        []string
	spinnerBuffer bytes.Buffer
	exitCode      int

	twoInstanceResponse      = []string{"{", "\"0\": {", "\"state\": \"RUNNING\",", "\"uptime\": 5,", "\"since\": 1511990275", "},", "\"1\": {", "\"state\": \"RUNNING\",", "\"uptime\": 5,", "\"since\": 1511990327", "}", "}"}
	alwaysRestartingResponse = []string{"{", "\"0\": {", "\"state\": \"STARTING\",", "\"uptime\": 5,", "\"since\": 1511990275", "},", "\"1\": {", "\"state\": \"RUNNING\",", "\"uptime\": 5,", "\"since\": 1511990327", "}", "}"}
	singleInstanceResponse   = []string{"{", "\"0\": {", "\"state\": \"RUNNING\",", "\"uptime\": 5,", "\"since\": 1511990275", "}", "}"}
	badInstanceResponse      = []string{"bad", "response"}
)

type testError struct {
	arg  int
	prob string
}

func (e *testError) Error() string {
	return e.prob
}

func TestMain(m *testing.M) {
	rr = &RollingRestart{}
	cliConn = &pluginfakes.FakeCliConnection{}

	output = []string{}

	oldPrintln := printLine
	defer func() { printLine = oldPrintln }()

	oldPrintf := printFormatted
	defer func() { printFormatted = oldPrintf }()

	oldPrintRedBold := printRedBold
	defer func() { printRedBold = oldPrintRedBold }()

	oldExit := exit
	defer func() { exit = oldExit }()

	printLine = printlnStub
	printFormatted = printfStub
	printRedBold = printRedBoldStub
	exit = exitStub

	fakeSpinner := NewSpinner(&spinnerBuffer)

	oldMaxRestartWaitCycles := maxRestartWaitCycles
	defer func() { maxRestartWaitCycles = oldMaxRestartWaitCycles }()

	oldSpinner := spinner
	defer func() { spinner = oldSpinner }()

	maxRestartWaitCycles = 1
	spinner = fakeSpinner

	code := m.Run()

	os.Exit(code)
}

func TestRollingRestart_Run_Success(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, twoInstanceResponse)
	setupCliCommandStub(true, true)

	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, 2, cliConn.CliCommandCallCount())
	require.Equal(t, []string{"restart-app-instance", "testApp", "0"}, cliConn.CliCommandArgsForCall(0))
	require.Equal(t, []string{"restart-app-instance", "testApp", "1"}, cliConn.CliCommandArgsForCall(1))

	require.Equal(t, 4, cliConn.CliCommandWithoutTerminalOutputCallCount())
	require.Equal(t, []string{"app", "testApp", "--guid"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(0))
	require.Equal(t, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(1))
	require.Equal(t, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(2))
	require.Equal(t, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(3))

	require.Equal(t, 4, len(output))
	require.Equal(t, "Beginning restart of app instances for testApp.\n", output[0])
	require.Equal(t, "Checking status of instance 0.\n", output[1])
	require.Equal(t, "Checking status of instance 1.\n", output[2])
	require.Equal(t, "Finished restart of app instances for testApp.\n", output[3])
	require.Contains(t, spinnerBuffer.String(), "OK")

	require.Equal(t, exitCode, 0)
}

func TestRollingRestart_Run_Success_SingleAppInstance(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, singleInstanceResponse)
	setupCliCommandStub(true, true)

	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, 3, cliConn.CliCommandCallCount())
	require.Equal(t, []string{"scale", "testApp", "-i", "2"}, cliConn.CliCommandArgsForCall(0))
	require.Equal(t, []string{"restart-app-instance", "testApp", "0"}, cliConn.CliCommandArgsForCall(1))
	require.Equal(t, []string{"scale", "testApp", "-i", "1"}, cliConn.CliCommandArgsForCall(2))

	require.Equal(t, 4, cliConn.CliCommandWithoutTerminalOutputCallCount())
	require.Equal(t, []string{"app", "testApp", "--guid"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(0))
	require.Equal(t, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(1))
	require.Equal(t, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(2))
	require.Equal(t, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}, cliConn.CliCommandWithoutTerminalOutputArgsForCall(2))

	require.Equal(t, 7, len(output))
	require.Equal(t, "Only found a single instance of testApp, scaling up to two instances.\n", output[0])
	require.Equal(t, "Checking status of instance 1.\n", output[1])
	require.Equal(t, "Finished scaling testApp to two instances.\n", output[2])
	require.Equal(t, "Beginning restart of app instances for testApp.\n", output[3])
	require.Equal(t, "Checking status of instance 0.\n", output[4])
	require.Equal(t, "Scaling testApp back down to one instance.\n", output[5])
	require.Equal(t, "Finished restart of app instances for testApp.\n", output[6])

	require.Contains(t, spinnerBuffer.String(), "OK")

	require.Equal(t, exitCode, 0)
}

func TestRollingRestart_Run_NoAppNameProvided(t *testing.T) {
	resetOutput()
	rr.Run(cliConn, []string{"rolling-restart"})

	require.Equal(t, exitCode, 1)
	require.Equal(t, "An application name was not provided. Usage: cf rolling-restart APP_NAME\n", output[0])
}

func TestRollingRestart_Run_MultipleAppNameProvided(t *testing.T) {
	resetOutput()
	rr.Run(cliConn, []string{"rolling-restart", "firstApp", "secondApp"})

	require.Equal(t, exitCode, 1)
	require.Equal(t, "Only a single app name is currently supported, please try again.\n", output[0])
}

func TestRollingRestart_Run_ArguementParsingError(t *testing.T) {
	resetOutput()
	rr.Run(cliConn, []string{"rolling-restart", "firstApp", "secondApp"})

	require.Equal(t, exitCode, 1)
	require.Equal(t, "Only a single app name is currently supported, please try again.\n", output[0])
}

func TestRollingRestart_Run_NotLoggedIn(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(false, false)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Equal(t, "You are not logged in, please log in and try again.\n", output[0])
}

func TestRollingRestart_Run_isLoggedInThrowsErr(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(false, true)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "CLI FAILURE")
}

func TestRollingRestart_Run_OrgNotSet(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(false, false)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "The logged in user does not have an Org set, please select an Org and Space and try again.")
}

func TestRollingRestart_Run_hasOrgThrowsErr(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(false, true)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "CLI FAILURE")
}

func TestRollingRestart_Run_SpaceNotSet(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(false, false)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "The logged in user does not have a Space set, please select a Space and try again.")
}

func TestRollingRestart_Run_hasSpaceThrowsError(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(false, true)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "CLI FAILURE")
}

func TestRollingRestart_Run_GetGuidThrowsError(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(false, true, twoInstanceResponse)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "CliCommandWithoutTerminalStubError")
}

func TestRollingRestart_Run_GetInstancesThrowsError(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, false, twoInstanceResponse)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "Failed to get the instance information for testApp.\n")
}

func TestRollingRestart_Run_ScaleAppThrowsError(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, singleInstanceResponse)
	setupCliCommandStub(true, false)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[1], "Failed to scale testApp to two instances.")
}

func TestRollingRestart_Run_RestartInstanceThrowsError(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, twoInstanceResponse)
	setupCliCommandStub(false, true)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[1], "Failed to restart instance 0.\n")
}

func TestRollingRestart_Run_InstanceJsonUnmarshallError(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, true)
	setupCliCommandWihtoutTerminalOutputStub(true, true, badInstanceResponse)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "CLI FAILURE")
}

func TestRollingRestart_Run_InstanceDoesNotRestartInCycleLimit(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, alwaysRestartingResponse)
	setupCliCommandStub(true, true)
	rr.Run(cliConn, []string{"rolling-restart", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "Beginning restart of app instances for testApp.")
	require.Contains(t, output[1], "Checking status of instance 0.")
	require.Contains(t, output[2], "Application did not restart within 1 Second(s), failing out. Check your current application state.")
}

func TestRollingRestart_Run_InstanceDoesNotRestartCustomCycleLimit(t *testing.T) {
	resetOutput()
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, alwaysRestartingResponse)
	setupCliCommandStub(true, true)
	rr.Run(cliConn, []string{"rolling-restart", "--max-cycles", "2", "testApp"})

	require.Equal(t, exitCode, 1)
	require.Contains(t, output[0], "Beginning restart of app instances for testApp.")
	require.Contains(t, output[1], "Checking status of instance 0.")
	require.Contains(t, output[2], "Application did not restart within 2 Second(s), failing out. Check your current application state.")
}

func setupHasSpaceStub(hasSpace bool, throwError bool) {
	cliConn.HasSpaceStub = func() (bool, error) {
		if throwError {
			return false, errors.New("CLI FAILURE\n")
		}
		return hasSpace, nil
	}
}

func setupHasOrganizationStub(hasOrganization bool, throwError bool) {
	cliConn.HasOrganizationStub = func() (bool, error) {
		if throwError {
			return false, errors.New("CLI FAILURE\n")
		}
		return hasOrganization, nil
	}
}

func setupIsLoggedInStub(isLoggedIn bool, throwError bool) {
	cliConn.IsLoggedInStub = func() (bool, error) {
		if throwError {
			return false, errors.New("CLI FAILURE\n")
		}
		return isLoggedIn, nil
	}
}

func setupCliCommandWihtoutTerminalOutputStub(getGUIDSuccess bool, getInstanceStatusSuccess bool, instanceResponse []string) {
	cliConn.CliCommandWithoutTerminalOutputStub = func(args ...string) ([]string, error) {
		if reflect.DeepEqual(args, []string{"app", "testApp", "--guid"}) && getGUIDSuccess {
			return []string{"valid-app-guid"}, nil
		} else if reflect.DeepEqual(args, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}) && getInstanceStatusSuccess {
			return instanceResponse, nil
		}
		return nil, &testError{1, "CliCommandWithoutTerminalStubError"}
	}
}

func setupCliCommandStub(restartSuccess bool, scaleSuccess bool) {
	cliConn.CliCommandStub = func(args ...string) ([]string, error) {
		if args[0] == "restart-app-instance" && args[1] == "testApp" && (args[2] == "0" || args[2] == "1") && restartSuccess {
			return nil, nil
		} else if args[0] == "scale" && args[1] == "testApp" && args[2] == "-i" && args[3] == "2" && scaleSuccess {
			return nil, nil
		}
		return nil, &testError{1, "CliCommandStubError"}
	}
}

func printlnStub(a ...interface{}) (n int, err error) {
	output = append(output, fmt.Sprintln(a...))
	return 0, nil
}

func printfStub(format string, a ...interface{}) (n int, err error) {
	output = append(output, fmt.Sprintf(format, a...))
	return 0, nil
}

func printRedBoldStub(a ...interface{}) (n int, err error) {
	return 0, nil
}

func exitStub(code int) {
	exitCode = code
}

func resetOutput() {
	output = []string{}
	cliConn = &pluginfakes.FakeCliConnection{}
}
