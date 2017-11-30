# rolling-restart

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

To build for all platforms please run (from a Mac or Linux system) `./scripts/build-all.sh` from the project root.