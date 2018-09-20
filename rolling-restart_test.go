package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/cloudfoundry/cli/cf/errors"
	"github.com/cloudfoundry/cli/plugin/pluginfakes"
	"github.com/stretchr/testify/require"
)

var (
	rr      *RollingRestart
	cliConn *pluginfakes.FakeCliConnection

	output                 []string
	spinnerBuffer          bytes.Buffer
	subCommandOutputBuffer bytes.Buffer

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
	return ""
}

func TestMain(m *testing.M) {
	rr = &RollingRestart{}
	cliConn = &pluginfakes.FakeCliConnection{}

	if os.Getenv("TEST_OS_EXIT") != "1" {
		output = []string{}

		oldPrintln := printLine
		defer func() { printLine = oldPrintln }()

		oldPrintf := printFormatted
		defer func() { printFormatted = oldPrintf }()

		printLine = printlnStub
		printFormatted = printfStub
	}

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
	setupIsLoggedInStub(true, false)
	setupHasOrganizationStub(true, false)
	setupHasSpaceStub(true, false)
	setupCliCommandWihtoutTerminalOutputStub(true, true, twoInstanceResponse)
	setupCliCommandStub(true)

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
	require.Equal(t, "Finished restart of app instances for testApp.", output[3])
	require.Equal(t, "\r/\rOK\n\r-\rOK\n", spinnerBuffer.String())
}

func TestRollingRestart_Run_NoAppNameProvided(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		rr.Run(cliConn, []string{"rolling-restart"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_NoAppNameProvided", t)
	require.Equal(t, "FAILED\nAn application name was not provided. Usage: cf rolling-restart APP_NAME\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_NotLoggedIn(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		t.Log("Hello")

		setupIsLoggedInStub(false, false)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_NotLoggedIn", t)
	require.Equal(t, "FAILED\nYou are not logged in, please log in and try again.\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_isLoggedInThrowsErr(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(false, true)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_isLoggedInThrowsErr", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_OrgNotSet(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(false, false)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_OrgNotSet", t)
	require.Equal(t, "FAILED\nThe logged in user does not have an Org set, please select an Org and Space and try again.\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_hasOrgThrowsErr(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(false, true)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_hasOrgThrowsErr", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_SpaceNotSet(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(false, false)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_SpaceNotSet", t)
	require.Equal(t, "FAILED\nThe logged in user does not have a Space set, please select a Space and try again.\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_hasSpaceThrowsError(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(false, true)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_hasSpaceThrowsError", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_GetGuidThrowsError(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(false, true)
		setupCliCommandWihtoutTerminalOutputStub(false, true, twoInstanceResponse)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_GetGuidThrowsError", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_GetInstancesThrowsError(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(false, true)
		setupCliCommandWihtoutTerminalOutputStub(true, false, twoInstanceResponse)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_GetInstancesThrowsError", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_RestartInstanceThrowsError(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(false, true)
		setupCliCommandWihtoutTerminalOutputStub(true, true, twoInstanceResponse)
		setupCliCommandStub(false)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_RestartInstanceThrowsError", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_InstanceJsonUnmarshallError(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(false, true)
		setupCliCommandWihtoutTerminalOutputStub(true, true, badInstanceResponse)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_InstanceJsonUnmarshallError", t)
	require.Equal(t, "FAILED\nCLI FAILURE\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_InstanceDoesNotRestartInCycleLimit(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(true, false)
		setupCliCommandWihtoutTerminalOutputStub(true, true, alwaysRestartingResponse)
		setupCliCommandStub(true)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_InstanceDoesNotRestartInCycleLimit", t)
	require.Equal(t, "Beginning restart of app instances for testApp.\nChecking status of instance 0.\nFAILED\nApplication did not restart within 1 Second(s), failing out. Check your current application state.\n\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_InstanceDoesNotRestartCustomCycleLimit(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(true, false)
		setupCliCommandWihtoutTerminalOutputStub(true, true, alwaysRestartingResponse)
		setupCliCommandStub(true)
		rr.Run(cliConn, []string{"rolling-restart", "--max-cycles", "2", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_InstanceDoesNotRestartCustomCycleLimit", t)
	require.Equal(t, "Beginning restart of app instances for testApp.\nChecking status of instance 0.\nFAILED\nApplication did not restart within 2 Second(s), failing out. Check your current application state.\n\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func TestRollingRestart_Run_SingleInstanceThrowError(t *testing.T) {
	if os.Getenv("TEST_OS_EXIT") == "1" {
		setupIsLoggedInStub(true, false)
		setupHasOrganizationStub(true, false)
		setupHasSpaceStub(true, false)
		setupCliCommandWihtoutTerminalOutputStub(true, true, singleInstanceResponse)
		setupCliCommandStub(true)
		rr.Run(cliConn, []string{"rolling-restart", "testApp"})
		return
	}

	expectExitCodeOne("TestRollingRestart_Run_SingleInstanceThrowError", t)

	require.Equal(t, "FAILED\nThere are too few instances to ensure zero-downtime, use `cf restart APP_NAME` if you are OK with downtime.\n", subCommandOutputBuffer.String())
	subCommandOutputBuffer.Reset()
}

func expectExitCodeOne(testName string, t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run="+testName)
	cmd.Env = append(os.Environ(), "TEST_OS_EXIT=1")
	cmd.Stdout = &subCommandOutputBuffer
	err := cmd.Run()

	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}

	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func setupHasSpaceStub(hasSpace bool, throwError bool) {
	cliConn.HasSpaceStub = func() (bool, error) {
		if throwError {
			return false, errors.New("CLI FAILURE")
		} else {
			return hasSpace, nil
		}
	}
}

func setupHasOrganizationStub(hasOrganization bool, throwError bool) {
	cliConn.HasOrganizationStub = func() (bool, error) {
		if throwError {
			return false, errors.New("CLI FAILURE")
		} else {
			return hasOrganization, nil
		}
	}
}

func setupIsLoggedInStub(isLoggedIn bool, throwError bool) {
	cliConn.IsLoggedInStub = func() (bool, error) {
		if throwError {
			return false, errors.New("CLI FAILURE")
		} else {
			return isLoggedIn, nil
		}
	}
}

func setupCliCommandWihtoutTerminalOutputStub(getGuidSuccess bool, getInstanceStatusSuccess bool, instanceResponse []string) {
	cliConn.CliCommandWithoutTerminalOutputStub = func(args ...string) ([]string, error) {
		if reflect.DeepEqual(args, []string{"app", "testApp", "--guid"}) && getGuidSuccess {
			return []string{"valid-app-guid"}, nil
		} else if reflect.DeepEqual(args, []string{"curl", "-X", "GET", "/v2/apps/valid-app-guid/instances"}) && getInstanceStatusSuccess {
			return instanceResponse, nil
		}
		return nil, &testError{}
	}
}

func setupCliCommandStub(restartSuccess bool) {
	cliConn.CliCommandStub = func(args ...string) ([]string, error) {
		if args[0] == "restart-app-instance" && args[1] == "testApp" && (args[2] == "0" || args[2] == "1") && restartSuccess {
			return nil, nil
		}
		return nil, &testError{}
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
