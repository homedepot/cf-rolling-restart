

# Rolling Restart for CF 
[![Go Lang Version](https://img.shields.io/badge/go-1.12-00ADD8.svg?style=flat)](http://golang.com) 
[![Go Report Card](https://goreportcard.com/badge/github.com/homedepot/cf-rolling-restart)](https://goreportcard.com/report/github.com/homedepot/cf-rolling-restart) 
[![Code Coverage](https://img.shields.io/codecov/c/github/homedepot/cf-rolling-restart.svg?style=flat)](https://codecov.io/gh/homedepot/cf-rolling-restart)
[![Build Status](https://travis-ci.org/homedepot/cf-rolling-restart.svg?branch=master)](https://travis-ci.org/homedepot/cf-rolling-restart) 
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat)](LICENSE)

CF plugin for zero downtime application restarts.

This was made to fill a the gap in zero-downtime restarts for the PCF CLI. It will restart your application one instance at a time until each instance is up. The plugin should provide feedback through the CLI through the whole process.

## Installation

Download the latest version from the releases page and make it executable.

```
$ cf install-plugin path/to/downloaded/binary
```

## Usage

```
$ cf rolling-restart [--max-cycles #] APP_NAME
```

The alias `rrs` also exists for a shorthand (Ex. `cf rrs APP_NAME`).
The flag `--max-cycles` augments the number of times the plugin will check to see if the app is up. The default is `120` cycles which roughly equate to ~2 minutes. Each cycle consists of checking the current state of the recently restarted instance and then pausing 1 second until the instance is running or the max cycles have been reached.

## Compiling

To build and test for your current platform please run `./script/cibuild` from the project root.

To build and install locally run `./script/install`.

## Contributing 

Check out the [contributing](CONTRIBUTING.md) readme for information on how to contriubte to the project. 

## License 

This project is released under the Apache2 free software license. More information can be found in the [LICENSE](LICENSE) file.
